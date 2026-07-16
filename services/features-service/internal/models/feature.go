package models

import (
	"database/sql"
	"time"
)

// Feature represents a land/property feature
// NOTE: features table does NOT have geometry_id column - geometries table has feature_id instead
type Feature struct {
	ID        uint64    `db:"id"`
	OwnerID   uint64    `db:"owner_id"`
	MapID     uint64    `db:"map_id"`
	Type      string    `db:"type"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// FeatureProperties represents feature_properties table
type FeatureProperties struct {
	ID                     string    `db:"id"` // VARCHAR PK
	FeatureID              uint64    `db:"feature_id"`
	Karbari                string    `db:"karbari"`
	RGB                    string    `db:"rgb"`
	Owner                  string    `db:"owner"`
	Label                  string    `db:"label"`
	Address                string    `db:"address"`
	Area                   float64   `db:"area"`
	Density                int       `db:"density"`
	Stability              float64   `db:"stability"`
	PricePSC               string    `db:"price_psc"` // Stored as string
	PriceIRR               string    `db:"price_irr"` // Stored as string
	MinimumPricePercentage int       `db:"minimum_price_percentage"`
	CreatedAt              time.Time `db:"created_at"`
	UpdatedAt              time.Time `db:"updated_at"`
}

// Trade represents trades table
type Trade struct {
	ID        uint64    `db:"id"`
	FeatureID uint64    `db:"feature_id"`
	BuyerID   uint64    `db:"buyer_id"`
	SellerID  uint64    `db:"seller_id"`
	IRRAmount float64   `db:"irr_amount"`
	PSCAmount float64   `db:"psc_amount"`
	Date      time.Time `db:"date"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// BuyFeatureRequest represents buy_feature_requests table
type BuyFeatureRequest struct {
	ID                   uint64       `db:"id"`
	BuyerID              uint64       `db:"buyer_id"`
	SellerID             uint64       `db:"seller_id"`
	FeatureID            uint64       `db:"feature_id"`
	Note                 string       `db:"note"`
	PricePSC             float64      `db:"price_psc"`
	PriceIRR             float64      `db:"price_irr"`
	Status               int          `db:"status"`
	RequestedGracePeriod sql.NullTime `db:"requested_grace_period"`
	DeletedAt            sql.NullTime `db:"deleted_at"` // Soft delete
	CreatedAt            time.Time    `db:"created_at"`
	UpdatedAt            time.Time    `db:"updated_at"`
}

// SellFeatureRequest represents sell_feature_requests table
type SellFeatureRequest struct {
	ID        uint64    `db:"id"`
	SellerID  uint64    `db:"seller_id"`
	FeatureID uint64    `db:"feature_id"`
	PricePSC  float64   `db:"price_psc"`
	PriceIRR  float64   `db:"price_irr"`
	Limit     int       `db:"limit"` // Percentage of stability (underpriced if < 100)
	Status    int       `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// LockedAsset represents locked_wallets/locked_assets table
type LockedAsset struct {
	ID                  uint64    `db:"id"`
	BuyFeatureRequestID uint64    `db:"buy_feature_request_id"`
	FeatureID           uint64    `db:"feature_id"`
	PSC                 float64   `db:"psc"`
	IRR                 float64   `db:"irr"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

// FeatureHourlyProfit represents feature_hourly_profits table
type FeatureHourlyProfit struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	FeatureID uint64    `db:"feature_id"`
	Asset     string    `db:"asset"` // blue/red/yellow
	Amount    float64   `db:"amount"`
	Deadline  time.Time `db:"dead_line"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	// Joined fields from feature_properties
	FeatureDBID  uint64 `db:"feature_db_id"` // features.id
	PropertiesID string `db:"properties_id"` // feature_properties.id
	Karbari      string `db:"karbari"`       // feature_properties.karbari
}

// FeatureLimit represents feature_limits table (for limited feature campaigns)
type FeatureLimit struct {
	ID                 uint64    `db:"id"`
	Title              string    `db:"title"`
	StartDate          time.Time `db:"start_date"`
	EndDate            time.Time `db:"end_date"`
	StartID            string    `db:"start_id"` // feature_properties.id (VARCHAR)
	EndID              string    `db:"end_id"`   // feature_properties.id (VARCHAR)
	PriceLimit         bool      `db:"price_limit"`
	VerifiedKYCLimit   bool      `db:"verified_kyc_limit"`
	Under18Limit       bool      `db:"under_18_limit"`
	MoreThan18Limit    bool      `db:"more_than_18_limit"`
	DynastyOwnerLimit  bool      `db:"dynasty_owner_limit"`
	IndividualBuyLimit bool      `db:"individual_buy_limit"`
	IndividualBuyCount int       `db:"individual_buy_count"`
	Expired            bool      `db:"expired"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

// LimitedFeaturePurchase tracks limited feature purchases per user
type LimitedFeaturePurchase struct {
	ID             uint64    `db:"id"`
	UserID         uint64    `db:"user_id"`
	FeatureLimitID uint64    `db:"feature_limit_id"`
	FeatureID      uint64    `db:"feature_id"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// Commission represents comissions table
type Commission struct {
	ID        uint64    `db:"id"`
	TradeID   uint64    `db:"trade_id"`
	PSC       float64   `db:"psc"`
	IRR       float64   `db:"irr"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// LockedFeature represents locked_features table
type LockedFeature struct {
	ID         uint64    `db:"id"`
	FeatureID  uint64    `db:"feature_id"`
	LockedFrom time.Time `db:"locked_from"`
	LockedTo   time.Time `db:"locked_to"`
	Status     int       `db:"status"` // 0 = active, 1 = unlocked
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Geometry represents geometries table
type Geometry struct {
	ID        uint64    `db:"id"`
	FeatureID uint64    `db:"feature_id"`
	Type      string    `db:"type"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Coordinate represents coordinates table
type Coordinate struct {
	ID         uint64  `db:"id"`
	GeometryID uint64  `db:"geometry_id"`
	X          float64 `db:"x"`
	Y          float64 `db:"y"`
}

// Building represents buildings table (pivot for feature_id and building_model_id)
type Building struct {
	ID              uint64        `db:"id"`
	FeatureID       uint64        `db:"feature_id"`
	BuildingModelID uint64        `db:"model_id"`
	Health          sql.NullInt64 `db:"health"`
	CreatedAt       time.Time     `db:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"`
}
