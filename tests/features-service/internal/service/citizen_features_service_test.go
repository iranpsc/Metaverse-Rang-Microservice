package service_test

import (
	"context"
	"testing"
	"time"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCitizenFeaturesRepo struct {
	countOwned     func(ctx context.Context, userID uint64, karbari string) (int32, error)
	countTrades    func(ctx context.Context, userID uint64, role, karbari string, start, end time.Time) (int32, error)
	listTimestamps func(ctx context.Context, userID uint64, role string, karbaris []string, start, end time.Time) ([]models.CitizenTradeTimestamp, error)
	listOwned      func(ctx context.Context, userID uint64, karbaris []string, search string, page, perPage int) ([]models.CitizenFeatureListItem, int, error)
	listMapMarkers func(ctx context.Context, userID uint64, karbaris []string) ([]models.CitizenFeatureMapMarker, error)
}

func (m *mockCitizenFeaturesRepo) CountOwnedByKarbari(ctx context.Context, userID uint64, karbari string) (int32, error) {
	return m.countOwned(ctx, userID, karbari)
}

func (m *mockCitizenFeaturesRepo) CountTradesByKarbari(ctx context.Context, userID uint64, role, karbari string, start, end time.Time) (int32, error) {
	return m.countTrades(ctx, userID, role, karbari, start, end)
}

func (m *mockCitizenFeaturesRepo) ListTradeTimestamps(ctx context.Context, userID uint64, role string, karbaris []string, start, end time.Time) ([]models.CitizenTradeTimestamp, error) {
	return m.listTimestamps(ctx, userID, role, karbaris, start, end)
}

func (m *mockCitizenFeaturesRepo) ListOwnedFeatures(ctx context.Context, userID uint64, karbaris []string, search string, page, perPage int) ([]models.CitizenFeatureListItem, int, error) {
	return m.listOwned(ctx, userID, karbaris, search, page, perPage)
}

func (m *mockCitizenFeaturesRepo) ListMapMarkers(ctx context.Context, userID uint64, karbaris []string) ([]models.CitizenFeatureMapMarker, error) {
	return m.listMapMarkers(ctx, userID, karbaris)
}

func TestCitizenFeaturesService_GetSummary_EmptyKarbaris(t *testing.T) {
	svc := service.NewCitizenFeaturesService(&mockCitizenFeaturesRepo{})
	result, err := svc.GetSummary(context.Background(), 1, "daily", nil, time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local))
	require.NoError(t, err)
	assert.Equal(t, "daily", result.Period)
	assert.Empty(t, result.Items)
}

func TestCitizenFeaturesService_GetSummary_PerKarbari(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	repo := &mockCitizenFeaturesRepo{}
	repo.countOwned = func(ctx context.Context, userID uint64, karbari string) (int32, error) {
		assert.Equal(t, uint64(7), userID)
		if karbari == "t" {
			return 3, nil
		}
		return 1, nil
	}
	repo.countTrades = func(ctx context.Context, userID uint64, role, karbari string, start, end time.Time) (int32, error) {
		assert.True(t, start.Before(end))
		if role == "buyer" && karbari == "t" {
			return 2, nil
		}
		if role == "seller" && karbari == "t" {
			return 1, nil
		}
		return 0, nil
	}

	svc := service.NewCitizenFeaturesService(repo)
	result, err := svc.GetSummary(context.Background(), 7, "weekly", []string{"t", "m"}, ref)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	assert.Equal(t, "weekly", result.Period)
	assert.Equal(t, "t", result.Items[0].Karbari)
	assert.Equal(t, "تجاری", result.Items[0].Label)
	assert.Equal(t, int32(3), result.Items[0].CurrentCount)
	assert.Equal(t, int32(2), result.Items[0].BoughtCount)
	assert.Equal(t, int32(1), result.Items[0].SoldCount)
	assert.Equal(t, "مسکونی", result.Items[1].Label)
}

func TestCitizenFeaturesService_GetChart_EmptyKarbaris(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	svc := service.NewCitizenFeaturesService(&mockCitizenFeaturesRepo{})
	chart, err := svc.GetChart(context.Background(), 1, "weekly", nil, ref)
	require.NoError(t, err)
	require.Len(t, chart.Labels, 7)
	require.Len(t, chart.Bought, 7)
	require.Len(t, chart.Sold, 7)
	for i := range chart.Bought {
		assert.Equal(t, int32(0), chart.Bought[i])
		assert.Equal(t, int32(0), chart.Sold[i])
	}
}

func TestCitizenFeaturesService_GetChart_BucketsTrades(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	window, err := service.ResolvePeriod("weekly", ref)
	require.NoError(t, err)

	boughtAt := window.Buckets[1].Start.Add(2 * time.Hour)
	soldAt := window.Buckets[3].Start.Add(3 * time.Hour)

	repo := &mockCitizenFeaturesRepo{}
	repo.listTimestamps = func(ctx context.Context, userID uint64, role string, karbaris []string, start, end time.Time) ([]models.CitizenTradeTimestamp, error) {
		assert.Equal(t, []string{"t"}, karbaris)
		if role == "buyer" {
			return []models.CitizenTradeTimestamp{{ID: 1, CreatedAt: boughtAt}}, nil
		}
		return []models.CitizenTradeTimestamp{{ID: 2, CreatedAt: soldAt}}, nil
	}

	svc := service.NewCitizenFeaturesService(repo)
	chart, err := svc.GetChart(context.Background(), 7, "weekly", []string{"t"}, ref)
	require.NoError(t, err)
	require.Len(t, chart.Labels, 7)
	assert.Equal(t, int32(1), chart.Bought[1])
	assert.Equal(t, int32(1), chart.Sold[3])
	assert.Equal(t, int32(0), chart.Bought[0])
}

func TestCitizenFeaturesService_GetFeatures_EmptyKarbaris(t *testing.T) {
	svc := service.NewCitizenFeaturesService(&mockCitizenFeaturesRepo{})
	page, err := svc.GetFeatures(context.Background(), 1, nil, "search", 1, 15)
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.Empty(t, page.MapMarkers)
	assert.Equal(t, 0, page.Total)
	assert.Equal(t, 1, page.CurrentPage)
	assert.Equal(t, 15, page.PerPage)
}

func TestCitizenFeaturesService_GetFeatures_SearchDoesNotAffectMap(t *testing.T) {
	repo := &mockCitizenFeaturesRepo{}
	repo.listOwned = func(ctx context.Context, userID uint64, karbaris []string, search string, page, perPage int) ([]models.CitizenFeatureListItem, int, error) {
		assert.Equal(t, "TO111", search)
		assert.Equal(t, 2, page)
		assert.Equal(t, 10, perPage)
		return []models.CitizenFeatureListItem{
			{ID: 5, VodID: "TO111-1", Karbari: "t"},
		}, 1, nil
	}
	repo.listMapMarkers = func(ctx context.Context, userID uint64, karbaris []string) ([]models.CitizenFeatureMapMarker, error) {
		assert.Equal(t, []string{"t"}, karbaris)
		return []models.CitizenFeatureMapMarker{
			{ID: 5, Karbari: "t"},
			{ID: 6, Karbari: "t"},
		}, nil
	}

	svc := service.NewCitizenFeaturesService(repo)
	page, err := svc.GetFeatures(context.Background(), 7, []string{"t"}, "TO111", 2, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Len(t, page.MapMarkers, 2)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, 2, page.CurrentPage)
	assert.Equal(t, 1, page.LastPage)
	require.NotNil(t, page.From)
	require.NotNil(t, page.To)
	assert.Equal(t, 11, *page.From)
	assert.Equal(t, 11, *page.To)
}
