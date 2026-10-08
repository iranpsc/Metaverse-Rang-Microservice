package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func expectBuildingModelByID(mock sqlmock.Sqlmock, id uint64) {
	mock.ExpectQuery("FROM building_models").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "model_id", "name", "sku", "images", "attributes", "file", "required_satisfaction",
		}).AddRow(uint64(7), id, "tower", "sku", "[]", "[]", "{}", 1.0))
}

func TestBuildingRepository_CRUD_SQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuildingRepository(db)
	now := time.Now()

	mock.ExpectExec("INSERT INTO building_models").
		WillReturnResult(sqlmock.NewResult(7, 1))
	require.NoError(t, repo.UpsertBuildingModel(context.Background(), 101, "tower", "sku", "[]", "[]", "{}", 1.0))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	ok, err := repo.HasBuilding(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs("buildings:feature:1").
		WillReturnRows(sqlmock.NewRows([]string{"lock"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery("SELECT id FROM building_models").
		WithArgs(uint64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(7)))
	mock.ExpectExec("INSERT INTO buildings").
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT RELEASE_LOCK").
		WithArgs("buildings:feature:1").
		WillReturnRows(sqlmock.NewRows([]string{"released"}).AddRow(1))
	require.NoError(t, repo.CreateBuilding(context.Background(), 1, 2, "101", "10", "0", "1,2", "{}", now, now, 3.5))

	expectBuildingModelByID(mock, 101)
	mock.ExpectQuery("WHERE b.feature_id = \\? AND b.model_id = \\?").
		WithArgs(uint64(1), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "construction_start_date", "construction_end_date", "launched_satisfaction",
			"rotation", "position", "bubble_diameter", "information",
			"model_id", "model_model_id", "model_name", "model_sku", "model_images",
			"model_attributes", "model_file", "model_required_satisfaction",
		}).AddRow(uint64(9), "2026-01-01 00:00:00", "2026-02-01 00:00:00", "10",
			"0", "1,2", "3.5", "{}",
			uint64(7), uint64(101), "tower", "sku", "[]", "[]", "{}", 1.25))
	b, err := repo.FindBuildingByFeatureAndModel(context.Background(), 1, "101")
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, uint64(9), b.Id)
	require.NotNil(t, b.Model)

	expectBuildingModelByID(mock, 101)
	mock.ExpectExec("UPDATE buildings").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectBuildingModelByID(mock, 101)
	mock.ExpectQuery("WHERE b.feature_id = \\? AND b.model_id = \\?").
		WithArgs(uint64(1), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "construction_start_date", "construction_end_date", "launched_satisfaction",
			"rotation", "position", "bubble_diameter", "information",
			"model_id", "model_model_id", "model_name", "model_sku", "model_images",
			"model_attributes", "model_file", "model_required_satisfaction",
		}).AddRow(uint64(9), "2026-01-01 00:00:00", "2026-02-01 00:00:00", "10",
			"0", "1,2", "3.5", "{}",
			uint64(7), uint64(101), "tower", "sku", "[]", "[]", "{}", 1.25))
	updated, err := repo.UpdateBuilding(context.Background(), 1, "101", "10", "0", "1,2", "{}", now, 3.5)
	require.NoError(t, err)
	require.NotNil(t, updated)

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs("buildings:feature:1").
		WillReturnRows(sqlmock.NewRows([]string{"lock"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM building_models").
		WithArgs(uint64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(7)))
	mock.ExpectQuery("SELECT launched_satisfaction").
		WithArgs(uint64(1), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"launched_satisfaction"}).AddRow("10"))
	mock.ExpectExec("DELETE FROM buildings").
		WithArgs(uint64(1), uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT RELEASE_LOCK").
		WithArgs("buildings:feature:1").
		WillReturnRows(sqlmock.NewRows([]string{"released"}).AddRow(1))
	launched, err := repo.DeleteBuilding(context.Background(), 1, "101")
	require.NoError(t, err)
	assert.Equal(t, "10", launched)

	mock.ExpectQuery("SELECT id FROM isic_codes").
		WithArgs("shop").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(3)))
	id, err := repo.FirstOrCreateIsicCode(context.Background(), "shop")
	require.NoError(t, err)
	assert.Equal(t, uint64(3), id)

	mock.ExpectQuery("SELECT id FROM isic_codes").
		WithArgs("new").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO isic_codes").
		WithArgs("new").
		WillReturnResult(sqlmock.NewResult(4, 1))
	id, err = repo.FirstOrCreateIsicCode(context.Background(), "new")
	require.NoError(t, err)
	assert.Equal(t, uint64(4), id)

	mock.ExpectQuery("WHERE b.construction_end_date <").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "feature_properties_id", "attributes", "density", "karbari",
		}).AddRow(uint64(9), uint64(1), "p1", "[]", int64(1), "m"))
	done, err := repo.FindCompleted(context.Background(), now, 10, 0)
	require.NoError(t, err)
	require.Len(t, done, 1)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
	n, err := repo.CountCompleted(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	empty, err := repo.CountCompletedByKarbari(context.Background(), 2, nil, now)
	require.NoError(t, err)
	assert.Empty(t, empty)
	mock.ExpectQuery("GROUP BY fp.karbari").
		WithArgs(sqlmock.AnyArg(), uint64(2), "m").
		WillReturnRows(sqlmock.NewRows([]string{"karbari", "count"}).AddRow("m", int32(2)))
	byK, err := repo.CountCompletedByKarbari(context.Background(), 2, []string{"m"}, now)
	require.NoError(t, err)
	assert.Equal(t, int32(2), byK["m"])

	dates, err := repo.ListCompletedEndDates(context.Background(), 2, nil, now, now, now)
	require.NoError(t, err)
	assert.Empty(t, dates)
	mock.ExpectQuery("SELECT b.construction_end_date").
		WillReturnRows(sqlmock.NewRows([]string{"construction_end_date"}).AddRow(now))
	dates, err = repo.ListCompletedEndDates(context.Background(), 2, []string{"m"}, now.Add(-time.Hour), now, now)
	require.NoError(t, err)
	require.Len(t, dates, 1)

	c, err := repo.CountUserCompletedBuildings(context.Background(), 2, nil, now)
	require.NoError(t, err)
	assert.Equal(t, 0, c)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
	c, err = repo.CountUserCompletedBuildings(context.Background(), 2, []string{"m"}, now)
	require.NoError(t, err)
	assert.Equal(t, 3, c)

	list, err := repo.ListUserCompletedBuildings(context.Background(), 2, nil, now, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
	mock.ExpectQuery("ORDER BY b.construction_end_date DESC").
		WillReturnRows(sqlmock.NewRows([]string{"sku", "karbari", "attributes", "images", "construction_end_date"}).
			AddRow("sku-1", "m", "[]", "[]", now))
	list, err = repo.ListUserCompletedBuildings(context.Background(), 2, []string{"m"}, now, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
