package models

import "time"

const (
	CitizenBuildingPerPage = 10
	CitizenBuildingPath    = "/api/citizen/buildings"
)

// CitizenBuildingSummaryItem is a per-karbari completed building count.
type CitizenBuildingSummaryItem struct {
	Karbari string
	Label   string
	Count   int32
}

// CitizenBuildingSummaryResult is the summary response payload.
type CitizenBuildingSummaryResult struct {
	Items []CitizenBuildingSummaryItem
}

// CitizenBuildingChartData is chart labels and completed counts per bucket.
type CitizenBuildingChartData struct {
	Labels    []string
	Completed []int32
}

// CitizenBuildingRow is a database row for the public building list.
type CitizenBuildingRow struct {
	FeaturePropertiesID string
	Karbari             string
	AttributesJSON      string
	ConstructionEndDate time.Time
}

// CitizenBuildingListItem is a mapped public building list row.
type CitizenBuildingListItem struct {
	FeaturePropertiesID string
	Karbari             string
	Area                *float64
	Visitors            *float64
	EmptyUnits          *float64
	Floors              *float64
	ConstructionEndDate *string
}

// CitizenBuildingsPage is a paginated public buildings list.
type CitizenBuildingsPage struct {
	Items       []CitizenBuildingListItem
	CurrentPage int
	PerPage     int
	Total       int
	LastPage    int
	From        *int
	To          *int
	Path        string
}
