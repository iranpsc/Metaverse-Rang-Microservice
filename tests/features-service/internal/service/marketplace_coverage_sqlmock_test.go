package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/metrics"
	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
	"metarang/features-service/tests/internal/testutil"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/logger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceService_ListAndGetHelpers(t *testing.T) {
	svc, mock := newMarketplaceSQLMock(t)
	now := time.Now()

	mock.ExpectQuery("WHERE buyer_id").
		WithArgs(uint64(2)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	list, err := svc.ListBuyRequests(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("WHERE buyer_id").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrConnDone)
	_, err = svc.ListBuyRequests(context.Background(), 2)
	require.Error(t, err)

	mock.ExpectQuery("WHERE seller_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(3)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	list, err = svc.ListReceivedBuyRequests(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("WHERE seller_id = \\?").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 10.0, 20.0, 100, 0, now, now))
	sells, err := svc.ListSellRequests(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, sells, 1)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	sellerID, err := svc.GetBuyRequestSellerID(context.Background(), 9)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), sellerID)

	mock.ExpectQuery("SELECT code FROM users").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("hm-2"))
	code, err := svc.GetUserCode(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "hm-2", code)

	mock.ExpectQuery("SELECT code FROM users").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	_, err = svc.GetUserCode(context.Background(), 2)
	require.Error(t, err)

	mock.ExpectQuery("FROM images").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://p"))
	url, err := svc.GetLatestProfilePhoto(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "https://p", url)

	mock.ExpectQuery("FROM images").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	url, err = svc.GetLatestProfilePhoto(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "", url)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_DeleteSellRequest(t *testing.T) {
	svc, mock := newMarketplaceSQLMock(t)
	now := time.Now()

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnError(sql.ErrNoRows)
	err := svc.DeleteSellRequest(context.Background(), 8, 3)
	require.Error(t, err)

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 99, 1, 10.0, 20.0, 100, 0, now, now))
	err = svc.DeleteSellRequest(context.Background(), 8, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not the seller")

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 10.0, 20.0, 100, 0, now, now))
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectExec("UPDATE feature_properties SET").
		WithArgs(sqlmock.AnyArg(), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.DeleteSellRequest(context.Background(), 8, 3))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_RejectWithMetrics(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	log := logger.NewLogger("features-test")
	stub := testutil.NewCommercialStub()
	svc := service.NewMarketplaceService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewGeometryRepository(db),
		repository.NewTradeRepository(db),
		repository.NewBuyRequestRepository(db),
		repository.NewSellRequestRepository(db),
		repository.NewLockedAssetRepository(db),
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureLimitRepository(db),
		repository.NewVariableRepository(db),
		client.NewCommercialClientFromGRPC(stub, stub),
		nil, nil,
		metrics.NewMarketplaceMetricsWithRegisterer(prometheus.NewRegistry()),
		db, log,
	)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	now := time.Now()
	mock.ExpectQuery("SELECT id, buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 10.0, 20.0, now, now))
	mock.ExpectExec("DELETE FROM transactions").
		WithArgs("App\\Models\\BuyFeatureRequest", uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM locked_wallets WHERE buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("COALESCE\\(SUM\\(la.psc\\)").
		WillReturnRows(sqlmock.NewRows([]string{"psc", "irr"}).AddRow(0.0, 0.0))

	require.NoError(t, svc.RejectBuyRequest(context.Background(), 9, 3))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_CreateSellRequest_PercentageSuccess(t *testing.T) {
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, testutil.NewCommercialStub(), notif)

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("FROM system_variables").
		WillReturnRows(sqlmock.NewRows([]string{"public_limit", "under_18_limit"}).AddRow("80", "110"))
	mock.ExpectQuery("SELECT birthdate FROM kycs").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectExec("INSERT INTO sell_feature_requests").
		WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec("UPDATE feature_properties SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTradeChannels(mock, 3, "seller")
	now := time.Now()
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 400.0, 400.0, 80, 0, now, now))

	out, err := svc.CreateSellRequest(context.Background(), &pb.CreateSellRequestRequest{
		SellerId: 3, FeatureId: 1, MinimumPricePercentage: 80,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(8), out.ID)
	require.NotEmpty(t, notif.Calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_SendBuyRequest_NilRequest(t *testing.T) {
	svc, _ := newMarketplaceSQLMock(t)
	_, err := svc.SendBuyRequest(context.Background(), nil)
	require.Error(t, err)
}

func TestMarketplaceService_CreateSellRequest_ExplicitPrices(t *testing.T) {
	svc, mock := newMarketplaceSQLMock(t)
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("FROM system_variables").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT birthdate FROM kycs").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectExec("INSERT INTO sell_feature_requests").
		WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec("UPDATE feature_properties SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	now := time.Now()
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 500.0, 500.0, 100, 0, now, now))

	out, err := svc.CreateSellRequest(context.Background(), &pb.CreateSellRequestRequest{
		SellerId: 3, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(8), out.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
