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

// CitizenChartPoint is a labeled amount for chart timelines.
type CitizenChartPoint struct {
	Karbari string
	Label   string
	Amount  float64
}

// CitizenBuildingChartResult is the chart response payload.
type CitizenBuildingChartResult struct {
	Completed []CitizenChartPoint
	Period    string
}

// CitizenBuildingRow is a database row for the public building list.
type CitizenBuildingRow struct {
	SKU                 string // building_models.sku
	Karbari             string
	AttributesJSON      string // building_models.attributes
	ImagesJSON          string // building_models.images (synced from 3dmeta)
	ConstructionEndDate time.Time
}

// CitizenBuildingImage is an image from a building model (3dmeta).
type CitizenBuildingImage struct {
	ID  uint64
	URL string
}

// CitizenBuildingListItem is a mapped public building list row.
type CitizenBuildingListItem struct {
	BuildingID          string
	Karbari             string
	Area                *float64
	Visitors            *float64
	EmptyUnits          *float64
	Density             *float64 // from building_model attributes, not feature_properties.density
	ConstructionEndDate *string
	Images              []CitizenBuildingImage
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
