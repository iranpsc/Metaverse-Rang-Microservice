// Package models defines domain types for calendar events.
package models

import "time"

// Calendar represents an event or version entry
type Calendar struct {
	ID           uint64     `db:"id"`
	Slug         *string    `db:"slug"`
	Title        string     `db:"title"`
	Content      string     `db:"content"`
	Color        string     `db:"color"`
	Writer       string     `db:"writer"`
	IsVersion    bool       `db:"is_version"`
	VersionTitle *string    `db:"version_title"`
	BtnName      *string    `db:"btn_name"`
	BtnLink      *string    `db:"btn_link"`
	Image        *string    `db:"image"`
	StartsAt     time.Time  `db:"starts_at"`
	EndsAt       *time.Time `db:"ends_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

// Interaction represents a like/dislike on a calendar event
type Interaction struct {
	ID           uint64    `db:"id"`
	LikeableType string    `db:"likeable_type"`
	LikeableID   uint64    `db:"likeable_id"`
	UserID       uint64    `db:"user_id"`
	Liked        bool      `db:"liked"` // true=like, false=dislike
	IPAddress    string    `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// View represents a view tracking entry
type View struct {
	ID           uint64    `db:"id"`
	ViewableType string    `db:"viewable_type"`
	ViewableID   uint64    `db:"viewable_id"`
	IPAddress    string    `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// CalendarStats holds aggregated statistics for an event
type CalendarStats struct {
	ViewsCount    int32
	LikesCount    int32
	DislikesCount int32
}
