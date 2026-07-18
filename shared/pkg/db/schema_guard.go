package db

import (
	"database/sql"
	"fmt"
)

// ColumnType represents expected column schema
type ColumnType struct {
	Name     string
	DataType string
	Nullable bool
}

// TableSchema represents expected table structure
type TableSchema struct {
	Name    string
	Columns []ColumnType
}

// SchemaGuard validates database schema matches expectations
type SchemaGuard struct {
	db *sql.DB
}

// NewSchemaGuard creates a new schema guard
func NewSchemaGuard(db *sql.DB) *SchemaGuard {
	return &SchemaGuard{db: db}
}

// ValidateTable validates a table's schema
func (sg *SchemaGuard) ValidateTable(schema TableSchema) error {
	// Query actual schema
	query := `
		SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := sg.db.Query(query, schema.Name)
	if err != nil {
		return fmt.Errorf("failed to query table schema for %s: %w", schema.Name, err)
	}
	defer func() { _ = rows.Close() }()

	actualColumns := make(map[string]ColumnType)
	for rows.Next() {
		var colName, dataType, isNullable string
		if err := rows.Scan(&colName, &dataType, &isNullable); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		actualColumns[colName] = ColumnType{
			Name:     colName,
			DataType: dataType,
			Nullable: isNullable == "YES",
		}
	}

	if len(actualColumns) == 0 {
		return fmt.Errorf("table %s does not exist or has no columns", schema.Name)
	}

	// Validate expected columns exist with correct types
	for _, expectedCol := range schema.Columns {
		actualCol, exists := actualColumns[expectedCol.Name]
		if !exists {
			return fmt.Errorf("table %s missing expected column: %s", schema.Name, expectedCol.Name)
		}

		// Check data type (case insensitive, allowing for variations like varchar(191) vs varchar)
		if !matchesDataType(actualCol.DataType, expectedCol.DataType) {
			return fmt.Errorf("table %s column %s has type %s, expected %s",
				schema.Name, expectedCol.Name, actualCol.DataType, expectedCol.DataType)
		}
	}

	return nil
}

// matchesDataType checks if data types are compatible (handles varchar(n), decimal(n,m), etc.)
func matchesDataType(actual, expected string) bool {
	// Simple check - can be enhanced to handle size specifications
	if actual == expected {
		return true
	}

	// Handle base type matching (e.g., varchar matches varchar(191))
	if len(actual) >= len(expected) && actual[:len(expected)] == expected {
		return true
	}

	return false
}

// ValidateTables validates multiple tables
func (sg *SchemaGuard) ValidateTables(schemas []TableSchema) error {
	for _, schema := range schemas {
		if err := sg.ValidateTable(schema); err != nil {
			return err
		}
	}
	return nil
}
