// Package models defines domain types for the support service.
package models

import (
	"time"
)

// Note represents a personal note (Laravel: notes.attachments is JSON array of URLs).
type Note struct {
	ID          uint64    `db:"id"`
	Title       string    `db:"title"`
	Content     string    `db:"content"`
	Attachments []string  `db:"attachments"`
	UserID      uint64    `db:"user_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
