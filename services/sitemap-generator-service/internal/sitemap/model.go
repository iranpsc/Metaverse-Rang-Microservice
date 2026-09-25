package sitemap

import (
	"context"
	"time"
)

const (
	// MaxURLsPerFile matches SitemapGenerator::MAX_URLS_PER_FILE.
	MaxURLsPerFile = 5000

	ChangeFreqDaily   = "daily"
	ChangeFreqMonthly = "monthly"

	PriorityCitizen  = 0.8
	PriorityVideo    = 0.8
	PriorityCalendar = 0.6
)

// Entry is one url element in a sitemap file.
type Entry struct {
	Loc        string
	LastMod    time.Time
	HasLastMod bool
	ChangeFreq string
	Priority   float64
	Images     []string
}

// User is a citizen row used to expand templates.json.
type User struct {
	Code       string
	UpdatedAt  time.Time
	HasUpdated bool
}

// Video is a tutorials row.
type Video struct {
	Slug       string
	UpdatedAt  time.Time
	HasUpdated bool
}

// Category is a video category row.
type Category struct {
	Slug       string
	Image      string
	UpdatedAt  time.Time
	HasUpdated bool
}

// SubCategory is a video subcategory row plus its parent category slug.
type SubCategory struct {
	Slug         string
	CategorySlug string
	Image        string
	UpdatedAt    time.Time
	HasUpdated   bool
}

// CalendarEvent is a calendars row with is_version = 0.
type CalendarEvent struct {
	ID         int64
	UpdatedAt  time.Time
	HasUpdated bool
}

// CalendarVersion is a calendars row with is_version = 1.
type CalendarVersion struct {
	VersionTitle string
	UpdatedAt    time.Time
	HasUpdated   bool
}

// Source reads the rows the generator turns into sitemap URLs.
type Source interface {
	Users(ctx context.Context) ([]User, error)
	Videos(ctx context.Context) ([]Video, error)
	Categories(ctx context.Context) ([]Category, error)
	SubCategories(ctx context.Context) ([]SubCategory, error)
	CalendarEvents(ctx context.Context) ([]CalendarEvent, error)
	CalendarVersions(ctx context.Context) ([]CalendarVersion, error)
}
