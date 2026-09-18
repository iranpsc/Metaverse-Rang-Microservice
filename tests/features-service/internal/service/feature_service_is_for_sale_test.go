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

func expectGetFeatureRelations(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM feature_hourly_profits").
		WithArgs(uint64(1), uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM buildings").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
}

func expectLatestSellRequest(mock sqlmock.Sqlmock, status *int) {
	cols := []string{"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at"}
	if status == nil {
		mock.ExpectQuery("FROM sell_feature_requests").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows(cols))
		return
	}
	now := time.Now()
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(8, 2, 1, 10.0, 20.0, 100, *status, now, now))
}

func TestFeatureService_GetFeature_IsForSaleFromLatestSellRequest(t *testing.T) {
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
			expectFeatureFindByID(mock, 2, "m", 10, 80)
			expectGetFeatureRelations(mock)
			expectLatestSellRequest(mock, tt.sellStatus)

			feat, err := svc.GetFeature(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, feat)
			assert.Equal(t, tt.want, feat.IsForSale)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFeatureService_GetFeature_IsForSaleWhenSellRequestLookupFails(t *testing.T) {
	svc, mock := newFeatureServiceSQLMock(t)
	expectFeatureFindByID(mock, 2, "m", 10, 80)
	expectGetFeatureRelations(mock)
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)

	feat, err := svc.GetFeature(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int32(0), feat.IsForSale)
	require.NoError(t, mock.ExpectationsWereMet())
}
