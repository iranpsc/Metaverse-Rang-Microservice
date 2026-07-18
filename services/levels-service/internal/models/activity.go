// Package models defines domain types for the levels service.
package models

import "time"

// UserActivity represents user activity sessions (login/logout tracking)
// Maps to Laravel: App\Models\Levels\UserActivity
type UserActivity struct {
	ID        uint64     `json:"id" db:"id"`
	UserID    uint64     `json:"user_id" db:"user_id"`
	Start     time.Time  `json:"start" db:"start"`
	End       *time.Time `json:"end" db:"end"`
	Total     *int32     `json:"total" db:"total"` // minutes
	IP        string     `json:"ip" db:"ip"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// UserLog represents aggregated user activity and score data
// Maps to Laravel: App\Models\UserLog
type UserLog struct {
	ID                uint64    `json:"id" db:"id"`
	UserID            uint64    `json:"user_id" db:"user_id"`
	TransactionsCount string    `json:"transactions_count" db:"transactions_count"` // decimal as string
	FollowersCount    string    `json:"followers_count" db:"followers_count"`       // decimal as string
	DepositAmount     string    `json:"deposit_amount" db:"deposit_amount"`         // decimal as string
	ActivityHours     string    `json:"activity_hours" db:"activity_hours"`         // decimal as string
	Score             string    `json:"score" db:"score"`                           // decimal as string
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// UserEvent represents user events (login, logout, etc.)
// Maps to Laravel: App\Models\UserEvent
type UserEvent struct {
	ID        uint64    `json:"id" db:"id"`
	UserID    uint64    `json:"user_id" db:"user_id"`
	Event     string    `json:"event" db:"event"`
	IP        string    `json:"ip" db:"ip"`
	Device    string    `json:"device" db:"device"`
	Status    int8      `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
