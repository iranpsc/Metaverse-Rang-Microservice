// Command migrate runs Laravel-style SQL migrations against MySQL.
//
// Usage:
//
//	migrate [up]            Run pending migrations
//	migrate rollback        Roll back the last batch
//	migrate status          Show ran / pending files
//	migrate reset           Roll back all migrations
//	migrate refresh         Reset, then migrate
//	migrate install         Create the migrations table
//	migrate baseline        Record pending files without running SQL
//	migrate make <name>     Create a new SQL migration stub
//
// Flags: -path, -step, -pretend, -force
// Database: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_DATABASE
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"metarang/shared/pkg/migrate"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		args = []string{"up"}
	}

	cmd := strings.ToLower(args[0])
	rest := args[1:]
	if strings.HasPrefix(args[0], "-") {
		cmd = "up"
		rest = args
	}

	fs := flag.NewFlagSet("migrate "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("path", defaultPath(), "directory of SQL migration files")
	step := fs.Int("step", 0, "limit number of migrations (up/rollback)")
	pretend := fs.Bool("pretend", false, "dump SQL without executing")
	force := fs.Bool("force", false, "allow migrate in production")
	if err := fs.Parse(rest); err != nil {
		return err
	}

	switch cmd {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "make":
		name := strings.Join(fs.Args(), " ")
		created, err := migrate.WriteStub(*path, name, time.Now().UTC())
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Created: %s\n", created)
		return nil
	case "up", "migrate", "rollback", "down", "reset", "refresh", "baseline", "install", "status":
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usageText)
	}

	if os.Getenv("APP_ENV") == "production" && !*force && commandMutates(cmd) {
		return fmt.Errorf("refusing to run %s in production without -force", cmd)
	}

	files, err := migrate.LoadDir(*path)
	if err != nil {
		return err
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	m := &migrate.Migrator{
		Store:  &migrate.SQLStore{DB: db},
		Exec:   migrate.SQLExec(db),
		Locker: &migrate.MySQLLocker{DB: db, Timeout: 15 * time.Second},
		Out:    stdout,
	}

	switch cmd {
	case "up", "migrate":
		return m.Run(ctx, files, *step, *pretend)
	case "rollback", "down":
		return m.Rollback(ctx, files, *step, *pretend)
	case "reset":
		return m.Reset(ctx, files, *pretend)
	case "refresh":
		return m.Refresh(ctx, files, *pretend)
	case "baseline":
		return m.Baseline(ctx, files)
	case "install":
		if err := m.Store.Ensure(ctx); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "Migration table ready.")
		return nil
	case "status":
		rows, err := m.Status(ctx, files)
		if err != nil {
			return err
		}
		printStatus(stdout, rows)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usageText)
	}
}

func commandMutates(cmd string) bool {
	switch cmd {
	case "up", "migrate", "rollback", "down", "reset", "refresh", "baseline", "install":
		return true
	default:
		return false
	}
}

func openDB() (*sql.DB, error) {
	host := getenv("DB_HOST", "127.0.0.1")
	port, err := strconv.Atoi(getenv("DB_PORT", "3306"))
	if err != nil {
		return nil, fmt.Errorf("DB_PORT: %w", err)
	}
	user := getenv("DB_USER", "root")
	pass := getenv("DB_PASSWORD", "")
	name := getenv("DB_DATABASE", "metarang_db")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=false&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		user, pass, host, port, name)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultPath() string {
	candidates := []string{"scripts/migrations"}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, "scripts", "migrations"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return "scripts/migrations"
}

func printStatus(w io.Writer, rows []migrate.StatusRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "Ran?\tBatch\tMigration")
	_, _ = fmt.Fprintln(tw, "----\t-----\t---------")
	for _, row := range rows {
		ran := "Yes"
		batch := strconv.Itoa(row.Batch)
		if row.Pending {
			ran = "No"
			batch = ""
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", ran, batch, row.Name)
	}
	_ = tw.Flush()
}

const usageText = `Laravel-style SQL migrations

Commands:
  up          Run pending migrations (default)
  rollback    Roll back the last batch (or -step N)
  status      Show which files have run
  reset       Roll back all migrations
  refresh     Reset, then migrate
  install     Create the migrations table
  baseline    Mark pending files as ran without executing SQL
  make NAME   Create scripts/migrations/YYYY_MM_DD_HHMMSS_name.sql

Flags:
  -path DIR   Migration directory (default: scripts/migrations)
  -step N     Run or roll back at most N migrations
  -pretend    Print SQL without executing
  -force      Allow destructive commands when APP_ENV=production

Database env: DB_HOST DB_PORT DB_USER DB_PASSWORD DB_DATABASE
`

func printUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, usageText)
}
