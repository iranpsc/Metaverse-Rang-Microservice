package service_test

import (
	"context"
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"
	"metarang/shared/pkg/period"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserCreatedAtRepo struct {
	createdAt time.Time
	err       error
}

func (m *mockUserCreatedAtRepo) GetUserCreatedAt(ctx context.Context, userID uint64) (time.Time, error) {
	return m.createdAt, m.err
}

type mockCitizenBuildingsRepo struct {
	countByKarbari func(ctx context.Context, userID uint64, karbaris []string, now time.Time) (map[string]int32, error)
	listEndDates   func(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error)
	listBuildings  func(ctx context.Context, userID uint64, karbaris []string, now time.Time, limit, offset int) ([]models.CitizenBuildingRow, error)
	countBuildings func(ctx context.Context, userID uint64, karbaris []string, now time.Time) (int, error)
}

func (m *mockCitizenBuildingsRepo) CountCompletedByKarbari(ctx context.Context, userID uint64, karbaris []string, now time.Time) (map[string]int32, error) {
	return m.countByKarbari(ctx, userID, karbaris, now)
}

func (m *mockCitizenBuildingsRepo) ListCompletedEndDates(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error) {
	return m.listEndDates(ctx, userID, karbaris, start, end, now)
}

func (m *mockCitizenBuildingsRepo) ListUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time, limit, offset int) ([]models.CitizenBuildingRow, error) {
	return m.listBuildings(ctx, userID, karbaris, now, limit, offset)
}

func (m *mockCitizenBuildingsRepo) CountUserCompletedBuildings(ctx context.Context, userID uint64, karbaris []string, now time.Time) (int, error) {
	return m.countBuildings(ctx, userID, karbaris, now)
}

func TestCitizenBuildingsService_GetSummary_EmptyKarbaris(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	svc := service.NewCitizenBuildingsService(&mockCitizenBuildingsRepo{}, &mockUserCreatedAtRepo{}, func() time.Time { return ref })

	result, err := svc.GetSummary(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Items)
}

func TestCitizenBuildingsService_GetSummary_PerKarbariWithZeroDefault(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	repo := &mockCitizenBuildingsRepo{}
	repo.countByKarbari = func(ctx context.Context, userID uint64, karbaris []string, now time.Time) (map[string]int32, error) {
		assert.Equal(t, uint64(7), userID)
		assert.Equal(t, []string{"m", "t"}, karbaris)
		assert.Equal(t, ref, now)
		return map[string]int32{"m": 5}, nil
	}

	svc := service.NewCitizenBuildingsService(repo, &mockUserCreatedAtRepo{}, func() time.Time { return ref })
	result, err := svc.GetSummary(context.Background(), 7, []string{"m", "t"})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	assert.Equal(t, "m", result.Items[0].Karbari)
	assert.Equal(t, "مسکونی", result.Items[0].Label)
	assert.Equal(t, int32(5), result.Items[0].Count)
	assert.Equal(t, "t", result.Items[1].Karbari)
	assert.Equal(t, "تجاری", result.Items[1].Label)
	assert.Equal(t, int32(0), result.Items[1].Count)
}

func TestCitizenBuildingsService_GetChart_EmptyKarbaris(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	svc := service.NewCitizenBuildingsService(&mockCitizenBuildingsRepo{}, &mockUserCreatedAtRepo{}, func() time.Time { return ref })

	result, err := svc.GetChart(context.Background(), 1, "weekly", nil)
	require.NoError(t, err)
	assert.Equal(t, "weekly", result.Period)
	assert.Empty(t, result.Completed)
}

func TestCitizenBuildingsService_GetChart_BucketsCompletedBuildings(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	window, err := period.ResolvePeriod("weekly", ref, time.Time{})
	require.NoError(t, err)

	firstBucket := window.Buckets[1].Start.Add(2 * time.Hour)
	secondBucket := window.Buckets[3].Start.Add(3 * time.Hour)

	repo := &mockCitizenBuildingsRepo{}
	repo.listEndDates = func(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error) {
		assert.Equal(t, []string{"m"}, karbaris)
		return []time.Time{firstBucket, secondBucket, secondBucket}, nil
	}

	svc := service.NewCitizenBuildingsService(repo, &mockUserCreatedAtRepo{}, func() time.Time { return ref })
	result, err := svc.GetChart(context.Background(), 7, "weekly", []string{"m"})
	require.NoError(t, err)
	assert.Equal(t, "weekly", result.Period)
	require.Len(t, result.Completed, 4)
	assert.Equal(t, "m", result.Completed[0].Karbari)
	assert.Equal(t, 1.0, result.Completed[1].Amount)
	assert.Equal(t, 2.0, result.Completed[3].Amount)
	assert.Equal(t, 0.0, result.Completed[0].Amount)
}

func TestCitizenBuildingsService_GetChart_InvalidPeriodFallsBackToDaily(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	repo := &mockCitizenBuildingsRepo{}
	repo.listEndDates = func(ctx context.Context, userID uint64, karbaris []string, start, end, now time.Time) ([]time.Time, error) {
		return []time.Time{}, nil
	}
	svc := service.NewCitizenBuildingsService(repo, &mockUserCreatedAtRepo{}, func() time.Time { return ref })

	result, err := svc.GetChart(context.Background(), 1, "invalid", []string{"m"})
	require.NoError(t, err)
	assert.Equal(t, "daily", result.Period)
	require.Len(t, result.Completed, 24)
}

func TestCitizenBuildingsService_GetBuildings_EmptyKarbaris(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	svc := service.NewCitizenBuildingsService(&mockCitizenBuildingsRepo{}, &mockUserCreatedAtRepo{}, func() time.Time { return ref })

	page, err := svc.GetBuildings(context.Background(), 1, nil, 1)
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.Equal(t, 0, page.Total)
	assert.Equal(t, 1, page.CurrentPage)
	assert.Equal(t, models.CitizenBuildingPerPage, page.PerPage)
}

func TestCitizenBuildingsService_GetBuildings_MapsAttributesAndPagination(t *testing.T) {
	ref := time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	endDate := time.Date(2024, 5, 2, 10, 0, 0, 0, time.Local)

	repo := &mockCitizenBuildingsRepo{}
	repo.countBuildings = func(ctx context.Context, userID uint64, karbaris []string, now time.Time) (int, error) {
		return 25, nil
	}
	repo.listBuildings = func(ctx context.Context, userID uint64, karbaris []string, now time.Time, limit, offset int) ([]models.CitizenBuildingRow, error) {
		assert.Equal(t, 10, limit)
		assert.Equal(t, 10, offset)
		return []models.CitizenBuildingRow{
			{
				SKU:                 "sku-residential-001",
				Karbari:             "m",
				AttributesJSON:      `[{"slug":"length","value":17},{"slug":"width","value":5},{"slug":"density","value":2}]`,
				ImagesJSON:          `[{"id":11,"url":"https://cdn.example/a.jpg"},"https://cdn.example/b.jpg"]`,
				ConstructionEndDate: endDate,
			},
		}, nil
	}

	svc := service.NewCitizenBuildingsService(repo, &mockUserCreatedAtRepo{}, func() time.Time { return ref })
	page, err := svc.GetBuildings(context.Background(), 7, []string{"m"}, 2)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	item := page.Items[0]
	assert.Equal(t, "sku-residential-001", item.BuildingID)
	assert.Equal(t, "m", item.Karbari)
	require.NotNil(t, item.Area)
	assert.Equal(t, 85.0, *item.Area) // length * width
	require.NotNil(t, item.Visitors)
	assert.Equal(t, 0.0, *item.Visitors)
	require.NotNil(t, item.EmptyUnits)
	assert.Equal(t, 0.0, *item.EmptyUnits)
	require.NotNil(t, item.Density)
	assert.Equal(t, 2.0, *item.Density)
	require.NotNil(t, item.ConstructionEndDate)
	require.Len(t, item.Images, 2)
	assert.Equal(t, uint64(11), item.Images[0].ID)
	assert.Equal(t, "https://cdn.example/a.jpg", item.Images[0].URL)
	assert.Equal(t, uint64(0), item.Images[1].ID)
	assert.Equal(t, "https://cdn.example/b.jpg", item.Images[1].URL)
	assert.Equal(t, 25, page.Total)
	assert.Equal(t, 3, page.LastPage)
	assert.Equal(t, 2, page.CurrentPage)
	require.NotNil(t, page.From)
	require.NotNil(t, page.To)
	assert.Equal(t, 11, *page.From)
	assert.Equal(t, 11, *page.To)
}
