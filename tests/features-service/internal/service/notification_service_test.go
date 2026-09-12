package service_test

import (
	"context"
	"database/sql"
	"errors"
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

func TestMarketplaceService_BuyFeature_Limited_SendsRGBPurchaseNotification(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	wallet.Yellow = "1000"
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniTradingLimited, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, "user-5")
	expectLimitation(mock, true)
	expectBuyerKYCLimited(mock, 2)
	expectBuySuccessDBTail(mock, 2, 5, "yellow", 0, 0)
	mock.ExpectExec("INSERT INTO limited_feature_purchases").
		WithArgs(uint64(2), uint64(3), uint64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectTradeChannels(mock, 2, "buyer")
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Len(t, notif.Calls, 1)
	assert.Equal(t, uint64(2), notif.Calls[0].UserID)
	assert.Equal(t, "BuyFeatureNotification", notif.Calls[0].Type)
	assert.Equal(t, "rgb", notif.Calls[0].Data["purchase_type"])
	assert.Equal(t, constants.ColorMaskoniPersian, notif.Calls[0].Data["color"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_RGB_SendsRGBPurchaseNotification(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	wallet.Yellow = "1000"
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniPriced, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, constants.RGBUserCode)
	expectBuyerKYCSimple(mock, 2)
	expectBuySuccessDBTail(mock, 2, 5, "yellow", 0, 0)
	expectTradeChannels(mock, 2, "buyer")
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Len(t, notif.Calls, 1)
	assert.Equal(t, "BuyFeatureNotification", notif.Calls[0].Type)
	assert.Equal(t, "rgb", notif.Calls[0].Data["purchase_type"])
	assert.Equal(t, "buy-land-metarang", notif.Calls[0].SMSTemplate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_UserToUser_NotifiesBuyerAndSeller(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

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
	expectTradeChannels(mock, 5, "seller")
	expectTradeChannels(mock, 2, "buyer")
	expectTradeChannels(mock, 5, "seller")
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	_, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Len(t, notif.Calls, 2)
	assert.Equal(t, "BuyFeatureNotification", notif.Calls[0].Type)
	assert.Equal(t, uint64(2), notif.Calls[0].UserID)
	assert.Equal(t, "user", notif.Calls[0].Data["purchase_type"])
	assert.Equal(t, "105", notif.Calls[0].Data["psc_amount"])
	assert.Equal(t, "105", notif.Calls[0].Data["irr_amount"])
	assert.Equal(t, "sellFeature", notif.Calls[1].Type)
	assert.Equal(t, uint64(5), notif.Calls[1].UserID)
	assert.Equal(t, "transactions", notif.Calls[1].Data["related-to"])
	assert.Equal(t, "sell-land-metarang", notif.Calls[1].SMSTemplate)
	assert.Contains(t, notif.Calls[1].Message, "واریز شد")
	assert.Contains(t, notif.Calls[1].Message, "95 psc")
	assert.Contains(t, notif.Calls[1].Message, "95")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_BuyFeature_NotificationFailureDoesNotFailPurchase(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	wallet.Yellow = "1000"
	notif := testutil.NewNotificationStub()
	notif.Err = errors.New("notifications down")
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

	expectFeatureFindByIDRGB(mock, 5, "m", constants.MaskoniPriced, "0", "0", 10, 80)
	expectOwnerCode(mock, 5, constants.RGBUserCode)
	expectBuyerKYCSimple(mock, 2)
	expectBuySuccessDBTail(mock, 2, 5, "yellow", 0, 0)
	expectTradeChannels(mock, 2, "buyer")
	expectReloadFeatureAndGeometry(mock, 2, constants.MaskoniSoldAndNotPriced)

	feat, err := svc.BuyFeature(context.Background(), 1, 2)
	require.NoError(t, err)
	require.NotNil(t, feat)
	require.Len(t, notif.Calls, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_SendBuyRequest_NotifiesBuyerAndSeller(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

	expectSendBuyRequestThroughRates(mock)
	mock.ExpectExec("INSERT INTO buy_feature_requests").
		WithArgs(uint64(2), uint64(3), uint64(1), "", 500.0, 500.0).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("INSERT INTO locked_assets").
		WithArgs(uint64(2), uint64(9), uint64(1), 525.0, 525.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	expectTradeChannels(mock, 2, "buyer")
	expectTradeChannels(mock, 3, "seller")

	_, err := svc.SendBuyRequest(context.Background(), &pb.SendBuyRequestRequest{
		BuyerId: 2, FeatureId: 1, PricePsc: "500", PriceIrr: "500",
	})
	require.NoError(t, err)
	require.Len(t, notif.Calls, 2)
	assert.Equal(t, "BuyRequestNotification", notif.Calls[0].Type)
	assert.Equal(t, uint64(2), notif.Calls[0].UserID)
	assert.Equal(t, "buyer", notif.Calls[0].Data["type"])
	assert.Equal(t, "525", notif.Calls[0].Data["price_psc"])
	assert.Equal(t, "525", notif.Calls[0].Data["price_irr"])
	assert.Contains(t, notif.Calls[0].Message, "525")
	assert.Equal(t, "buy-land-request", notif.Calls[0].SMSTemplate)
	assert.Equal(t, uint64(3), notif.Calls[1].UserID)
	assert.Equal(t, "seller", notif.Calls[1].Data["type"])
	assert.Equal(t, "500", notif.Calls[1].Data["price_psc"])
	assert.Equal(t, "500", notif.Calls[1].Data["price_irr"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_AcceptBuyRequest_NotifiesBuyerAndSeller(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

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
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("WHERE feature_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}))
	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("FROM trades").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr_amount", "psc_amount", "date", "created_at", "updated_at",
		}).AddRow(7, 1, 2, 3, 100.0, 100.0, now, now, now))
	mock.ExpectQuery("SELECT name FROM users WHERE id").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("buyer"))
	expectTradeChannels(mock, 3, "seller")
	expectTradeChannels(mock, 2, "buyer")
	expectTradeChannels(mock, 3, "seller")

	_, err := svc.AcceptBuyRequest(context.Background(), 9, 3)
	require.NoError(t, err)
	require.Len(t, notif.Calls, 2)
	assert.Equal(t, "BuyFeatureNotification", notif.Calls[0].Type)
	assert.Equal(t, uint64(2), notif.Calls[0].UserID)
	assert.Equal(t, "user", notif.Calls[0].Data["purchase_type"])
	assert.Equal(t, "105", notif.Calls[0].Data["psc_amount"])
	assert.Equal(t, "105", notif.Calls[0].Data["irr_amount"])
	assert.Equal(t, "sellFeature", notif.Calls[1].Type)
	assert.Equal(t, uint64(3), notif.Calls[1].UserID)
	assert.Equal(t, "7", notif.Calls[1].Data["trade_id"])
	assert.Contains(t, notif.Calls[1].Message, "95 psc")
	assert.Contains(t, notif.Calls[1].Message, "95")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_CreateSellRequest_SendsSellNotification(t *testing.T) {
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc, mock := newMarketplaceWithClients(t, wallet, notif)

	now := time.Now()
	expectFeatureFindByIDRGB(mock, 3, "m", constants.MaskoniSoldAndNotPriced, "0", "0", 1000, 80)
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
		WithArgs(uint64(3), uint64(1), 450.0, 450.0, 90).
		WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec("UPDATE feature_properties SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectTradeChannels(mock, 3, "seller")
	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 450.0, 450.0, 90, 0, now, now))

	out, err := svc.CreateSellRequest(context.Background(), &pb.CreateSellRequestRequest{
		SellerId: 3, FeatureId: 1, MinimumPricePercentage: 90,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, notif.Calls, 1)
	assert.Equal(t, "SellRequestNotification", notif.Calls[0].Type)
	assert.Equal(t, uint64(3), notif.Calls[0].UserID)
	assert.Equal(t, "p1", notif.Calls[0].Data["properties_id"])
	assert.Equal(t, "sell-land-request", notif.Calls[0].SMSTemplate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetSingleProfit_SendsDepositNotification(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc := service.NewProfitService(
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		client.NewCommercialClientFromGRPC(wallet, wallet),
		client.NewNotificationClientFromGRPC(notif),
		db,
		logger.NewLogger("features-test"),
	)

	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(profitFindByIDRows(2, 1.5))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"withdraw_profit"}).AddRow(10))
	mock.ExpectExec("SET amount = 0, dead_line").
		WithArgs(sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(profitFindByIDRows(2, 0))

	_, err := svc.GetSingleProfit(context.Background(), 11, 2)
	require.NoError(t, err)
	require.Len(t, wallet.AddCalls, 1)
	assert.Equal(t, testutil.BalanceOp{UserID: 2, Asset: "yellow", Amount: 1.5}, wallet.AddCalls[0])
	require.Len(t, notif.Calls, 1)
	assert.Equal(t, "FeatureHourlyProfitDeposit", notif.Calls[0].Type)
	assert.Equal(t, "p1", notif.Calls[0].Data["id"])
	assert.Contains(t, notif.Calls[0].Message, "رنگ زرد")
	assert.False(t, notif.Calls[0].SendSMS)
	assert.False(t, notif.Calls[0].SendEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetProfitsByApplication_SendsBatchNotification(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	wallet := testutil.NewCommercialStub()
	notif := testutil.NewNotificationStub()
	svc := service.NewProfitService(
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		client.NewCommercialClientFromGRPC(wallet, wallet),
		client.NewNotificationClientFromGRPC(notif),
		db,
		logger.NewLogger("features-test"),
	)

	now := time.Now()
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"withdraw_profit"}).AddRow(10))
	mock.ExpectQuery("WHERE fhp.user_id = \\? AND fp.karbari").
		WithArgs(uint64(2), "m").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active", "created_at", "updated_at",
		}).AddRow(11, 2, 5, "yellow", 1.25, now, true, now, now).
			AddRow(12, 2, 6, "yellow", 0.75, now, true, now, now))
	mock.ExpectExec("SET amount = 0, dead_line").
		WithArgs(sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET amount = 0, dead_line").
		WithArgs(sqlmock.AnyArg(), uint64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	total, err := svc.GetProfitsByApplication(context.Background(), 2, "m")
	require.NoError(t, err)
	assert.Equal(t, 2.0, total)
	require.Len(t, notif.Calls, 1)
	assert.Equal(t, "FeatureHourlyProfitDeposit", notif.Calls[0].Type)
	assert.Equal(t, "yellow", notif.Calls[0].Data["asset"])
	assert.Equal(t, "مسکونی", notif.Calls[0].Data["karbari"])
	assert.Empty(t, notif.Calls[0].Data["id"])
	assert.Contains(t, notif.Calls[0].Message, "ملک های مسکونی")
	assert.False(t, notif.Calls[0].SendSMS)
	require.NoError(t, mock.ExpectationsWereMet())
}
