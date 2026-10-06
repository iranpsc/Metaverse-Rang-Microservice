package repository_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureRepository_SearchByIDOrAddress(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewFeatureRepository(db)

	mock.ExpectQuery("FROM feature_properties").
		WithArgs("%TEH-%", "%TEH-%").
		WillReturnRows(sqlmock.NewRows([]string{
			"feature_properties_id", "address", "price_psc", "price_irr", "karbari", "feature_id", "owner_code",
		}).AddRow("teh-1", "Tehran", "2.5", "100", "m", uint64(7), "hm-1"))

	rows, err := repo.SearchByIDOrAddress(context.Background(), "TEH-")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "teh-1", rows[0].FeaturePropertiesID)
	assert.Equal(t, "Tehran", rows[0].Address)
	assert.Equal(t, "2.5", rows[0].PricePSC)
	assert.Equal(t, "m", rows[0].Karbari)
	assert.Equal(t, uint64(7), rows[0].FeatureID)
	assert.Equal(t, "hm-1", rows[0].OwnerCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureRepository_SearchByIDOrAddress_QueryError(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewFeatureRepository(db)

	mock.ExpectQuery("FROM feature_properties").
		WithArgs("%x%", "%x%").
		WillReturnError(errors.New("db down"))

	rows, err := repo.SearchByIDOrAddress(context.Background(), "x")
	require.Error(t, err)
	assert.Nil(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
