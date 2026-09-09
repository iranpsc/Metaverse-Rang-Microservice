package service_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
	"metarang/features-service/tests/internal/testutil"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/logger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMarketplaceWithWallet(t *testing.T, stub *testutil.CommercialStub) (*service.MarketplaceService, sqlmock.Sqlmock) {
	t.Helper()
	return newMarketplaceWithClients(t, stub, nil)
}

func newMarketplaceWithClients(t *testing.T, stub *testutil.CommercialStub, notif *testutil.NotificationStub) (*service.MarketplaceService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := testutil.NewSQLMock(t)
	log := logger.NewLogger("features-test")
	var ncli *client.NotificationClient
	if notif != nil {
		ncli = client.NewNotificationClientFromGRPC(notif)
	}
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
		ncli,
		nil,
		nil,
		db,
		log,
	)
	return svc, mock
}

func featureFindCols() []string {
	return []string{
		"id", "owner_id", "map_id", "type", "created_at", "updated_at",
		"prop_id", "feature_id", "karbari", "rgb", "owner", "label", "address",
		"area", "density", "stability", "price_psc", "price_irr", "minimum_price_percentage",
		"prop_created_at", "prop_updated_at",
	}
}

func expectFeatureFindByIDRGB(mock sqlmock.Sqlmock, ownerID uint64, karbari, rgb, pricePSC, priceIRR string, stability float64, minPct int) {
	now := time.Now()
	mock.ExpectQuery("FROM features f").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, ownerID, 1, "polygon", now, now,
			"p1", 1, karbari, rgb, "o", "l", "addr",
			10.0, 1, stability, pricePSC, priceIRR, minPct, now, now,
		))
}

func expectOwnerCode(mock sqlmock.Sqlmock, ownerID uint64, code string) {
	mock.ExpectQuery("SELECT code FROM users WHERE id").
		WithArgs(ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow(code))
}

func expectBuyerKYCLimited(mock sqlmock.Sqlmock, buyerID uint64) {
	mock.ExpectQuery("SELECT u.name, u.dynasty_id, k.birthdate").
		WithArgs(buyerID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "dynasty_id", "birthdate"}).AddRow("buyer", nil, nil))
}

func expectBuyerKYCSimple(mock sqlmock.Sqlmock, buyerID uint64) {
	mock.ExpectQuery("SELECT u.name, k.birthdate FROM users u").
		WithArgs(buyerID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "birthdate"}).AddRow("buyer", nil))
}

func limitationCols() []string {
	return []string{
		"id", "title", "start_date", "end_date", "start_id", "end_id",
		"price_limit", "verified_kyc_limit", "under_18_limit", "more_than_18_limit",
		"dynasty_owner_limit", "individual_buy_limit", "individual_buy_count", "expired",
		"created_at", "updated_at",
	}
}

func expectLimitation(mock sqlmock.Sqlmock, priceLimit bool) {
	now := time.Now()
	mock.ExpectQuery("FROM feature_limits").
		WithArgs("p1", "p1").
		WillReturnRows(sqlmock.NewRows(limitationCols()).AddRow(
			uint64(3), "campaign", now, now, "a", "z",
			priceLimit, false, false, false, false, false, 0, false, now, now,
		))
}

func expectBuySuccessDBTail(mock sqlmock.Sqlmock, buyerID, sellerID uint64, color string, irrAmount, pscAmount float64) {
	newStatus := constants.ChangeStatusToSoldAndNotPriced("m")
	mock.ExpectExec("UPDATE features SET owner_id").
		WithArgs(buyerID, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET rgb = \\?, owner = \\?, label").
		WithArgs(newStatus, "buyer", "", constants.DefaultPublicPricingLimit, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO trades").
		WithArgs(uint64(1), buyerID, sellerID, irrAmount, pscAmount).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(buyerID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, user_id, feature_id, asset, amount").
		WithArgs(uint64(1), sellerID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? AND user_id").
		WithArgs(uint64(1), sellerID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? ORDER BY").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO feature_hourly_profits").
		WithArgs(buyerID, uint64(1), color, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(1), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectReloadFeatureAndGeometry(mock sqlmock.Sqlmock, ownerID uint64, rgb string) {
	expectFeatureFindByIDRGB(mock, ownerID, "m", rgb, "0", "0", 10, 80)
	now := time.Now()
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "created_at", "updated_at"}).
			AddRow(3, "Polygon", now, now))
}

func expectTradeChannels(mock sqlmock.Sqlmock, userID uint64, name string) {
	t := time.Now()
	mock.ExpectQuery("trade_notification_channels").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"name", "phone", "phone_verified_at", "notifications"}).
			AddRow(name, "09120000000", t, `{"trades_sms":true,"trades_email":true}`))
}

func TestMarketplaceService_BuyFeature_Limited_NoLimitation(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniTradingLimited, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	mock.ExpectQuery("FROM feature_limits").
		WithArgs("p1", "p1").
		WillReturnError(sql.ErrNoRows)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "پشتیبانی") || strings.Contains(err.Error(), "خطا"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_Limited_YellowTooLow(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Yellow = "1"
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniTradingLimited, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	expectLimitation(mock, true)
	expectBuyerKYCLimited(mock, 2)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "لیتر") || strings.Contains(err.Error(), "رنگ"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_Limited_HappyPath(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Yellow = "1000"
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniTradingLimited, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	expectLimitation(mock, true)
	expectBuyerKYCLimited(mock, 2)
	expectBuySuccessDBTail(mock, 2, 5, "yellow", 0, 0)
	mock.ExpectExec("INSERT INTO limited_feature_purchases").
		WithArgs(uint64(2), uint64(3), uint64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	feat, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.NotNil(t, feat)

	require.Len(t, stub.DeductCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "yellow", Amount: 10}, stub.DeductCalls[0])
	require.GreaterOrEqual(t, len(stub.AddCalls), 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 5, Asset: "yellow", Amount: 10}, stub.AddCalls[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_Limited_AddSellerFails_RollbackBuyer(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Yellow = "1000"
	stub.FailNthAdd = 1
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniTradingLimited, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	expectLimitation(mock, true)
	expectBuyerKYCLimited(mock, 2)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.Error(t, err)
	require.Len(t, stub.DeductCalls, 1)
	require.Len(t, stub.AddCalls, 2)
	assert.Equal(t, uint64(5), stub.AddCalls[0].UserID)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "yellow", Amount: 10}, stub.AddCalls[1])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_RGB_HappyPath(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Yellow = "1000"
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniPriced, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, constants.RGBUserCode)
	expectBuyerKYCSimple(mock, 2)
	expectBuySuccessDBTail(mock, 2, 5, "yellow", 0, 0)
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Len(t, stub.DeductCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "yellow", Amount: 10}, stub.DeductCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 5, Asset: "yellow", Amount: 10}, stub.AddCalls[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_UserToUser_HappyPath(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniSoldAndPriced, "100", "100", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	expectBuyerKYCSimple(mock, 2)
	mock.ExpectQuery("SELECT id FROM users WHERE code").
		WithArgs(constants.RGBUserCode).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(99)))
	mock.ExpectExec("INSERT INTO trades").
		WithArgs(uint64(1), uint64(2), uint64(5), 100.0, 100.0).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO comissions").
		WithArgs(uint64(7), 10.0, 10.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE features SET owner_id").
		WithArgs(uint64(2), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET rgb = \\?, owner = \\?, label").
		WithArgs(constants.ChangeStatusToSoldAndNotPriced("m"), "buyer", "", constants.DefaultPublicPricingLimit, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, user_id, feature_id, asset, amount").
		WithArgs(uint64(1), uint64(5)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? AND user_id").
		WithArgs(uint64(1), uint64(5)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? ORDER BY").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO feature_hourly_profits").
		WithArgs(uint64(2), uint64(1), "yellow", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(1), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET deleted_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)

	require.Len(t, stub.DeductCalls, 2)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 105}, stub.DeductCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "irr", Amount: 105}, stub.DeductCalls[1])
	assert.Equal(t, testutil.BalanceOp{UserID: 5, Asset: "psc", Amount: 95}, stub.AddCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 5, Asset: "irr", Amount: 95}, stub.AddCalls[1])
	assert.Equal(t, testutil.BalanceOp{UserID: 99, Asset: "psc", Amount: 10}, stub.AddCalls[2])
	assert.Equal(t, testutil.BalanceOp{UserID: 99, Asset: "irr", Amount: 10}, stub.AddCalls[3])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_UserToUser_IRRDeductFails_RollbackPSC(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailNthDeduct = 2
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniSoldAndPriced, "100", "100", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	expectBuyerKYCSimple(mock, 2)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.Error(t, err)
	require.Len(t, stub.DeductCalls, 2)
	require.Len(t, stub.AddCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 105}, stub.AddCalls[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectSendBuyRequestThroughRates(mock sqlmock.Sqlmock) {
	expectFeatureFindByIDRGB(mock, 3, "m", "d", "0", "0", 1000, 80)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(uint64(2), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
}

func TestMarketplaceService_SendBuyRequest_WalletSuccess(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectSendBuyRequestThroughRates(mock)
	mock.ExpectExec("INSERT INTO buy_feature_requests").
		WithArgs(uint64(2), uint64(3), uint64(1), "", 500.0, 500.0).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("INSERT INTO locked_wallets").
		WithArgs(uint64(9), uint64(1), 525.0, 525.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))

	req, err := svc.SendBuyRequest(context.Background(), &pb.SendBuyRequestRequest{
		BuyerId: 2, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Len(t, stub.DeductCalls, 2)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 525}, stub.DeductCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "irr", Amount: 525}, stub.DeductCalls[1])
	require.Len(t, stub.CreateTxCalls, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_SendBuyRequest_IRRDeductFails_RollbackPSC(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailNthDeduct = 2
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectSendBuyRequestThroughRates(mock)
	mock.ExpectExec("INSERT INTO buy_feature_requests").
		WithArgs(uint64(2), uint64(3), uint64(1), "", 500.0, 500.0).
		WillReturnResult(sqlmock.NewResult(9, 1))

	_, err := svc.SendBuyRequest(context.Background(), &pb.SendBuyRequestRequest{
		BuyerId: 2, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.Error(t, err)
	require.Len(t, stub.AddCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 525}, stub.AddCalls[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_SendBuyRequest_InsufficientPSC(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Psc = "0"
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectSendBuyRequestThroughRates(mock)

	_, err := svc.SendBuyRequest(context.Background(), &pb.SendBuyRequestRequest{
		BuyerId: 2, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "psc")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_SendBuyRequest_InsufficientIRR(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Irr = "0"
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectSendBuyRequestThroughRates(mock)

	_, err := svc.SendBuyRequest(context.Background(), &pb.SendBuyRequestRequest{
		BuyerId: 2, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ریال")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_AcceptBuyRequest_SellerIRRAddFails_RollbackPSC(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailNthAdd = 2
	svc, mock := newMarketplaceWithWallet(t, stub)

	now := time.Now()
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(9, uint64(2), uint64(3), 1, "n", 100.0, 100.0, 0, nil, now, now))
	expectFeatureFindByIDRGB(mock, 3, "m", "a", "100", "100", 10, 80)
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT id, buy_feature_request_id, feature_id, psc, irr").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 105.0, 105.0, now, now))

	_, err := svc.AcceptBuyRequest(context.Background(), 9, 3)
	require.Error(t, err)
	require.Len(t, stub.AddCalls, 2)
	assert.Equal(t, testutil.BalanceOp{UserID: 3, Asset: "psc", Amount: 95}, stub.AddCalls[0])
	require.Len(t, stub.DeductCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 3, Asset: "psc", Amount: 95}, stub.DeductCalls[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_AcceptBuyRequest_WalletThenCommit(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	now := time.Now()
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(9, uint64(2), uint64(3), 1, "n", 100.0, 100.0, 0, nil, now, now))
	expectFeatureFindByIDRGB(mock, 3, "m", "a", "100", "100", 10, 80)
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT id, buy_feature_request_id, feature_id, psc, irr").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 105.0, 105.0, now, now))
	mock.ExpectQuery("SELECT id FROM users WHERE code").
		WithArgs(constants.RGBUserCode).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(99)))
	mock.ExpectQuery("SELECT id, user_id, feature_id, asset, amount").
		WithArgs(uint64(1), uint64(3)).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO trades").
		WithArgs(uint64(1), uint64(2), uint64(3), 100.0, 100.0).
		WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO comissions").
		WithArgs(uint64(7), 10.0, 10.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE features SET owner_id").
		WithArgs(uint64(2), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT name FROM users WHERE id").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("buyer"))
	mock.ExpectQuery("SELECT birthdate FROM kycs WHERE user_id").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("SET rgb = \\?, owner = \\?, label").
		WithArgs(constants.ChangeStatusToSoldAndNotPriced("m"), "buyer", "", constants.DefaultPublicPricingLimit, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? AND user_id").
		WithArgs(uint64(1), uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? ORDER BY").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO feature_hourly_profits").
		WithArgs(uint64(2), uint64(1), "yellow", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(1), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET status = \\?, updated_at = NOW\\(\\) WHERE id").
		WithArgs(1, uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET deleted_at = NOW\\(\\) WHERE id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM locked_wallets WHERE buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("WHERE feature_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(10, uint64(4), uint64(3), 1, "n", 10.0, 20.0, 0, nil, now, now))
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(10, uint64(4), uint64(3), 1, "n", 10.0, 20.0, 0, nil, now, now))
	mock.ExpectQuery("FROM locked_wallets").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(8, 10, 1, 5.0, 6.0, now, now))
	mock.ExpectExec("DELETE FROM locked_wallets").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET deleted_at = NOW\\(\\) WHERE id").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	req, err := svc.AcceptBuyRequest(context.Background(), 9, 3)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, testutil.BalanceOp{UserID: 3, Asset: "psc", Amount: 95}, stub.AddCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 3, Asset: "irr", Amount: 95}, stub.AddCalls[1])
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectRefundLockedWallet(mock sqlmock.Sqlmock, requestID uint64, sellerOrBuyerRows *sqlmock.Rows) {
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(requestID).
		WillReturnRows(sellerOrBuyerRows)
	now := time.Now()
	mock.ExpectQuery("SELECT id, buy_feature_request_id, feature_id, psc, irr").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, requestID, 1, 10.5, 21.0, now, now))
	mock.ExpectExec("DELETE FROM transactions WHERE transactionable_type").
		WithArgs("App\\Models\\BuyFeatureRequest", requestID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM locked_wallets WHERE buy_feature_request_id").
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM buy_feature_requests WHERE id").
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestMarketplaceService_RejectBuyRequest_RefundsLocked(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectRefundLockedWallet(mock, 9, buyRequestFindRows(3, 2, 0))

	require.NoError(t, svc.RejectBuyRequest(context.Background(), 9, 3))
	require.Len(t, stub.AddCalls, 2)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 10.5}, stub.AddCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "irr", Amount: 21}, stub.AddCalls[1])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_DeleteBuyRequest_RefundsLocked(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)

	expectRefundLockedWallet(mock, 9, buyRequestFindRows(3, 2, 0))

	require.NoError(t, svc.DeleteBuyRequest(context.Background(), 9, 2))
	require.Len(t, stub.AddCalls, 2)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "psc", Amount: 10.5}, stub.AddCalls[0])
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "irr", Amount: 21}, stub.AddCalls[1])
	require.NoError(t, mock.ExpectationsWereMet())
}
