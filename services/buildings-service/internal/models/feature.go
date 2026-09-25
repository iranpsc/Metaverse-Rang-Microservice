package models

import "time"

// Feature represents a land/property feature (minimal fields used by buildings).
type Feature struct {
	ID        uint64    `db:"id"`
	OwnerID   uint64    `db:"owner_id"`
	MapID     uint64    `db:"map_id"`
	Type      string    `db:"type"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// FeatureProperties represents feature_properties table (minimal fields used by buildings).
type FeatureProperties struct {
	ID                     string    `db:"id"`
	FeatureID              uint64    `db:"feature_id"`
	Karbari                string    `db:"karbari"`
	RGB                    string    `db:"rgb"`
	Owner                  string    `db:"owner"`
	Label                  string    `db:"label"`
	Address                string    `db:"address"`
	Area                   float64   `db:"area"`
	Density                int       `db:"density"`
	Stability              float64   `db:"stability"`
	PricePSC               string    `db:"price_psc"`
	PriceIRR               string    `db:"price_irr"`
	MinimumPricePercentage int       `db:"minimum_price_percentage"`
	CreatedAt              time.Time `db:"created_at"`
	UpdatedAt              time.Time `db:"updated_at"`
}
