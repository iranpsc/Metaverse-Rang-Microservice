package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFeatureServiceSQLMock(t *testing.T) (*service.FeatureService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := testutil.NewSQLMock(t)
	svc := service.NewFeatureService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewGeometryRepository(db),
		repository.NewImageRepository(db),
		repository.NewBuildingRepository(db),
		repository.NewTradeRepository(db),
		repository.NewHourlyProfitRepository(db),
		nil, db, nil, "https://app.test",
	)
	return svc, mock
}

func expectLatestSellRequest(mock sqlmock.Sqlmock, status *int) {
	expectLatestSellRequestForFeature(mock, 1, status)
}

func expectLatestSellRequestForFeature(mock sqlmock.Sqlmock, featureID uint64, status *int) {
	cols := []string{"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at"}
	if status == nil {
		mock.ExpectQuery("FROM sell_feature_requests").
			WithArgs(featureID).
			WillReturnRows(sqlmock.NewRows(cols))
		return
	}
	now := time.Now()
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(featureID).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(8, 2, featureID, 10.0, 20.0, 100, *status, now, now))
}

func expectListMyFeaturesOwnerPage(mock sqlmock.Sqlmock, ownerID uint64, featureIDs ...uint64) {
	now := time.Now()
	rows := sqlmock.NewRows(featureFindCols())
	for _, featureID := range featureIDs {
		rows.AddRow(
			featureID, ownerID, 1, "polygon", now, now,
			"p1", featureID, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		)
	}
	mock.ExpectQuery("LIMIT").
		WithArgs(ownerID, 5, 0).
		WillReturnRows(rows)
}

func TestFeatureService_ListMyFeatures_IsForSaleFromLatestSellRequest(t *testing.T) {
	open := 0
	completed := 1
	tests := []struct {
		name       string
		sellStatus *int
		want       int32
	}{
		{name: "open sell request status 0 sets is_for_sale 1", sellStatus: &open, want: 1},
		{name: "completed sell request status 1 sets is_for_sale 0", sellStatus: &completed, want: 0},
		{name: "missing sell request sets is_for_sale 0", sellStatus: nil, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mock := newFeatureServiceSQLMock(t)
			expectListMyFeaturesOwnerPage(mock, 2, 1)
			expectLatestSellRequest(mock, tt.sellStatus)

			list, err := svc.ListMyFeatures(context.Background(), 2, 1, "", "")
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, tt.want, list[0].IsForSale)
			if tt.sellStatus != nil {
				require.NotNil(t, list[0].LatestSellRequest)
				assert.Equal(t, int32(*tt.sellStatus), list[0].LatestSellRequest.Status)
			} else {
				assert.Nil(t, list[0].LatestSellRequest)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFeatureService_ListMyFeatures_SetsIsForSaleOnEachItem(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	open := 0
	completed := 1
	expectListMyFeaturesOwnerPage(mock, 2, 1, 2)
	expectLatestSellRequestForFeature(mock, 1, &open)
	expectLatestSellRequestForFeature(mock, 2, &completed)

	list, err := svc.ListMyFeatures(context.Background(), 2, 1, "", "")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, int32(1), list[0].IsForSale)
	assert.Equal(t, int32(0), list[1].IsForSale)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureService_ListMyFeatures_IsForSaleWhenSellRequestLookupFails(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	expectListMyFeaturesOwnerPage(mock, 2, 1)
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)

	list, err := svc.ListMyFeatures(context.Background(), 2, 1, "", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int32(0), list[0].IsForSale)
	assert.Nil(t, list[0].LatestSellRequest)
	require.NoError(t, mock.ExpectationsWereMet())
}
