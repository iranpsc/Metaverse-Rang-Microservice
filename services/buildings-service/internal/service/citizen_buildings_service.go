package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/shared/pkg/helpers"
	periodpkg "metarang/shared/pkg/period"
)

type citizenBuildingsRepo interface {
	CountCompletedByKarbari(ctx context.Context, userID uint64, karbaris []string, now time.Time) (map[string]int32, error)
	ListCompletedEndDates(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error)
	ListUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time, limit, offset int) ([]models.CitizenBuildingRow, error)
	CountUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time) (int, error)
}

type citizenBuildingsUserRepo interface {
	GetUserCreatedAt(ctx context.Context, userID uint64) (time.Time, error)
}

// CitizenBuildingsService implements public citizen building queries.
type CitizenBuildingsService struct {
	repo     citizenBuildingsRepo
	userRepo citizenBuildingsUserRepo
	now      func() time.Time
}

func NewCitizenBuildingsService(repo citizenBuildingsRepo, userRepo citizenBuildingsUserRepo, now func() time.Time) *CitizenBuildingsService {
	if now == nil {
		now = time.Now
	}
	return &CitizenBuildingsService{repo: repo, userRepo: userRepo, now: now}
}

// GetSummary returns per-karbari counts of completed buildings owned by the user.
func (s *CitizenBuildingsService) GetSummary(
	ctx context.Context,
	userID uint64,
	allowedKarbaris []string,
) (*models.CitizenBuildingSummaryResult, error) {
	if len(allowedKarbaris) == 0 {
		return &models.CitizenBuildingSummaryResult{Items: []models.CitizenBuildingSummaryItem{}}, nil
	}

	counts, err := s.repo.CountCompletedByKarbari(ctx, userID, allowedKarbaris, s.now())
	if err != nil {
		return nil, fmt.Errorf("count completed buildings by karbari: %w", err)
	}

	items := make([]models.CitizenBuildingSummaryItem, 0, len(allowedKarbaris))
	for _, karbari := range allowedKarbaris {
		items = append(items, models.CitizenBuildingSummaryItem{
			Karbari: karbari,
			Label:   KarbariLabel(karbari),
			Count:   counts[karbari],
		})
	}

	return &models.CitizenBuildingSummaryResult{Items: items}, nil
}

// GetChart returns time-bucketed completed building counts, one series per karbari.
func (s *CitizenBuildingsService) GetChart(
	ctx context.Context,
	userID uint64,
	period string,
	allowedKarbaris []string,
) (*models.CitizenBuildingChartResult, error) {
	period = periodpkg.NormalizePeriod(period)
	registeredAt, err := s.userRepo.GetUserCreatedAt(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user registration date: %w", err)
	}
	window, err := periodpkg.ResolvePeriod(period, s.now(), registeredAt)
	if err != nil {
		return nil, err
	}

	if len(allowedKarbaris) == 0 {
		return &models.CitizenBuildingChartResult{
			Completed: []models.CitizenChartPoint{},
			Period:    period,
		}, nil
	}

	completed := make([]models.CitizenChartPoint, 0, len(allowedKarbaris)*len(window.Buckets))
	for _, karbari := range allowedKarbaris {
		endDates, err := s.repo.ListCompletedEndDates(
			ctx,
			userID,
			[]string{karbari},
			window.Start,
			window.End,
			s.now(),
		)
		if err != nil {
			return nil, fmt.Errorf("list completed building end dates for %s: %w", karbari, err)
		}
		for _, bucket := range window.Buckets {
			completed = append(completed, models.CitizenChartPoint{
				Karbari: karbari,
				Label:   bucket.Label,
				Amount:  float64(countEndDatesInBucket(endDates, bucket)),
			})
		}
	}

	return &models.CitizenBuildingChartResult{
		Completed: completed,
		Period:    period,
	}, nil
}

// GetBuildings returns a paginated list of completed buildings (10 per page).
func (s *CitizenBuildingsService) GetBuildings(
	ctx context.Context,
	userID uint64,
	allowedKarbaris []string,
	page int,
) (*models.CitizenBuildingsPage, error) {
	if page < 1 {
		page = 1
	}
	perPage := models.CitizenBuildingPerPage

	if len(allowedKarbaris) == 0 {
		return &models.CitizenBuildingsPage{
			Items:       []models.CitizenBuildingListItem{},
			CurrentPage: page,
			PerPage:     perPage,
			Total:       0,
			LastPage:    1,
			Path:        models.CitizenBuildingPath,
		}, nil
	}

	now := s.now()
	total, err := s.repo.CountUserCompletedBuildings(ctx, userID, allowedKarbaris, now)
	if err != nil {
		return nil, fmt.Errorf("count user completed buildings: %w", err)
	}

	lastPage := int(math.Max(1, math.Ceil(float64(total)/float64(perPage))))
	offset := (page - 1) * perPage
	rows, err := s.repo.ListUserCompletedBuildings(ctx, userID, allowedKarbaris, now, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("list user completed buildings: %w", err)
	}

	items := make([]models.CitizenBuildingListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapCitizenBuildingRow(row))
	}

	result := &models.CitizenBuildingsPage{
		Items:       items,
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		LastPage:    lastPage,
		Path:        models.CitizenBuildingPath,
	}
	if len(items) > 0 {
		from := offset + 1
		to := offset + len(items)
		result.From = &from
		result.To = &to
	}

	return result, nil
}

func mapCitizenBuildingRow(row models.CitizenBuildingRow) models.CitizenBuildingListItem {
	length := extractBuildingAttributeFloat(row.AttributesJSON, "length")
	width := extractBuildingAttributeFloat(row.AttributesJSON, "width")
	var area *float64
	if length != nil && width != nil {
		a := (*length) * (*width)
		area = &a
	}
	// Visitors and empty units are placeholders until those features are implemented.
	visitors := 0.0
	emptyUnits := 0.0
	density := extractBuildingAttributeFloat(row.AttributesJSON, "density")

	var constructionEndDate *string
	if !row.ConstructionEndDate.IsZero() {
		formatted := helpers.FormatJalaliDate(row.ConstructionEndDate)
		constructionEndDate = &formatted
	}

	return models.CitizenBuildingListItem{
		BuildingID:          row.SKU,
		Karbari:             row.Karbari,
		Area:                area,
		Visitors:            &visitors,
		EmptyUnits:          &emptyUnits,
		Density:             density,
		ConstructionEndDate: constructionEndDate,
		Images:              parseBuildingModelImages(row.ImagesJSON),
	}
}

// parseBuildingModelImages parses building_models.images JSON synced from the 3dmeta API.
// Supports [{"id":1,"url":"..."}] objects and plain URL string arrays.
func parseBuildingModelImages(imagesJSON string) []models.CitizenBuildingImage {
	if imagesJSON == "" {
		return []models.CitizenBuildingImage{}
	}

	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(imagesJSON), &raw); err != nil {
		return []models.CitizenBuildingImage{}
	}

	out := make([]models.CitizenBuildingImage, 0, len(raw))
	for _, item := range raw {
		var asString string
		if err := json.Unmarshal(item, &asString); err == nil {
			if asString != "" {
				out = append(out, models.CitizenBuildingImage{URL: asString})
			}
			continue
		}

		var asObj map[string]interface{}
		if err := json.Unmarshal(item, &asObj); err != nil {
			continue
		}
		url := extractImageURL(asObj)
		if url == "" {
			continue
		}
		out = append(out, models.CitizenBuildingImage{
			ID:  extractImageID(asObj),
			URL: url,
		})
	}
	return out
}

func extractImageURL(obj map[string]interface{}) string {
	for _, key := range []string{"url", "path", "src"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func extractImageID(obj map[string]interface{}) uint64 {
	v, ok := obj["id"]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		if n >= 0 {
			return uint64(n)
		}
	case json.Number:
		if i, err := n.Int64(); err == nil && i >= 0 {
			return uint64(i)
		}
	case string:
		if i, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func extractBuildingAttributeFloat(attributesJSON, slug string) *float64 {
	if attributesJSON == "" {
		return nil
	}

	var attrs []map[string]interface{}
	if err := json.Unmarshal([]byte(attributesJSON), &attrs); err != nil {
		return nil
	}

	for _, attr := range attrs {
		s, ok := attr["slug"].(string)
		if !ok || s != slug {
			continue
		}
		if attr["value"] == nil {
			return nil
		}
		switch v := attr["value"].(type) {
		case float64:
			return &v
		case int:
			f := float64(v)
			return &f
		case json.Number:
			if f, err := v.Float64(); err == nil {
				return &f
			}
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return &f
			}
		}
	}
	return nil
}

func countEndDatesInBucket(endDates []time.Time, bucket periodpkg.PeriodBucket) int32 {
	var count int32
	for _, endDate := range endDates {
		if !endDate.Before(bucket.Start) && !endDate.After(bucket.End) {
			count++
		}
	}
	return count
}
