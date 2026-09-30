package models

// BuildingEntryConfig is the owner's entry-fee and level-scope settings for one feature.
type BuildingEntryConfig struct {
	FeatureID      uint64
	FeePSC         string
	FeeIRR         string
	About          string
	LevelScopeType string
	LevelSlug      string
	IsActive       bool
}

// BuildingEntryCoupon is a discount code plus how many times guests have used it.
type BuildingEntryCoupon struct {
	ID                 uint64
	FeatureID          uint64
	Code               string
	DiscountPercentage int32
	MaxUsageCount      int32
	RealUsageCount     int32
}

// BuildingEntrySession is one paid visit window. EnteredAt is not reset when the guest re-enters.
// Fees are decimal strings with two fractional digits, matching decimal(18,2).
type BuildingEntrySession struct {
	FeatureID uint64
	UserID    uint64
	FeePSC    string
	FeeIRR    string
	CouponID  *uint64
}

// BuildingEntryAccess is the guest's current 24-hour visit window, if one is still valid.
type BuildingEntryAccess struct {
	ID     uint64
	Inside bool
}
