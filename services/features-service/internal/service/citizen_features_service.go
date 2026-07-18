package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"metarang/features-service/internal/models"
)

// DisplayableKarbaris matches Laravel UserFeaturesService::DISPLAYABLE_KARBARIS.
var DisplayableKarbaris = []string{"a", "m", "t", "g", "s", "b", "e", "n"}

var karbariLabels = map[string]string{
	"a": "آموزشی",
	"t": "تجاری",
	"m": "مسکونی",
	"e": "اداری",
	"b": "بهداشتی",
	"s": "فضای سبز",
	"f": "فرهنگی",
	"p": "پارکینگ",
	"z": "مذهبی",
	"n": "نمایشگاه",
	"g": "گردشگری",
}

type citizenFeaturesRepo interface {
	CountOwnedByKarbari(ctx context.Context, userID uint64, karbari string) (int32, error)
	CountTradesByKarbari(ctx context.Context, userID uint64, role, karbari string, start, end time.Time) (int32, error)
	ListTradeTimestamps(ctx context.Context, userID uint64, role string, karbaris []string, start, end time.Time) ([]models.CitizenTradeTimestamp, error)
	ListOwnedFeatures(ctx context.Context, userID uint64, karbaris []string, search string, page, perPage int) ([]models.CitizenFeatureListItem, int, error)
	ListMapMarkers(ctx context.Context, userID uint64, karbaris []string) ([]models.CitizenFeatureMapMarker, error)
}

// CitizenFeaturesService implements public citizen feature asset queries.
type CitizenFeaturesService struct {
	repo citizenFeaturesRepo
	now  func() time.Time
}

func NewCitizenFeaturesService(repo citizenFeaturesRepo) *CitizenFeaturesService {
	return &CitizenFeaturesService{
		repo: repo,
		now:  time.Now,
	}
}

// CitizenFeatureSummaryResult is the summary response payload.
type CitizenFeatureSummaryResult struct {
	Items  []models.CitizenFeatureSummaryItem
	Period string
}

// GetSummary returns per-karbari inventory and period trade counts.
func (s *CitizenFeaturesService) GetSummary(
	ctx context.Context,
	userID uint64,
	period string,
	allowedKarbaris []string,
	reference time.Time,
) (*models.CitizenFeatureSummaryResult, error) {
	period = NormalizePeriod(period)
	if reference.IsZero() {
		reference = s.now()
	}
	window, err := ResolvePeriod(period, reference)
	if err != nil {
		return nil, err
	}

	items := make([]models.CitizenFeatureSummaryItem, 0, len(allowedKarbaris))
	for _, karbari := range allowedKarbaris {
		current, err := s.repo.CountOwnedByKarbari(ctx, userID, karbari)
		if err != nil {
			return nil, fmt.Errorf("current count for %s: %w", karbari, err)
		}
		bought, err := s.repo.CountTradesByKarbari(ctx, userID, "buyer", karbari, window.Start, window.End)
		if err != nil {
			return nil, fmt.Errorf("bought count for %s: %w", karbari, err)
		}
		sold, err := s.repo.CountTradesByKarbari(ctx, userID, "seller", karbari, window.Start, window.End)
		if err != nil {
			return nil, fmt.Errorf("sold count for %s: %w", karbari, err)
		}
		items = append(items, models.CitizenFeatureSummaryItem{
			Karbari:      karbari,
			Label:        KarbariLabel(karbari),
			CurrentCount: current,
			BoughtCount:  bought,
			SoldCount:    sold,
		})
	}

	return &models.CitizenFeatureSummaryResult{
		Items:  items,
		Period: period,
	}, nil
}

// GetChart returns bought/sold counts bucketed by period.
func (s *CitizenFeaturesService) GetChart(
	ctx context.Context,
	userID uint64,
	period string,
	allowedKarbaris []string,
	reference time.Time,
) (*models.CitizenFeatureChartData, error) {
	period = NormalizePeriod(period)
	if reference.IsZero() {
		reference = s.now()
	}
	window, err := ResolvePeriod(period, reference)
	if err != nil {
		return nil, err
	}

	labels := make([]string, len(window.Buckets))
	bought := make([]int32, len(window.Buckets))
	sold := make([]int32, len(window.Buckets))
	for i, bucket := range window.Buckets {
		labels[i] = bucket.Label
	}

	if len(allowedKarbaris) == 0 {
		return &models.CitizenFeatureChartData{
			Labels: labels,
			Bought: bought,
			Sold:   sold,
		}, nil
	}

	boughtTrades, err := s.repo.ListTradeTimestamps(ctx, userID, "buyer", allowedKarbaris, window.Start, window.End)
	if err != nil {
		return nil, fmt.Errorf("list bought trades: %w", err)
	}
	soldTrades, err := s.repo.ListTradeTimestamps(ctx, userID, "seller", allowedKarbaris, window.Start, window.End)
	if err != nil {
		return nil, fmt.Errorf("list sold trades: %w", err)
	}

	for i, bucket := range window.Buckets {
		bought[i] = countTradesInBucket(boughtTrades, bucket)
		sold[i] = countTradesInBucket(soldTrades, bucket)
	}

	return &models.CitizenFeatureChartData{
		Labels: labels,
		Bought: bought,
		Sold:   sold,
	}, nil
}

// GetFeatures returns a paginated feature list plus search-independent map markers.
func (s *CitizenFeaturesService) GetFeatures(
	ctx context.Context,
	userID uint64,
	allowedKarbaris []string,
	search string,
	page, perPage int,
) (*models.CitizenFeaturesPage, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	if len(allowedKarbaris) == 0 {
		return &models.CitizenFeaturesPage{
			Items:       []models.CitizenFeatureListItem{},
			MapMarkers:  []models.CitizenFeatureMapMarker{},
			CurrentPage: page,
			PerPage:     perPage,
			Total:       0,
			LastPage:    1,
			Path:        "/api/citizen/features",
		}, nil
	}

	items, total, err := s.repo.ListOwnedFeatures(ctx, userID, allowedKarbaris, search, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}
	markers, err := s.repo.ListMapMarkers(ctx, userID, allowedKarbaris)
	if err != nil {
		return nil, fmt.Errorf("list map markers: %w", err)
	}

	lastPage := int(math.Max(1, math.Ceil(float64(total)/float64(perPage))))
	var from, to *int
	if len(items) > 0 {
		f := (page-1)*perPage + 1
		t := f + len(items) - 1
		from = &f
		to = &t
	}

	return &models.CitizenFeaturesPage{
		Items:       items,
		MapMarkers:  markers,
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		LastPage:    lastPage,
		From:        from,
		To:          to,
		Path:        "/api/citizen/features",
	}, nil
}

// KarbariLabel returns the Persian label for a karbari code.
func KarbariLabel(karbari string) string {
	if label, ok := karbariLabels[karbari]; ok {
		return label
	}
	return "نامشخص"
}

func countTradesInBucket(trades []models.CitizenTradeTimestamp, bucket PeriodBucket) int32 {
	var count int32
	for _, trade := range trades {
		if !trade.CreatedAt.Before(bucket.Start) && !trade.CreatedAt.After(bucket.End) {
			count++
		}
	}
	return count
}
