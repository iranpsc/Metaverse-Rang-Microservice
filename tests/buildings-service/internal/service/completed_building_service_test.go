package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCompletedBuildingRepo struct {
	rows       []models.CompletedBuildingRow
	total      int
	listErr    error
	countErr   error
	lastLimit  int
	lastOffset int
}

func (m *mockCompletedBuildingRepo) FindCompleted(ctx context.Context, now time.Time, limit, offset int) ([]models.CompletedBuildingRow, error) {
	m.lastLimit = limit
	m.lastOffset = offset
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.rows, nil
}

func (m *mockCompletedBuildingRepo) CountCompleted(ctx context.Context, now time.Time) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.total, nil
}

func TestCompletedBuildingService_Paginate_DefaultsPageAndPerPage(t *testing.T) {
	repo := &mockCompletedBuildingRepo{total: 0}
	svc := service.NewCompletedBuildingService(repo)

	page, err := svc.Paginate(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, 1, page.CurrentPage)
	assert.Equal(t, models.CompletedBuildingPerPage, page.PerPage)
	assert.Equal(t, models.CompletedBuildingPath, page.Path)
	assert.Equal(t, 10, repo.lastLimit)
	assert.Equal(t, 0, repo.lastOffset)
}

func TestCompletedBuildingService_Paginate_MapsAttributes(t *testing.T) {
	density := 3
	repo := &mockCompletedBuildingRepo{
		total: 1,
		rows: []models.CompletedBuildingRow{
			{
				ID:                  42,
				FeatureID:           7,
				FeaturePropertiesID: "abc-123",
				AttributesJSON:      `[{"slug":"length","value":30},{"slug":"width","value":50},{"slug":"name","value":"Tower A"}]`,
				Density:             &density,
				Karbari:             "m",
			},
		},
	}
	svc := service.NewCompletedBuildingService(repo)

	page, err := svc.Paginate(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)

	item := page.Items[0]
	assert.Equal(t, uint64(42), item.ID)
	assert.Equal(t, uint64(7), item.FeatureID)
	assert.Equal(t, "ABC-123", item.FeaturePropertiesID)
	require.NotNil(t, item.Length)
	assert.Equal(t, "30", *item.Length)
	require.NotNil(t, item.Width)
	assert.Equal(t, "50", *item.Width)
	require.NotNil(t, item.Density)
	assert.Equal(t, "3", *item.Density)
	assert.Equal(t, "m", item.Karbari)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, 1, page.LastPage)
	require.NotNil(t, page.From)
	require.NotNil(t, page.To)
	assert.Equal(t, 1, *page.From)
	assert.Equal(t, 1, *page.To)
}

func TestCompletedBuildingService_Paginate_MissingAttributesAreNil(t *testing.T) {
	repo := &mockCompletedBuildingRepo{
		total: 1,
		rows: []models.CompletedBuildingRow{
			{
				ID:                  1,
				FeatureID:           2,
				FeaturePropertiesID: "x",
				AttributesJSON:      `[]`,
				Density:             nil,
				Karbari:             "",
			},
		},
	}
	svc := service.NewCompletedBuildingService(repo)

	page, err := svc.Paginate(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Nil(t, page.Items[0].Length)
	assert.Nil(t, page.Items[0].Width)
	assert.Nil(t, page.Items[0].Density)
	assert.Equal(t, "", page.Items[0].Karbari)
}

func TestCompletedBuildingService_Paginate_SecondPageOffset(t *testing.T) {
	repo := &mockCompletedBuildingRepo{total: 25, rows: nil}
	svc := service.NewCompletedBuildingService(repo)

	page, err := svc.Paginate(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, 2, page.CurrentPage)
	assert.Equal(t, 3, page.LastPage)
	assert.Equal(t, 25, page.Total)
	assert.Equal(t, 10, repo.lastLimit)
	assert.Equal(t, 10, repo.lastOffset)
	assert.Nil(t, page.From)
	assert.Nil(t, page.To)
}

func TestCompletedBuildingService_Paginate_ListError(t *testing.T) {
	repo := &mockCompletedBuildingRepo{listErr: errors.New("db down")}
	svc := service.NewCompletedBuildingService(repo)

	_, err := svc.Paginate(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list completed buildings")
}

func TestCompletedBuildingService_Paginate_CountError(t *testing.T) {
	repo := &mockCompletedBuildingRepo{countErr: errors.New("count failed")}
	svc := service.NewCompletedBuildingService(repo)

	_, err := svc.Paginate(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count completed buildings")
}
