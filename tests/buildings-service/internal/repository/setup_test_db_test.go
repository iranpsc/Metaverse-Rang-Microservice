package repository_test

import (
	"database/sql"
	"testing"

	"metarang/buildings-service/tests/internal/testutil"
)

func setupTestDB(t *testing.T) *sql.DB {
	return testutil.OpenMySQLOrSkip(t)
}
