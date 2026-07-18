// Package models defines data models for the features service.
package models

import "time"

// CitizenFeatureListItem is a public list-row for citizen feature assets.
type CitizenFeatureListItem struct {
	ID        uint64
	VodID     string
	Address   string
	Area      float64
	Density   int32
	Karbari   string
	OwnerCode string
	PricePSC  string
	PriceIRR  string
	Center    *CitizenFeatureCenter
	Label     string
	Images    []CitizenFeatureImage
}

// CitizenFeatureCenter is the computed centroid of a feature polygon.
type CitizenFeatureCenter struct {
	X float64
	Y float64
}

// CitizenFeatureImage is a polymorphic feature image.
type CitizenFeatureImage struct {
	ID  uint64
	URL string
}

// CitizenFeatureMapMarker is a map marker (karbari-filtered, search-independent).
type CitizenFeatureMapMarker struct {
	ID      uint64
	Center  *CitizenFeatureCenter
	Karbari string
}

// CitizenFeatureSummaryItem is a per-karbari summary card.
type CitizenFeatureSummaryItem struct {
	Karbari      string
	Label        string
	CurrentCount int32
	BoughtCount  int32
	SoldCount    int32
}

// CitizenFeatureChartData is the bought/sold chart payload.
type CitizenFeatureChartData struct {
	Labels []string
	Bought []int32
	Sold   []int32
}

// CitizenFeaturesPage is a paginated public features list plus map markers.
type CitizenFeaturesPage struct {
	Items       []CitizenFeatureListItem
	MapMarkers  []CitizenFeatureMapMarker
	CurrentPage int
	PerPage     int
	Total       int
	LastPage    int
	From        *int
	To          *int
	Path        string
}

// CitizenFeatureSummaryResult is the summary response payload.
type CitizenFeatureSummaryResult struct {
	Items  []CitizenFeatureSummaryItem
	Period string
}

// CitizenTradeTimestamp is a trade row used for chart bucketing.
type CitizenTradeTimestamp struct {
	ID        uint64
	CreatedAt time.Time
}
