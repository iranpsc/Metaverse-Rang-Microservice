package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buyRequestCols() []string {
	return []string{
		"id", "buyer_id", "seller_id", "feature_id", "note", "price_psc", "price_irr",
		"status", "requested_grace_period", "created_at", "updated_at",
	}
}

func TestBuyRequestRepository_GetAllForFeature(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("WHERE feature_id = \\? AND deleted_at IS NULL").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows(buyRequestCols()).
			AddRow(1, 2, 3, 5, "n", 10.0, 20.0, 0, nil, now, now))

	list, err := repo.GetAllForFeature(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, uint64(1), list[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_CancelAllForFeature(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)

	mock.ExpectExec("SET deleted_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	require.NoError(t, repo.CancelAllForFeature(context.Background(), 5))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_Delete(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)

	mock.ExpectExec("DELETE FROM buy_feature_requests WHERE id").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Delete(context.Background(), 1))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_UpdateStatusWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("SET status = \\?, updated_at = NOW\\(\\) WHERE id").
		WithArgs(1, uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatusWithTx(context.Background(), tx, 9, 1))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_SoftDeleteWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("SET deleted_at = NOW\\(\\) WHERE id").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, repo.SoftDeleteWithTx(context.Background(), tx, 9))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_FindByID_NoRows(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)

	req, err := repo.FindByID(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, req)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuyRequestRepository_ListKeepsRowsWithVarcharGracePeriod(t *testing.T) {
	now := time.Now()
	grace := []byte("2026-09-30 17:42:00")

	t.Run("buyer", func(t *testing.T) {
		db, mock := testutil.NewSQLMock(t)
		repo := repository.NewBuyRequestRepository(db)
		mock.ExpectQuery("WHERE buyer_id = \\? AND deleted_at IS NULL").
			WithArgs(uint64(2)).
			WillReturnRows(sqlmock.NewRows(buyRequestCols()).
				AddRow(1, 2, 3, 5, "n", 10.0, 20.0, 0, grace, now, now))

		list, err := repo.ListByBuyerID(context.Background(), 2)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, uint64(1), list[0].ID)
		require.True(t, list[0].RequestedGracePeriod.Valid)
		assert.Equal(t, 2026, list[0].RequestedGracePeriod.Time.Year())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("seller", func(t *testing.T) {
		db, mock := testutil.NewSQLMock(t)
		repo := repository.NewBuyRequestRepository(db)
		mock.ExpectQuery("WHERE seller_id = \\? AND deleted_at IS NULL").
			WithArgs(uint64(3)).
			WillReturnRows(sqlmock.NewRows(buyRequestCols()).
				AddRow(1, 2, 3, 5, "n", 10.0, 20.0, 0, grace, now, now))

		list, err := repo.ListBySellerID(context.Background(), 3)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, uint64(1), list[0].ID)
		require.True(t, list[0].RequestedGracePeriod.Valid)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestBuyRequestRepository_FindByID_VarcharGracePeriod(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewBuyRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows(buyRequestCols()).
			AddRow(9, 2, 3, 5, "n", 10.0, 20.0, 0, []byte("2026-09-30 17:42:00"), now, now))

	req, err := repo.FindByID(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.True(t, req.RequestedGracePeriod.Valid)
	assert.Equal(t, 30, req.RequestedGracePeriod.Time.Day())
	require.NoError(t, mock.ExpectationsWereMet())
}
