package models

import (
	"time"
)

// CitizenProfile represents a citizen's public profile data
type CitizenProfile struct {
	ID              uint64
	Name            string
	Email           string
	Phone           string
	Code            string
	Score           int32
	EmailVerifiedAt time.Time
	KYC            *CitizenKYC
	Privacy        map[string]bool
	ProfilePhotos  []*ProfilePhoto
	PersonalInfo   *CitizenPersonalInfo
	CurrentLevel   *CitizenLevel   // Current level (if privacy allows)
	AchievedLevels []*CitizenLevel // All achieved levels (if privacy allows)
	Avatar         string          // Avatar URL (if privacy allows)
}

// CitizenKYC represents KYC data for citizen profile
type CitizenKYC struct {
	ID           uint64
	UserID       uint64
	Fname        string
	Lname        string
	NationalCode string
	Status       int32
	Birthdate    time.Time
	Address      string // Address field from KYC
}

// ProfilePhoto represents a profile photo
type ProfilePhoto struct {
	ID  uint64
	URL string
}

// CitizenPersonalInfo represents personal info for citizen profile
type CitizenPersonalInfo struct {
	ID             uint64
	UserID         uint64
	Occupation     string
	Education      string
	Memory         string
	LovedCity      string
	LovedCountry   string
	LovedLanguage  string
	ProblemSolving string
	Prediction     string
	About          string
	Passions       map[string]bool
}

// CitizenReferral represents a referral in the citizen referrals list
type CitizenReferral struct {
	ID             uint64
	Code           string
	Name           string
	Image          string
	CreatedAt      time.Time
	ReferrerOrders []*ReferrerOrder
}

// ReferrerOrder represents a referral order history entry
type ReferrerOrder struct {
	ID        uint64
	Amount    int64
	CreatedAt time.Time
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	CurrentPage int32
	NextPageURL string
	PrevPageURL string
}

// ReferralChartData represents aggregated referral chart data
type ReferralChartData struct {
	TotalReferralsCount       string
	TotalReferralOrdersAmount string
	ChartData                 []*ChartDataPoint
}

// ChartDataPoint represents a single data point in the chart
type ChartDataPoint struct {
	Label       string
	Count       int32
	TotalAmount int64
}

// CitizenLevel represents level information for citizen profile (Laravel PersonalInfo)
type CitizenLevel struct {
	ID    uint64
	Name  string
	Slug  string
	Score int32
	Image string
}
