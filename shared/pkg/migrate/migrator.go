// Package migrate runs Laravel-style SQL migrations against MySQL.
package migrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StatusRow is one line of `migrate status`.
type StatusRow struct {
	Name    string
	Batch   int
	Pending bool
}

// ExecFunc runs a single SQL statement.
type ExecFunc func(ctx context.Context, statement string) error

// Migrator is a Laravel-style migration runner: pending files, batches, rollback.
type Migrator struct {
	Store  Store
	Exec   ExecFunc
	Locker Locker
	Out    io.Writer
	Now    func() time.Time
}

func (m *Migrator) out() io.Writer {
	if m.Out != nil {
		return m.Out
	}
	return io.Discard
}

func (m *Migrator) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Migrator) withLock(ctx context.Context, fn func() error) error {
	locker := m.Locker
	if locker == nil {
		locker = NoopLocker{}
	}
	unlock, err := locker.Lock(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	return fn()
}

func appliedSet(rows []Applied) map[string]Applied {
	out := make(map[string]Applied, len(rows))
	for _, row := range rows {
		out[row.Name] = row
	}
	return out
}

func nextBatch(rows []Applied) int {
	max := 0
	for _, row := range rows {
		if row.Batch > max {
			max = row.Batch
		}
	}
	return max + 1
}

func pending(files []Migration, applied map[string]Applied) []Migration {
	var out []Migration
	for _, f := range files {
		if _, ok := applied[f.Name]; !ok {
			out = append(out, f)
		}
	}
	return out
}

func lookup(files []Migration) map[string]Migration {
	out := make(map[string]Migration, len(files))
	for _, f := range files {
		out[f.Name] = f
	}
	return out
}

// Status returns ran/pending rows in filename order.
func (m *Migrator) Status(ctx context.Context, files []Migration) ([]StatusRow, error) {
	if err := m.Store.Ensure(ctx); err != nil {
		return nil, err
	}
	appliedRows, err := m.Store.Applied(ctx)
	if err != nil {
		return nil, err
	}
	applied := appliedSet(appliedRows)

	var rows []StatusRow
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		seen[f.Name] = struct{}{}
		if a, ok := applied[f.Name]; ok {
			rows = append(rows, StatusRow{Name: f.Name, Batch: a.Batch, Pending: false})
			continue
		}
		rows = append(rows, StatusRow{Name: f.Name, Pending: true})
	}
	for _, a := range appliedRows {
		if _, ok := seen[a.Name]; ok {
			continue
		}
		rows = append(rows, StatusRow{Name: a.Name, Batch: a.Batch, Pending: false})
	}
	return rows, nil
}

// Run executes pending up migrations as a single new batch.
// step=0 runs all pending; otherwise at most step files.
func (m *Migrator) Run(ctx context.Context, files []Migration, step int, pretend bool) error {
	return m.withLock(ctx, func() error {
		return m.runUnlocked(ctx, files, step, pretend)
	})
}

func (m *Migrator) runUnlocked(ctx context.Context, files []Migration, step int, pretend bool) error {
	if err := m.Store.Ensure(ctx); err != nil {
		return err
	}
	appliedRows, err := m.Store.Applied(ctx)
	if err != nil {
		return err
	}
	todo := pending(files, appliedSet(appliedRows))
	if step > 0 && len(todo) > step {
		todo = todo[:step]
	}
	if len(todo) == 0 {
		_, _ = fmt.Fprintln(m.out(), "Nothing to migrate.")
		return nil
	}

	batch := nextBatch(appliedRows)
	for _, file := range todo {
		if err := m.runUp(ctx, file, batch, pretend); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) runUp(ctx context.Context, file Migration, batch int, pretend bool) error {
	_, _ = fmt.Fprintf(m.out(), "Migrating: %s\n", file.Name)
	start := m.now()
	if pretend {
		printStatements(m.out(), file.Up)
		_, _ = fmt.Fprintf(m.out(), "Migrated:  %s (pretend)\n", file.Name)
		return nil
	}
	if err := m.execScript(ctx, file.Up); err != nil {
		return fmt.Errorf("migrate %s: %w", file.Name, err)
	}
	if err := m.Store.Record(ctx, file.Name, batch); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(m.out(), "Migrated:  %s (%s)\n", file.Name, m.now().Sub(start).Round(time.Millisecond))
	return nil
}

// Rollback undoes the last batch (or step migrations if step > 0).
func (m *Migrator) Rollback(ctx context.Context, files []Migration, step int, pretend bool) error {
	return m.withLock(ctx, func() error {
		return m.rollbackUnlocked(ctx, files, step, pretend)
	})
}

func (m *Migrator) rollbackUnlocked(ctx context.Context, files []Migration, step int, pretend bool) error {
	if err := m.Store.Ensure(ctx); err != nil {
		return err
	}
	appliedRows, err := m.Store.Applied(ctx)
	if err != nil {
		return err
	}
	if len(appliedRows) == 0 {
		_, _ = fmt.Fprintln(m.out(), "Nothing to rollback.")
		return nil
	}

	var todo []Applied
	if step > 0 {
		todo = lastN(appliedRows, step)
	} else {
		todo = lastBatch(appliedRows)
	}
	if len(todo) == 0 {
		_, _ = fmt.Fprintln(m.out(), "Nothing to rollback.")
		return nil
	}

	byName := lookup(files)
	for i := len(todo) - 1; i >= 0; i-- {
		row := todo[i]
		file, ok := byName[row.Name]
		if !ok {
			return fmt.Errorf("cannot rollback %s: SQL file not found", row.Name)
		}
		if err := m.runDown(ctx, file, pretend); err != nil {
			return err
		}
	}
	return nil
}

func lastBatch(rows []Applied) []Applied {
	max := 0
	for _, row := range rows {
		if row.Batch > max {
			max = row.Batch
		}
	}
	var out []Applied
	for _, row := range rows {
		if row.Batch == max {
			out = append(out, row)
		}
	}
	return out
}

func lastN(rows []Applied, n int) []Applied {
	if n >= len(rows) {
		out := make([]Applied, len(rows))
		copy(out, rows)
		return out
	}
	out := make([]Applied, n)
	copy(out, rows[len(rows)-n:])
	return out
}

func (m *Migrator) runDown(ctx context.Context, file Migration, pretend bool) error {
	if strings.TrimSpace(file.Down) == "" {
		return fmt.Errorf("cannot rollback %s: missing -- migrate:down section", file.Name)
	}
	_, _ = fmt.Fprintf(m.out(), "Rolling back: %s\n", file.Name)
	start := m.now()
	if pretend {
		printStatements(m.out(), file.Down)
		_, _ = fmt.Fprintf(m.out(), "Rolled back:  %s (pretend)\n", file.Name)
		return nil
	}
	if err := m.execScript(ctx, file.Down); err != nil {
		return fmt.Errorf("rollback %s: %w", file.Name, err)
	}
	if err := m.Store.Forget(ctx, file.Name); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(m.out(), "Rolled back:  %s (%s)\n", file.Name, m.now().Sub(start).Round(time.Millisecond))
	return nil
}

// Reset rolls back every applied migration, newest first.
func (m *Migrator) Reset(ctx context.Context, files []Migration, pretend bool) error {
	return m.withLock(ctx, func() error {
		return m.resetUnlocked(ctx, files, pretend)
	})
}

func (m *Migrator) resetUnlocked(ctx context.Context, files []Migration, pretend bool) error {
	if err := m.Store.Ensure(ctx); err != nil {
		return err
	}
	appliedRows, err := m.Store.Applied(ctx)
	if err != nil {
		return err
	}
	return m.rollbackUnlocked(ctx, files, len(appliedRows), pretend)
}

// Refresh resets then runs all files.
func (m *Migrator) Refresh(ctx context.Context, files []Migration, pretend bool) error {
	return m.withLock(ctx, func() error {
		if err := m.resetUnlocked(ctx, files, pretend); err != nil {
			return err
		}
		return m.runUnlocked(ctx, files, 0, pretend)
	})
}

// Baseline records pending files as applied without executing SQL.
// Use after importing schema.sql so historical ALTER files are not re-run.
func (m *Migrator) Baseline(ctx context.Context, files []Migration) error {
	return m.withLock(ctx, func() error {
		if err := m.Store.Ensure(ctx); err != nil {
			return err
		}
		appliedRows, err := m.Store.Applied(ctx)
		if err != nil {
			return err
		}
		todo := pending(files, appliedSet(appliedRows))
		if len(todo) == 0 {
			_, _ = fmt.Fprintln(m.out(), "Nothing to baseline.")
			return nil
		}
		batch := nextBatch(appliedRows)
		for _, file := range todo {
			if err := m.Store.Record(ctx, file.Name, batch); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(m.out(), "Baselined: %s (batch %d)\n", file.Name, batch)
		}
		return nil
	})
}

func (m *Migrator) execScript(ctx context.Context, script string) error {
	if m.Exec == nil {
		return fmt.Errorf("migrator has no SQL executor")
	}
	for _, stmt := range SplitStatements(script) {
		if err := m.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func printStatements(w io.Writer, script string) {
	for _, stmt := range SplitStatements(script) {
		_, _ = fmt.Fprintf(w, "%s;\n", stmt)
	}
}

// WriteStub writes a new Laravel-style SQL migration file into dir.
func WriteStub(dir, slug string, now time.Time) (string, error) {
	slug = sanitizeMigrationSlug(slug)
	if slug == "" {
		return "", fmt.Errorf("migration name is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create migrations dir: %w", err)
	}
	stamp := now.UTC().Format("2006_01_02_150405")
	path := filepath.Join(dir, stamp+"_"+slug+".sql")
	body := `-- migrate:up


-- migrate:down

`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
