package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"metarang/features-service/internal/models"
	"metarang/shared/pkg/helpers"
	periodpkg "metarang/shared/pkg/period"
)

type citizenBuildingsRepo interface {
	CountCompletedByKarbari(ctx context.Context, userID uint64, karbaris []string, now time.Time) (map[string]int32, error)
	ListCompletedEndDates(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error)
	ListUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time, limit, offset int) ([]models.CitizenBuildingRow, error)
	CountUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time) (int, error)
}

// CitizenBuildingsService implements public citizen building queries.
type CitizenBuildingsService struct {
	repo citizenBuildingsRepo
	now  func() time.Time
}

func NewCitizenBuildingsService(repo citizenBuildingsRepo, now func() time.Time) *CitizenBuildingsService {
	if now == nil {
		now = time.Now
	}
	return &CitizenBuildingsService{repo: repo, now: now}
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

// GetChart returns time-bucketed completed building counts.
func (s *CitizenBuildingsService) GetChart(
	ctx context.Context,
	userID uint64,
	period string,
	allowedKarbaris []string,
) (*models.CitizenBuildingChartData, string, error) {
	period = periodpkg.NormalizePeriod(period)
	window, err := periodpkg.ResolvePeriod(period, s.now())
	if err != nil {
		return nil, period, err
	}

	labels := make([]string, len(window.Buckets))
	completed := make([]int32, len(window.Buckets))
	for i, bucket := range window.Buckets {
		labels[i] = bucket.Label
	}

	if len(allowedKarbaris) == 0 {
		return &models.CitizenBuildingChartData{
			Labels:    labels,
			Completed: completed,
		}, period, nil
	}

	endDates, err := s.repo.ListCompletedEndDates(
		ctx,
		userID,
		allowedKarbaris,
		window.Start,
		window.End,
		s.now(),
	)
	if err != nil {
		return nil, period, fmt.Errorf("list completed building end dates: %w", err)
	}

	for i, bucket := range window.Buckets {
		completed[i] = countEndDatesInBucket(endDates, bucket)
	}

	return &models.CitizenBuildingChartData{
		Labels:    labels,
		Completed: completed,
	}, period, nil
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
	area := extractBuildingAttributeFloat(row.AttributesJSON, "area")
	visitors := extractBuildingAttributeFloat(row.AttributesJSON, "visitors")
	emptyUnits := extractBuildingAttributeFloat(row.AttributesJSON, "empty_units")
	floors := extractBuildingAttributeFloat(row.AttributesJSON, "floors")

	var constructionEndDate *string
	if !row.ConstructionEndDate.IsZero() {
		formatted := helpers.FormatJalaliDate(row.ConstructionEndDate)
		constructionEndDate = &formatted
	}

	return models.CitizenBuildingListItem{
		FeaturePropertiesID: strings.ToUpper(row.FeaturePropertiesID),
		Karbari:             row.Karbari,
		Area:                area,
		Visitors:            visitors,
		EmptyUnits:          emptyUnits,
		Floors:              floors,
		ConstructionEndDate: constructionEndDate,
	}
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
