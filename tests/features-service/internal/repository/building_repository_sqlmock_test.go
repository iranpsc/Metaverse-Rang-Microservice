package repository_test

import (
	"context"
	"testing"

	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildingRepository_FindByFeatureID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuildingRepository(db)

	cols := []string{
		"id", "construction_start_date", "construction_end_date", "launched_satisfaction",
		"rotation", "position", "bubble_diameter", "information",
		"model_id", "model_model_id", "model_name", "model_sku", "model_images",
		"model_attributes", "model_file", "model_required_satisfaction",
	}
	mock.ExpectQuery("WHERE b.feature_id = \\?").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(9), "2024-01-01", "2024-02-01", "10",
			"0", "1,2", "3.5", "{}",
			uint64(7), uint64(101), "tower", "sku", "[]",
			"[]", "{}", 1.25,
		))
	one, err := repo.FindByFeatureID(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, uint64(9), one[0].Id)
	require.NotNil(t, one[0].Model)
	assert.Equal(t, "tower", one[0].Model.Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildingRepository_FindByFeatureIDs(t *testing.T) {
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
	assert.Equal(t, "101", byID[1][0].Model.ModelId)
	require.NoError(t, mock.ExpectationsWereMet())
}
