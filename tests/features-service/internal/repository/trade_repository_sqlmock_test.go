package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tradeCols() []string {
	return []string{"id", "feature_id", "buyer_id", "seller_id", "irr_amount", "psc_amount", "date", "created_at", "updated_at"}
}

func TestTradeRepository_Create(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)

	mock.ExpectExec("INSERT INTO trades").
		WithArgs(uint64(1), uint64(2), uint64(3), 50.0, 10.0).
		WillReturnResult(sqlmock.NewResult(9, 1))

	id, err := repo.Create(context.Background(), 1, 2, 3, 50, 10)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTradeRepository_GetLatestForFeature(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM trades").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(tradeCols()).
			AddRow(9, 1, 2, 3, 50.0, 10.0, now, now, now))

	trade, err := repo.GetLatestForFeature(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), trade.ID)

	mock.ExpectQuery("FROM trades").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)
	trade, err = repo.GetLatestForFeature(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, trade)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTradeRepository_GetLatestForFeatureWithSeller(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)
	now := time.Now()

	cols := append(tradeCols(), "seller_user_id", "seller_name", "seller_code")
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(9, 1, 2, 3, 50.0, 10.0, now, now, now, 3, "Ali", "hm-1"))

	trade, seller, err := repo.GetLatestForFeatureWithSeller(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), trade.ID)
	assert.Equal(t, "Ali", seller.Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTradeRepository_GetLatestUnderpricedForSeller(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)
	now := time.Now()

	mock.ExpectQuery("INNER JOIN sell_feature_requests").
		WithArgs(uint64(3), uint64(1)).
		WillReturnRows(sqlmock.NewRows(tradeCols()).
			AddRow(9, 1, 2, 3, 50.0, 10.0, now, now, now))

	trade, err := repo.GetLatestUnderpricedForSeller(context.Background(), 3, 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), trade.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTradeRepository_IsWithin24Hours(t *testing.T) {
	repo := repository.NewTradeRepository(nil)
	assert.False(t, repo.IsWithin24Hours(nil))
	assert.True(t, repo.IsWithin24Hours(&models.Trade{CreatedAt: time.Now().Add(-time.Hour)}))
	assert.False(t, repo.IsWithin24Hours(&models.Trade{CreatedAt: time.Now().Add(-25 * time.Hour)}))
}

func TestTradeRepository_GetTimeRemaining(t *testing.T) {
	repo := repository.NewTradeRepository(nil)
	h, m := repo.GetTimeRemaining(nil)
	assert.Equal(t, 0, h)
	assert.Equal(t, 0, m)

	h, m = repo.GetTimeRemaining(&models.Trade{CreatedAt: time.Now().Add(-25 * time.Hour)})
	assert.Equal(t, 0, h)
	assert.Equal(t, 0, m)

	// Use 30 minutes elapsed so remaining hours stay 23 even after clock truncation.
	h, m = repo.GetTimeRemaining(&models.Trade{CreatedAt: time.Now().Add(-30 * time.Minute)})
	assert.Equal(t, 23, h)
	assert.GreaterOrEqual(t, m, 0)
}

func TestTradeRepository_ListByFeatureWithDetails(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewTradeRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM trades t").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr_amount", "psc_amount",
			"date", "created_at", "buyer_code", "buyer_name",
		}).AddRow(287, 10, 491, 1, 0.0, 0.0, now, now, "hm-2000491", "Parsa"))

	mock.ExpectQuery("FROM transactions").
		WithArgs(`App\Models\Trade`, uint64(287)).
		WillReturnRows(sqlmock.NewRows([]string{"payable_id", "asset", "amount", "action"}).
			AddRow(287, "red", 1250.0, "withdraw"))

	trades, err := repo.ListByFeatureWithDetails(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	assert.Equal(t, uint64(287), trades[0].ID)
	assert.Equal(t, "hm-2000491", trades[0].BuyerCode)
	assert.Equal(t, "Parsa", trades[0].BuyerName)
	require.Len(t, trades[0].Transactions, 1)
	assert.Equal(t, "red", trades[0].Transactions[0].Asset)
	assert.Equal(t, 1250.0, trades[0].Transactions[0].Amount)
	assert.Equal(t, "withdraw", trades[0].Transactions[0].Action)
	require.NoError(t, mock.ExpectationsWereMet())
}
