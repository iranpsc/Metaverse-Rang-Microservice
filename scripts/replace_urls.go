package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// Database connection details
	dbUser     = "metargb_user"
	dbPassword = "metargb_password"
	dbName     = "metargb_db"
	dbHost     = "127.0.0.1" // Change to your DB host if not local
	dbPort     = "3306"

	// URLs to replace
	oldURL1 = "https://api.rgb.irpsc.com"
	newURL1 = "https://api.metarang.com"

	oldURL2 = "https://admin.rgb.irpsc.com"
	newURL2 = "https://admin.metarang.com"
)

func main() {
	// 1. Connect to the MySQL database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", 
		dbUser, dbPassword, dbHost, dbPort, dbName)
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Error configuring database connection: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Error connecting to the database: %v", err)
	}
	fmt.Println("🚀 Successfully connected to metargb_db")

	// 2. Fetch all text-based columns from all tables in the database
	query := `
		SELECT TABLE_NAME, COLUMN_NAME 
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = ? 
		AND DATA_TYPE IN ('varchar', 'text', 'mediumtext', 'longtext', 'char', 'tinytext')`

	rows, err := db.Query(query, dbName)
	if err != nil {
		log.Fatalf("❌ Error fetching database schema: %v", err)
	}
	defer rows.Close()

	// 3. Iterate through tables and columns to apply the updates
	var totalRowsAffected int64

	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			log.Printf("⚠️ Error scanning row: %v", err)
			continue
		}

		// Escape table and column names with backticks to prevent SQL syntax issues
		escapedTable := fmt.Sprintf("`%s`", strings.ReplaceAll(tableName, "`", "``"))
		escapedColumn := fmt.Sprintf("`%s`", strings.ReplaceAll(columnName, "`", "``"))

		// Construct the update query using MySQL's REPLACE function
		// We handle both URL replacements in a single query per column
		updateQuery := fmt.Sprintf(
			"UPDATE %s SET %s = REPLACE(REPLACE(%s, ?, ?), ?, ?) WHERE %s LIKE ? OR %s LIKE ?",
			escapedTable, escapedColumn, escapedColumn, escapedColumn, escapedColumn,
		)

		// Execute the update
		result, err := db.Exec(updateQuery, 
			oldURL1, newURL1, 
			oldURL2, newURL2, 
			"%"+oldURL1+"%", 
			"%"+oldURL2+"%",
		)
		if err != nil {
			log.Printf("⚠️ Error updating table %s, column %s: %v", tableName, columnName, err)
			continue
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			continue
		}

		if rowsAffected > 0 {
			fmt.Printf("✅ Updated %d row(s) in [%s].[%s]\n", rowsAffected, tableName, columnName)
			totalRowsAffected += rowsAffected
		}
	}

	fmt.Printf("\n🎉 Script finished. Total rows updated across all tables: %d\n", totalRowsAffected)
}