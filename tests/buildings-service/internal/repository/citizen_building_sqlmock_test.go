package repository_test

import (
	"context"
	"testing"
	"time"

	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildingRepository_CompletedSQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuildingRepository(db)
	now := time.Now()

	empty, err := repo.CountCompletedByKarbari(context.Background(), 2, nil, now)
	require.NoError(t, err)
	assert.Empty(t, empty)
	dates, err := repo.ListCompletedEndDates(context.Background(), 2, nil, now, now, now)
	require.NoError(t, err)
	assert.Empty(t, dates)
	n, err := repo.CountUserCompletedBuildings(context.Background(), 2, nil, now)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	rows, err := repo.ListUserCompletedBuildings(context.Background(), 2, nil, now, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(now).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
	count, err := repo.CountCompleted(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	mock.ExpectQuery("GROUP BY fp.karbari").
		WithArgs(now, uint64(2), "m").
		WillReturnRows(sqlmock.NewRows([]string{"karbari", "count"}).AddRow("m", int32(2)))
	byK, err := repo.CountCompletedByKarbari(context.Background(), 2, []string{"m"}, now)
	require.NoError(t, err)
	assert.Equal(t, int32(2), byK["m"])

	mock.ExpectQuery("SELECT b.construction_end_date").
		WithArgs(now, now.Add(-time.Hour), now, uint64(2), "m").
		WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(now))
	dates, err = repo.ListCompletedEndDates(context.Background(), 2, []string{"m"}, now.Add(-time.Hour), now, now)
	require.NoError(t, err)
	require.Len(t, dates, 1)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(now, uint64(2), "m").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	n, err = repo.CountUserCompletedBuildings(context.Background(), 2, []string{"m"}, now)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	mock.ExpectQuery("ORDER BY b.construction_end_date DESC").
		WithArgs(now, uint64(2), "m", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"sku", "karbari", "attributes", "images", "end"}).
			AddRow("sku", "m", "[]", "[]", now))
	list, err := repo.ListUserCompletedBuildings(context.Background(), 2, []string{"m"}, now, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildingRepository_FindByFeatureIDsAndInfo(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuildingRepository(db)

	got, err := repo.FindByFeatureIDs(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	cols := []string{
		"feature_id", "id", "construction_start_date", "construction_end_date", "launched_satisfaction",
		"rotation", "position", "bubble_diameter", "information",
		"model_id", "model_model_id", "model_name", "model_sku", "model_images",
		"model_attributes", "model_file", "model_required_satisfaction",
	}
	mock.ExpectQuery("WHERE b.feature_id IN").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(1), uint64(9), "2024-01-01", "2024-02-01", "10",
			"0", "1,2", "3.5", "{}",
			uint64(7), uint64(101), "tower", "sku", "[]",
			"[]", "{}", 1.25,
		))
	byID, err := repo.FindByFeatureIDs(context.Background(), []uint64{1})
	require.NoError(t, err)
	require.Len(t, byID[1], 1)
	assert.Equal(t, uint64(9), byID[1][0].Id)
	require.NotNil(t, byID[1][0].Model)
	assert.Equal(t, "tower", byID[1][0].Model.Name)

	findCols := cols[1:]
	mock.ExpectQuery("WHERE b.feature_id = \\?").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(findCols).AddRow(
			uint64(9), "2024-01-01", "2024-02-01", "10",
			"0", "1,2", "3.5", "{}",
			uint64(7), uint64(101), "tower", "sku", "[]",
			"[]", "{}", 1.25,
		))
	one, err := repo.FindByFeatureID(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, one, 1)

	mock.ExpectQuery("FROM building_models").
		WithArgs(uint64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "model_id", "name", "sku", "images", "attributes", "file", "required_satisfaction",
		}).AddRow(uint64(7), uint64(101), "n", "s", "[]", "[]", "{}", 1.0))
	mock.ExpectExec("SET information").
		WithArgs(`{"name":"n"}`, uint64(1), uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateBuildingInformation(context.Background(), 1, "101", `{"name":"n"}`))
	require.NoError(t, mock.ExpectationsWereMet())
}
