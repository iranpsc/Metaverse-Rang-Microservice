package testutil

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// OpenMySQLOrSkip returns a pooled MySQL connection when TEST_MYSQL_DSN is set; otherwise skips the test.
// Example DSN: user:pass@tcp(127.0.0.1:3306)/metargb_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci
func OpenMySQLOrSkip(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run repository integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("mysql ping: %v", err)
	}
	return db
}
