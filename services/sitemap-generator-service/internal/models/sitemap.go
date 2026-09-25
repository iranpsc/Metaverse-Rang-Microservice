// Package models defines domain types used when building sitemaps.
package models

import "time"

// URLEntry is a single sitemap URL record.
type URLEntry struct {
	Loc        string
	LastMod    time.Time
	ChangeFreq string
	Priority   string
	ImageLoc   string // empty omits image extension
}

// UserRow is the minimal user data needed for citizen sitemaps.
type UserRow struct {
	ID        uint64
	Code      string
	UpdatedAt time.Time
}

// VideoRow is the minimal video data needed for education sitemaps.
type VideoRow struct {
	ID        uint64
	Slug      string
	UpdatedAt time.Time
}

// CategoryRow is the minimal category data needed for education sitemaps.
type CategoryRow struct {
	ID        uint64
	Slug      string
	Image     string
	UpdatedAt time.Time
}

// SubCategoryRow is a sub-category joined with its parent category slug.
type SubCategoryRow struct {
	ID           uint64
	Slug         string
	Image        string
	CategorySlug string
	UpdatedAt    time.Time
}

// CalendarRow is the minimal calendar data for events/versions sitemaps.
type CalendarRow struct {
	ID           uint64
	VersionTitle string
	UpdatedAt    time.Time
}
