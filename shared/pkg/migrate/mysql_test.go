package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestSQLStoreAndMigratorMySQL(t *testing.T) {
	db := openTestMySQL(t)
	if db == nil {
		t.Skip("MySQL not available")
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	table := "migrate_pkg_test_" + suffix
	name := "2026_08_21_000000_pkg_test_" + suffix

	store := &SQLStore{DB: db}
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}

	m := &Migrator{
		Store:  store,
		Exec:   SQLExec(db),
		Locker: &MySQLLocker{DB: db, Timeout: 5 * time.Second},
	}
	files := []Migration{{
		Name: name,
		Up:   fmt.Sprintf("CREATE TABLE `%s` (id INT PRIMARY KEY)", table),
		Down: fmt.Sprintf("DROP TABLE `%s`", table),
	}}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS `"+table+"`")
		_, _ = db.ExecContext(context.Background(), "DELETE FROM migrations WHERE migration = ?", name)
	})

	if err := m.Run(ctx, files, 0, false); err != nil {
		t.Fatal(err)
	}
	applied, err := store.Applied(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range applied {
		if row.Name == name {
			found = true
			if row.Batch < 1 {
				t.Fatalf("batch = %d", row.Batch)
			}
		}
	}
	if !found {
		t.Fatal("migration was not recorded")
	}

	if err := m.Rollback(ctx, files, 1, false); err != nil {
		t.Fatal(err)
	}
	var n int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("table still exists after rollback")
	}
}

func openTestMySQL(t *testing.T) *sql.DB {
	t.Helper()
	host := getenv("DB_HOST", "127.0.0.1")
	port := getenv("DB_PORT", "3306")
	user := getenv("DB_USER", "root")
	pass := getenv("DB_PASSWORD", "root_password")
	name := getenv("DB_DATABASE", "metarang_test")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local", user, pass, host, port, name)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil
	}
	return db
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
