package database

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaGuard validates database schema matches expected structure
func TestSchemaGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	db := connectTestDB(t)
	defer db.Close()

	t.Run("VarcharPrimaryKeys", func(t *testing.T) {
		// Laravel utf8mb4 defaults to varchar(191) for indexed string PKs.
		testVarcharPK(t, db, "transactions", "id", 191)
		testVarcharPK(t, db, "feature_properties", "id", 191)
	})

	t.Run("DecimalPrecision", func(t *testing.T) {
		testDecimalColumn(t, db, "wallets", "psc", 20, 10)
		testDecimalColumn(t, db, "wallets", "red", 20, 10)
		testDecimalColumn(t, db, "wallets", "blue", 20, 10)
		testDecimalColumn(t, db, "wallets", "yellow", 20, 10)
	})

	t.Run("StringPriceColumns", func(t *testing.T) {
		testVarcharColumn(t, db, "feature_properties", "price_psc", 191)
		testVarcharColumn(t, db, "feature_properties", "price_irr", 191)
	})

	t.Run("SoftDeleteColumns", func(t *testing.T) {
		testColumnExists(t, db, "buy_feature_requests", "deleted_at")
		testColumnType(t, db, "buy_feature_requests", "deleted_at", "timestamp")
	})

	t.Run("PolymorphicColumns", func(t *testing.T) {
		testColumnExists(t, db, "transactions", "payable_id")
		testColumnExists(t, db, "transactions", "payable_type")
		testColumnType(t, db, "transactions", "payable_id", "bigint")
		testVarcharColumn(t, db, "transactions", "payable_type", 191)
	})

	t.Run("RequiredIndexes", func(t *testing.T) {
		testIndexExists(t, db, "users", "users_email_unique")
		testIndexExists(t, db, "users", "users_code_index")
		testIndexExists(t, db, "users", "users_name_index")
		testIndexExists(t, db, "users", "PRIMARY")
		testIndexExists(t, db, "features", "PRIMARY")
		testIndexExists(t, db, "transactions", "PRIMARY")
		testIndexExists(t, db, "buy_feature_requests", "PRIMARY")
	})

	t.Run("ForeignKeys", func(t *testing.T) {
		// Informational: production dumps often omit FK constraints.
		testForeignKey(t, db, "wallets", "user_id", "users", "id")
		testForeignKey(t, db, "features", "owner_id", "users", "id")
		testForeignKey(t, db, "transactions", "user_id", "users", "id")
	})
}

func testVarcharPK(t *testing.T, db *sql.DB, table, column string, expectedLength int) {
	var columnKey, dataType string
	var characterMaxLength sql.NullInt64

	query := `
		SELECT COLUMN_KEY, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column).Scan(&columnKey, &dataType, &characterMaxLength)
	require.NoError(t, err, "Column %s.%s not found", table, column)

	assert.Equal(t, "PRI", columnKey, "Column %s.%s should be primary key", table, column)
	assert.Equal(t, "varchar", dataType, "Column %s.%s should be VARCHAR", table, column)

	if characterMaxLength.Valid {
		assert.Equal(t, int64(expectedLength), characterMaxLength.Int64,
			"Column %s.%s should have length %d", table, column, expectedLength)
	}
}

func testDecimalColumn(t *testing.T, db *sql.DB, table, column string, precision, scale int) {
	var dataType string
	var numericPrecision, numericScale sql.NullInt64

	query := `
		SELECT DATA_TYPE, NUMERIC_PRECISION, NUMERIC_SCALE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column).Scan(&dataType, &numericPrecision, &numericScale)
	require.NoError(t, err, "Column %s.%s not found", table, column)

	assert.Equal(t, "decimal", dataType, "Column %s.%s should be DECIMAL", table, column)

	if numericPrecision.Valid {
		assert.Equal(t, int64(precision), numericPrecision.Int64,
			"Column %s.%s should have precision %d", table, column, precision)
	}

	if numericScale.Valid {
		assert.Equal(t, int64(scale), numericScale.Int64,
			"Column %s.%s should have scale %d", table, column, scale)
	}
}

func testVarcharColumn(t *testing.T, db *sql.DB, table, column string, length int) {
	var dataType string
	var characterMaxLength sql.NullInt64

	query := `
		SELECT DATA_TYPE, CHARACTER_MAXIMUM_LENGTH
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column).Scan(&dataType, &characterMaxLength)
	require.NoError(t, err, "Column %s.%s not found", table, column)

	assert.Equal(t, "varchar", dataType, "Column %s.%s should be VARCHAR", table, column)

	if characterMaxLength.Valid {
		assert.Equal(t, int64(length), characterMaxLength.Int64,
			"Column %s.%s should have length %d", table, column, length)
	}
}

func testColumnExists(t *testing.T, db *sql.DB, table, column string) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Column %s.%s should exist", table, column)
}

func testColumnType(t *testing.T, db *sql.DB, table, column, expectedType string) {
	var dataType string
	query := `
		SELECT DATA_TYPE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column).Scan(&dataType)
	require.NoError(t, err)
	assert.Equal(t, expectedType, dataType, "Column %s.%s should be type %s", table, column, expectedType)
}

func testIndexExists(t *testing.T, db *sql.DB, table, indexName string) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND INDEX_NAME = ?
	`

	err := db.QueryRow(query, table, indexName).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "Index %s on table %s should exist", indexName, table)
}

func testForeignKey(t *testing.T, db *sql.DB, table, column, refTable, refColumn string) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		AND COLUMN_NAME = ?
		AND REFERENCED_TABLE_NAME = ?
		AND REFERENCED_COLUMN_NAME = ?
	`

	err := db.QueryRow(query, table, column, refTable, refColumn).Scan(&count)
	require.NoError(t, err)

	// Note: Foreign keys might not be enforced in some setups, so this is informational
	if count == 0 {
		t.Logf("Warning: Foreign key %s.%s -> %s.%s not found", table, column, refTable, refColumn)
	}
}

func connectTestDB(t *testing.T) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		getEnv("DB_USER", "test_user"),
		getEnv("DB_PASSWORD", "test_password"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metarang_test"),
	)

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	return db
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
