// Package db provides shared database connection helpers and query utilities.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Connection wraps sql.DB with additional features
type Connection struct {
	DB *sql.DB
}

// NewConnection creates a new database connection with retry logic
func NewConnection(cfg Config) (*Connection, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	var db *sql.DB
	var err error

	// Retry logic for initial connection (MySQL may report healthy before accepting TCP).
	const maxRetries = 30
	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to open database after %d attempts: %w", maxRetries, err)
			}
			log.Printf("Database open attempt %d/%d failed: %v", attempt, maxRetries, err)
			time.Sleep(retryDelay(attempt))
			continue
		}

		err = db.Ping()
		if err != nil {
			_ = db.Close()
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to ping database after %d attempts: %w", maxRetries, err)
			}
			log.Printf("Database ping attempt %d/%d failed: %v", attempt, maxRetries, err)
			time.Sleep(retryDelay(attempt))
			continue
		}

		break
	}

	// Set connection pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // default
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5) // default
	}

	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute) // default
	}

	return &Connection{DB: db}, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	return c.DB.Close()
}

// Ping verifies connection is alive
func (c *Connection) Ping() error {
	return c.DB.Ping()
}

func retryDelay(attempt int) time.Duration {
	delay := time.Duration(attempt) * time.Second
	if delay > 5*time.Second {
		return 5 * time.Second
	}
	return delay
}
