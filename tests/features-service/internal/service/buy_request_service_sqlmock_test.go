package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
	"metarang/features-service/tests/internal/testutil"
	"metarang/shared/pkg/logger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBuyRequestService(t *testing.T, stub *testutil.CommercialStub) (*service.BuyRequestService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := testutil.NewSQLMock(t)
	log := logger.NewLogger("buy-request-test")
	svc := service.NewBuyRequestService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewTradeRepository(db),
		repository.NewBuyRequestRepository(db),
		repository.NewSellRequestRepository(db),
		repository.NewLockedAssetRepository(db),
		repository.NewHourlyProfitRepository(db),
		client.NewCommercialClientFromGRPC(stub, stub),
		db,
		log,
	)
	return svc, mock
}

func TestBuyRequestService_SendBuyRequest(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newBuyRequestService(t, stub)

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectExec("INSERT INTO buy_feature_requests").
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("INSERT INTO locked_assets").
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := svc.SendBuyRequest(context.Background(), 2, 1, 500, 500, "note")
	require.NoError(t, err)
	assert.Equal(t, uint64(9), id)

	expectFeatureFindByID(mock, 3, "m", 1000, 90)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnError(sql.ErrNoRows)
	_, err = svc.SendBuyRequest(context.Background(), 2, 1, 1, 1, "")
	require.Error(t, err)
}

func TestBuyRequestService_SendBuyRequest_IRRDeductFails(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailNthDeduct = 2
	svc, mock := newBuyRequestService(t, stub)
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectExec("INSERT INTO buy_feature_requests").
		WillReturnResult(sqlmock.NewResult(9, 1))
	_, err := svc.SendBuyRequest(context.Background(), 2, 1, 500, 500, "note")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IRR")
}

func TestBuyRequestService_SendBuyRequest_Insufficient(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Psc = "0"
	stub.Irr = "0"
	svc, mock := newBuyRequestService(t, stub)
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	_, err := svc.SendBuyRequest(context.Background(), 2, 1, 500, 500, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "موجودی")
}

func TestBuyRequestService_RejectDeleteGraceList(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newBuyRequestService(t, stub)
	now := time.Now()

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnError(sql.ErrNoRows)
	require.Error(t, svc.RejectBuyRequest(context.Background(), 9, 3))

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(99, 2, 0))
	err := svc.RejectBuyRequest(context.Background(), 9, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not the seller")

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	mock.ExpectQuery("SELECT id, buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 10.0, 20.0, now, now))
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.RejectBuyRequest(context.Background(), 9, 3))

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 99, 0))
	err = svc.DeleteBuyRequest(context.Background(), 9, 2)
	require.Error(t, err)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	mock.ExpectQuery("SELECT id, buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 10.0, 20.0, now, now))
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.DeleteBuyRequest(context.Background(), 9, 2))

	require.Error(t, svc.UpdateGracePeriod(context.Background(), 9, 3, 0))
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	mock.ExpectExec("SET requested_grace_period").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.UpdateGracePeriod(context.Background(), 9, 3, 5))

	mock.ExpectQuery("WHERE buyer_id").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}))
	list, err := svc.ListBuyRequests(context.Background(), 2)
	require.NoError(t, err)
	assert.Empty(t, list)

	mock.ExpectQuery("WHERE seller_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(3)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	mock.ExpectQuery("SELECT code FROM users").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("hm-2"))
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT code FROM users").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("hm-3"))
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	received, err := svc.ListReceivedBuyRequests(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, received, 1)
	assert.Equal(t, "hm-2", received[0].BuyerCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestService_AcceptBuyRequest_UnderpricedAndNotFound(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newBuyRequestService(t, stub)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnError(sql.ErrConnDone)
	require.Error(t, svc.AcceptBuyRequest(context.Background(), 9, 3))

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(99, 2, 0))
	err := svc.AcceptBuyRequest(context.Background(), 9, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not the seller")

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	now := time.Now()
	mock.ExpectQuery("`limit` < 100").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at",
		}).AddRow(8, 3, 1, 10.0, 20.0, 90, 0, now, now))
	mock.ExpectQuery("INNER JOIN sell_feature_requests").
		WithArgs(uint64(3), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr_amount", "psc_amount", "date", "created_at", "updated_at",
		}).AddRow(9, 1, 2, 3, 50.0, 10.0, now, now.Add(-time.Hour), now))
	err = svc.AcceptBuyRequest(context.Background(), 9, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "۲۴ ساعت")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestService_AcceptBuyRequest_HappyPath(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newBuyRequestService(t, stub)
	now := time.Now()

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT id, buy_feature_request_id").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 10.5, 21.0, now, now))
	mock.ExpectQuery("SELECT id FROM users WHERE code").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(2000000)))
	mock.ExpectExec("INSERT INTO trades").
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO comissions").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE features SET owner_id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT name FROM users").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("buyer"))
	mock.ExpectQuery("SELECT birthdate FROM kycs").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"birthdate"}).AddRow(time.Now().AddDate(-10, 0, 0)))
	mock.ExpectExec("SET rgb = \\?, owner = \\?, label").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, user_id, feature_id, asset, amount").
		WithArgs(uint64(1), uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? AND user_id").
		WithArgs(uint64(1), uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, asset FROM feature_hourly_profits WHERE feature_id = \\? ORDER BY").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO feature_hourly_profits").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET status").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET deleted_at").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("WHERE feature_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(9, 2, 3, 1, "n", 10.0, 20.0, 0, nil, now, now).
			AddRow(10, 4, 3, 1, "n", 10.0, 20.0, 0, nil, now, now))
	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
			"status", "requested_grace_period", "created_at", "updated_at",
		}).AddRow(10, 4, 3, 1, "n", 10.0, 20.0, 0, nil, now, now))
	mock.ExpectQuery("SELECT id, buy_feature_request_id").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(8, 10, 1, 5.0, 5.0, now, now))
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET deleted_at").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, svc.AcceptBuyRequest(context.Background(), 9, 3))
	require.NoError(t, mock.ExpectationsWereMet())
}
