// Package models defines domain types for the dynasty service.
package models

import "time"

// Dynasty represents a user's dynasty
type Dynasty struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	FeatureID uint64    `db:"feature_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Family represents a dynasty's family
type Family struct {
	ID        uint64    `db:"id"`
	DynastyID uint64    `db:"dynasty_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// FamilyMember represents a member of a family
type FamilyMember struct {
	ID           uint64    `db:"id"`
	FamilyID     uint64    `db:"family_id"`
	UserID       uint64    `db:"user_id"`
	Relationship string    `db:"relationship"` // owner, father, mother, offspring, spouse
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// JoinRequest represents a request to join a dynasty
type JoinRequest struct {
	ID           uint64    `db:"id"`
	FromUser     uint64    `db:"from_user"`
	ToUser       uint64    `db:"to_user"`
	Status       int16     `db:"status"` // 0=pending, 1=accepted, -1=rejected
	Relationship string    `db:"relationship"`
	Message      *string   `db:"message"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// ChildPermission represents permissions for children under 18
type ChildPermission struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	Verified  bool      `db:"verified"`
	BFR       bool      `db:"BFR"`  // Buy Feature Request
	SF        bool      `db:"SF"`   // Sell Feature
	W         bool      `db:"W"`    // Wallet
	JU        bool      `db:"JU"`   // Join/Unjoin
	DM        bool      `db:"DM"`   // Dynasty Management
	PIUP      bool      `db:"PIUP"` // Personal Info Update
	PITC      bool      `db:"PITC"` // Personal Info Type Change
	PIC       bool      `db:"PIC"`  // Personal Info Change
	ESOO      bool      `db:"ESOO"` // Edit Settings On/Off
	COTB      bool      `db:"COTB"` // Change Of The Birth
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// DynastyPermission represents default dynasty permissions
type DynastyPermission struct {
	ID        uint64     `db:"id"`
	BFR       bool       `db:"BFR"`
	SF        bool       `db:"SF"`
	W         bool       `db:"W"`
	JU        bool       `db:"JU"`
	DM        bool       `db:"DM"`
	PIUP      bool       `db:"PIUP"`
	PITC      bool       `db:"PITC"`
	PIC       bool       `db:"PIC"`
	ESOO      bool       `db:"ESOO"`
	COTB      bool       `db:"COTB"`
	CreatedAt *time.Time `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

// DynastyPrize represents dynasty rewards
type DynastyPrize struct {
	ID                         uint64    `db:"id"`
	Member                     string    `db:"member"`
	Satisfaction               float64   `db:"satisfaction"`
	IntroductionProfitIncrease float64   `db:"introduction_profit_increase"`
	AccumulatedCapitalReserve  float64   `db:"accumulated_capital_reserve"`
	DataStorage                float64   `db:"data_storage"`
	PSC                        int       `db:"psc"`
	CreatedAt                  time.Time `db:"created_at"`
	UpdatedAt                  time.Time `db:"updated_at"`
}

// DynastyMessage represents predefined dynasty messages
type DynastyMessage struct {
	ID        uint64    `db:"id"`
	Type      string    `db:"type"`
	Message   string    `db:"message"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UserBasic holds basic user info for join requests.
type UserBasic struct {
	ID           uint64
	Code         string
	Name         string
	ProfilePhoto *string
}
