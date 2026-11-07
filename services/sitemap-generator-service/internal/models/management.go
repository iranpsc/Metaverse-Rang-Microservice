package models

import "time"

type SitemapFile struct {
	Language    string     `json:"language"`
	Filename    string     `json:"filename"`
	Type        string     `json:"type"`
	URLCount    int        `json:"url_count"`
	Status      string     `json:"status"`
	UpdatedAt   time.Time  `json:"updated_at"`
	GeneratedAt *time.Time `json:"generated_at,omitempty"`
}

type GenerationRun struct {
	ID           int64      `json:"id"`
	Trigger      string     `json:"trigger"`
	Target       string     `json:"target"`
	Language     string     `json:"language,omitempty"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	DurationMS   *int64     `json:"duration_ms,omitempty"`
	URLCount     int        `json:"url_count"`
	AddedCount   int        `json:"added_count"`
	RemovedCount int        `json:"removed_count"`
	Error        string     `json:"error,omitempty"`
}
