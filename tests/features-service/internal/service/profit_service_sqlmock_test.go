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

func newProfitSQLMock(t *testing.T) (service.ProfitServiceInterface, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := testutil.NewSQLMock(t)
	svc := service.NewProfitService(
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		nil, nil, db, logger.NewLogger("features-test"),
	)
	return svc, mock
}

func profitFindByIDRows(userID uint64, amount float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active",
		"created_at", "updated_at", "feature_db_id", "properties_id", "karbari",
	}).AddRow(11, userID, 5, "yellow", amount, now, true, now, now, 5, "p1", "m")
}

func TestProfitService_GetSingleProfit_NotFound(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnError(sql.ErrNoRows)

	_, err := svc.GetSingleProfit(context.Background(), 11, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profit not found")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetSingleProfit_Unauthorized(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(profitFindByIDRows(99, 1.0))

	_, err := svc.GetSingleProfit(context.Background(), 11, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetSingleProfit_ZeroAmountSkipsWallet(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(profitFindByIDRows(2, 0))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"withdraw_profit"}).AddRow(10))
	mock.ExpectExec("SET amount = 0, dead_line").
		WithArgs(sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(profitFindByIDRows(2, 0))

	p, err := svc.GetSingleProfit(context.Background(), 11, 2)
	require.NoError(t, err)
	assert.Equal(t, uint64(11), p.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetProfitsByApplication_InvalidKarbari_NoDB(t *testing.T) {
	svc, _ := newProfitSQLMock(t)
	_, err := svc.GetProfitsByApplication(context.Background(), 2, "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid karbari")
}

func TestProfitService_GetHourlyProfits_Defaults(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	now := time.Now()
	joinCols := []string{
		"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active",
		"created_at", "updated_at", "feature_db_id", "properties_id", "karbari",
	}
	mock.ExpectQuery("WHERE fhp.user_id").
		WithArgs(uint64(2), int32(11), int32(0)).
		WillReturnRows(sqlmock.NewRows(joinCols).
			AddRow(1, 2, 5, "yellow", 1.0, now, true, now, now, 5, "p1", "m"))
	mock.ExpectQuery("GROUP BY fp.karbari").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"karbari", "total"}).AddRow("m", 1.0))

	list, tm, tt, ta, hasMore, err := svc.GetHourlyProfits(context.Background(), 2, 0, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.False(t, hasMore)
	assert.Equal(t, "1.00", tm)
	assert.Equal(t, "0.00", tt)
	assert.Equal(t, "0.00", ta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetHourlyProfitTimePercentage_Nil(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	mock.ExpectQuery("ORDER BY dead_line ASC").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)

	pct, err := svc.GetHourlyProfitTimePercentage(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, 0.0, pct)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_GetHourlyProfitTimePercentage_FromLastWithdraw(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	now := time.Now()
	deadline := now.Add(5 * 24 * time.Hour)
	updatedAt := now.Add(-30 * time.Minute)

	mock.ExpectQuery("ORDER BY dead_line ASC").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active", "created_at", "updated_at",
		}).AddRow(11, 2, 5, "yellow", 1.0, deadline, true, now.Add(-10*24*time.Hour), updatedAt))
	mock.ExpectQuery("SELECT withdraw_profit FROM user_variables").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"withdraw_profit"}).AddRow(10))

	pct, err := svc.GetHourlyProfitTimePercentage(context.Background(), 2)
	require.NoError(t, err)
	assert.InDelta(t, 50.0, pct, 0.1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_RunHourlyProfitCalculation_Zero(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	mock.ExpectQuery("SELECT fhp.id, fhp.feature_id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id"}))

	n, err := svc.RunHourlyProfitCalculation(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_RunHourlyProfitCalculation_Cancelled(t *testing.T) {
	svc, mock := newProfitSQLMock(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.RunHourlyProfitCalculation(ctx)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProfitService_TransferProfitOnSale(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	stub := testutil.NewCommercialStub()
	svc := service.NewProfitService(
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		client.NewCommercialClientFromGRPC(stub, stub),
		nil, db, logger.NewLogger("features-test"),
	)
	now := time.Now()
	mock.ExpectQuery("FROM feature_hourly_profits").
		WithArgs(uint64(1), uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active", "created_at", "updated_at",
		}).AddRow(11, 3, 1, "yellow", 4.5, now, true, now, now))
	mock.ExpectQuery("ORDER BY id DESC LIMIT 1").
		WithArgs(uint64(1), uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "asset"}).AddRow(11, "yellow"))
	mock.ExpectExec("SET user_id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, svc.TransferProfitOnSale(context.Background(), 1, 3, 2, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}
