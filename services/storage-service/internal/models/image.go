// Package models defines domain types for the storage service.
package models

import "time"

// Image represents a polymorphic image record
type Image struct {
	ID            uint64    `db:"id"`
	ImageableType string    `db:"imageable_type"` // e.g., "App\\Models\\User", "App\\Models\\Feature"
	ImageableID   uint64    `db:"imageable_id"`
	URL           string    `db:"url"`  // Full URL to the image
	Type          *string   `db:"type"` // Optional: profile, feature, video, etc.
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
