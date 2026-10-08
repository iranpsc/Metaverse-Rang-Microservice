package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureService_SearchFeatures_IncludesLatestPendingSellRequest(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("FROM feature_properties").
		WithArgs("%teh-%", "%teh-%").
		WillReturnRows(sqlmock.NewRows([]string{
			"feature_properties_id", "address", "price_psc", "price_irr", "karbari", "feature_id", "owner_code",
		}).AddRow("teh-1", "Tehran", "2.5", "3500", "m", uint64(7), "hm-1"))
	mock.ExpectQuery("FROM coordinates").
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "id", "geometry_id", "x", "y"}).
			AddRow(uint64(7), uint64(11), uint64(3), "51.123400", "35.678900"))
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(uint64(8), uint64(2), uint64(7), 10.5, 20.0, 100, 0, now, now))

	results, err := svc.SearchFeatures(context.Background(), " teh- ")
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	assert.Equal(t, uint64(7), got.Id)
	assert.Equal(t, "TEH-1", got.FeaturePropertiesId)
	assert.Equal(t, "Tehran", got.Address)
	assert.Equal(t, "مسکونی", got.Karbari)
	assert.Equal(t, "2.5", got.PricePsc)
	assert.Equal(t, "3500", got.PriceIrr)
	assert.Equal(t, "HM-1", got.OwnerCode)
	require.Len(t, got.Coordinates, 1)
	assert.Equal(t, uint64(11), got.Coordinates[0].Id)
	assert.InDelta(t, 51.1234, got.Coordinates[0].X, 0.000001)
	assert.InDelta(t, 35.6789, got.Coordinates[0].Y, 0.000001)
	require.NotNil(t, got.LatestSellRequest)
	assert.Equal(t, uint64(8), got.LatestSellRequest.Id)
	assert.Equal(t, uint64(2), got.LatestSellRequest.SellerId)
	assert.Equal(t, uint64(7), got.LatestSellRequest.FeatureId)
	assert.Equal(t, int32(0), got.LatestSellRequest.Status)
	assert.Equal(t, "10.5000000000", got.LatestSellRequest.PricePsc)
	assert.NotEmpty(t, got.LatestSellRequest.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureService_SearchFeatures_OmitsSellRequestWhenNonePending(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)

	mock.ExpectQuery("FROM feature_properties").
		WithArgs("%block%", "%block%").
		WillReturnRows(sqlmock.NewRows([]string{
			"feature_properties_id", "address", "price_psc", "price_irr", "karbari", "feature_id", "owner_code",
		}).AddRow("block-9", "Shiraz", "1", "2", "X", uint64(4), "ab"))
	mock.ExpectQuery("FROM coordinates").
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "id", "geometry_id", "x", "y"}))
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(4)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}))

	results, err := svc.SearchFeatures(context.Background(), "block")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "X", results[0].Karbari)
	assert.Equal(t, "AB", results[0].OwnerCode)
	assert.Empty(t, results[0].Coordinates)
	assert.Nil(t, results[0].LatestSellRequest)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureService_SearchFeatures_EmptyTerm(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	results, err := svc.SearchFeatures(context.Background(), "   ")
	require.NoError(t, err)
	assert.Empty(t, results)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureService_SearchFeatures_RepositoryError(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	mock.ExpectQuery("FROM feature_properties").
		WithArgs("%err%", "%err%").
		WillReturnError(errors.New("db down"))

	results, err := svc.SearchFeatures(context.Background(), "err")
	require.Error(t, err)
	assert.Nil(t, results)
	require.NoError(t, mock.ExpectationsWereMet())
}
