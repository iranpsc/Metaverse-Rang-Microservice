package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradeRepository_FindSystemUserID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)

	mock.ExpectQuery("SELECT id FROM users WHERE code").
		WithArgs(constants.RGBUserCode).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(2000000)))
	id, err := repo.FindSystemUserID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(2000000), id)

	mock.ExpectQuery("SELECT id FROM users WHERE code").
		WithArgs(constants.RGBUserCode).
		WillReturnError(sql.ErrNoRows)
	_, err = repo.FindSystemUserID(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTradeRepository_ListByFeatureWithDetails_EmptyAndWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM trades t").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr", "psc", "date", "created_at", "buyer_code", "buyer_name",
		}))
	list, err := repo.ListByFeatureWithDetails(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, list)

	mock.ExpectQuery("FROM trades t").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr", "psc", "date", "created_at", "buyer_code", "buyer_name",
		}).AddRow(9, 1, 2, 3, 50.0, 10.0, now, now, "hm-2", "Ali"))
	mock.ExpectQuery("FROM transactions").
		WillReturnRows(sqlmock.NewRows([]string{"payable_id", "asset", "amount", "action"}).
			AddRow(uint64(9), "yellow", 1.5, "withdraw"))

	list, err = repo.ListByFeatureWithDetails(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Len(t, list[0].Transactions, 1)
	assert.Equal(t, "yellow", list[0].Transactions[0].Asset)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVariableRepository_InvalidateAndClearCache(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewVariableRepository(db)

	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(2.0))
	assert.Equal(t, 2.0, repo.GetRate(context.Background(), "psc"))

	repo.InvalidateCache("psc")
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(3.0))
	assert.Equal(t, 3.0, repo.GetRate(context.Background(), "psc"))

	repo.ClearCache()
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(4.0))
	assert.Equal(t, 4.0, repo.GetRate(context.Background(), "psc"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageRepository_GetImageByID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewImageRepository(db)

	mock.ExpectQuery("SELECT id, url").
		WithArgs(uint64(4), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(uint64(4), "https://x"))
	img, err := repo.GetImageByID(context.Background(), 1, 4)
	require.NoError(t, err)
	assert.Equal(t, "https://x", img.URL)

	mock.ExpectQuery("SELECT id, url").
		WithArgs(uint64(4), uint64(1)).
		WillReturnError(sql.ErrNoRows)
	img, err = repo.GetImageByID(context.Background(), 1, 4)
	require.NoError(t, err)
	assert.Nil(t, img)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPropertiesRepository_GetByFeatureIDAndUpdatePricing(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewPropertiesRepository(db)
	now := time.Now()

	cols := []string{
		"id", "feature_id", "karbari", "rgb", "owner", "label", "address", "area", "density", "stability",
		"price_psc", "price_irr", "minimum_price_percentage", "created_at", "updated_at",
	}
	mock.ExpectQuery("FROM feature_properties").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"p1", 1, "m", "a", "o", "l", "addr", 10.0, 1, 5.0, "1", "2", 80, now, now,
		))
	p, err := repo.GetByFeatureID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "p1", p.ID)

	mock.ExpectExec("SET price_psc = \\?, price_irr").
		WithArgs("10", "20", 90, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdatePricing(context.Background(), 1, "10", "20", 90))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureLimitRepository_CountAndActive(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewFeatureLimitRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM limited_feature_purchases").
		WithArgs(uint64(2), uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
	n, err := repo.CountLimitedPurchases(context.Background(), 2, 3)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	cols := []string{
		"id", "title", "start_date", "end_date", "start_id", "end_id",
		"price_limit", "verified_kyc_limit", "under_18_limit", "more_than_18_limit",
		"dynasty_owner_limit", "individual_buy_limit", "individual_buy_count", "expired",
		"created_at", "updated_at",
	}
	mock.ExpectQuery("FROM feature_limits").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(3), "campaign", now, now, "a", "z",
			true, false, false, false, false, false, 0, false, now, now,
		))
	limits, err := repo.GetActiveLimitations(context.Background())
	require.NoError(t, err)
	require.Len(t, limits, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureRepository_LockAndPendingAndBBoxProps(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewFeatureRepository(db)

	mock.ExpectQuery("FROM locked_features").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	locked, err := repo.IsLocked(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, locked)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	pending, err := repo.HasPendingBuyRequests(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, pending)

	mock.ExpectQuery("FROM coordinates c").
		WillReturnRows(sqlmock.NewRows([]string{"geometry_id"}))
	feats, props, err := repo.FindByBoundingBoxWithProperties(context.Background(), []string{"0,0", "1,0", "1,1", "0,1"})
	require.NoError(t, err)
	assert.Empty(t, feats)
	assert.Empty(t, props)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_FindListDeleteSQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(8, 3, 5, 10.0, 20.0, 100, 0, now, now))
	found, err := repo.FindByID(context.Background(), 8)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), found.SellerID)

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)
	found, err = repo.FindByID(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, found)

	mock.ExpectQuery("WHERE seller_id").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(8, 3, 5, 10.0, 20.0, 100, 0, now, now))
	list, err := repo.ListBySellerID(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("WHERE seller_id = \\? AND feature_id").
		WithArgs(uint64(3), uint64(5)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(8, 3, 5, 10.0, 20.0, 100, 0, now, now))
	latest, err := repo.GetLatestForSellerAndFeature(context.Background(), 3, 5)
	require.NoError(t, err)
	assert.Equal(t, uint64(8), latest.ID)

	mock.ExpectQuery("feature_id = \\? AND status = 0").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(9, 3, 5, 11.0, 21.0, 100, 0, now, now))
	open, err := repo.GetLatestOpenByFeatureID(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), open.ID)

	mock.ExpectQuery("WHERE feature_id IN").
		WithArgs(uint64(5), uint64(6)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(9, 3, 5, 11.0, 21.0, 100, 0, now, now).
			AddRow(4, 3, 5, 1.0, 2.0, 90, 0, now.Add(-time.Hour), now).
			AddRow(10, 3, 6, 12.0, 22.0, 100, 0, now, now))
	byIDs, err := repo.GetLatestOpenByFeatureIDs(context.Background(), []uint64{5, 6})
	require.NoError(t, err)
	require.Len(t, byIDs, 2)
	assert.Equal(t, uint64(9), byIDs[5].ID)
	assert.Equal(t, uint64(10), byIDs[6].ID)

	mock.ExpectExec("DELETE FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(context.Background(), 8))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_FindByUserID_Defaults(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(2), 11, 0).
		WillReturnRows(sqlmock.NewRows(hourlyProfitJoinCols()).AddRow(
			11, 2, 5, "yellow", 0.0, now, true, now, now, 5, "p1", "m",
		))
	list, more, err := repo.FindByUserID(context.Background(), 2, 1, 10)
	require.NoError(t, err)
	assert.False(t, more)
	require.Len(t, list, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
