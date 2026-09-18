package models

import "time"

const ResetTypeMobile = "mobile"

// Reset is a row in the resets table tracking email/mobile change attempts.
type Reset struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	Type      string    `db:"type"`
	Value     string    `db:"value"`
	Verified  bool      `db:"verified"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
