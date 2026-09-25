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

func TestGeometryRepository_GetCoordinatesByFeatureID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewGeometryRepository(db)

	mock.ExpectQuery("SELECT c.x, c.y").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"x", "y"}).AddRow(1.5, 2.5))

	coords, err := repo.GetCoordinatesByFeatureID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.500000,2.500000"}, coords)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureRepository_FindByID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewFeatureRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM features f").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "owner_id", "map_id", "type", "created_at", "updated_at",
			"prop_id", "feature_id", "karbari", "rgb", "owner", "label", "address",
			"area", "density", "stability", "price_psc", "price_irr", "minimum_price_percentage",
			"prop_created_at", "prop_updated_at",
		}).AddRow(
			uint64(9), uint64(2), uint64(1), "land", now, now,
			"p1", uint64(9), "m", "d", "o", "l", "a",
			10.5, 3, 1.0, "1", "2", 80,
			now, now,
		))

	feature, props, err := repo.FindByID(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, feature)
	assert.Equal(t, uint64(9), feature.ID)
	assert.Equal(t, uint64(2), feature.OwnerID)
	require.NotNil(t, props)
	assert.Equal(t, "m", props.Karbari)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_ActivateDeactivate(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectExec("SET is_active = 1").
		WithArgs(uint64(4)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, repo.ActivateProfitsForFeature(context.Background(), 4))

	mock.ExpectExec("SET is_active = 0").
		WithArgs(uint64(4)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, repo.DeactivateProfitsForFeature(context.Background(), 4))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_GetUserCreatedAt(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewUserRepository(db)
	want := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT created_at FROM users").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(want))
	got, err := repo.GetUserCreatedAt(context.Background(), 7)
	require.NoError(t, err)
	assert.True(t, want.Equal(got))

	mock.ExpectQuery("SELECT created_at FROM users").
		WithArgs(uint64(8)).
		WillReturnError(sql.ErrNoRows)
	got, err = repo.GetUserCreatedAt(context.Background(), 8)
	require.NoError(t, err)
	assert.True(t, got.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}
