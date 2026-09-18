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

func sellRequestCols() []string {
	return []string{"id", "seller_id", "feature_id", "price_psc", "price_irr", "limit", "status", "created_at", "updated_at"}
}

func TestSellRequestRepository_IsUnderpriced(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	ok, err := repo.IsUnderpriced(context.Background(), 5)
	require.NoError(t, err)
	assert.True(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestUnderpricedForSeller(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("`limit` < 100").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(8, 3, 5, 10.0, 20.0, 90, 0, now, now))

	req, err := repo.GetLatestUnderpricedForSeller(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, 90, req.Limit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_UpdateAllForFeatureToCompleted(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	require.NoError(t, repo.UpdateAllForFeatureToCompleted(context.Background(), 5))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_UpdateAllForFeatureToCompletedWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("SET status = 1, updated_at = NOW\\(\\) WHERE feature_id").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, repo.UpdateAllForFeatureToCompletedWithTx(context.Background(), tx, 5))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_ListByFeatureID_NewestFirst(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)
	older := time.Now().Add(-time.Hour)
	newer := time.Now()

	mock.ExpectQuery("WHERE feature_id = \\?").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(2, 3, 10, 15.0, 25.0, 100, 0, newer, newer).
			AddRow(1, 3, 10, 10.0, 20.0, 90, 0, older, older))

	requests, err := repo.ListByFeatureID(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, requests, 2)
	assert.Equal(t, uint64(2), requests[0].ID)
	assert.Equal(t, uint64(1), requests[1].ID)
	assert.True(t, requests[0].CreatedAt.After(requests[1].CreatedAt) || requests[0].CreatedAt.Equal(requests[1].CreatedAt))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestOpenByFeatureID_NoRows(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectQuery("feature_id = \\? AND status = 0").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)

	req, err := repo.GetLatestOpenByFeatureID(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, req)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestOpenByFeatureIDs_Empty(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	out, err := repo.GetLatestOpenByFeatureIDs(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, out)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_ListByFeatureID_Empty(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectQuery("WHERE feature_id = \\?").
		WithArgs(uint64(99)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()))

	requests, err := repo.ListByFeatureID(context.Background(), 99)
	require.NoError(t, err)
	assert.Empty(t, requests)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestByFeatureID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(2, 3, 10, 15.0, 25.0, 100, 0, now, now))

	req, err := repo.GetLatestByFeatureID(context.Background(), 10)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, uint64(2), req.ID)
	assert.Equal(t, uint64(10), req.FeatureID)
	assert.Equal(t, 0, req.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestByFeatureID_None(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(99)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()))

	req, err := repo.GetLatestByFeatureID(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, req)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSellRequestRepository_GetLatestByFeatureID_CompletedStatus(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewSellRequestRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM sell_feature_requests").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows(sellRequestCols()).
			AddRow(5, 3, 10, 15.0, 25.0, 100, 1, now, now))

	req, err := repo.GetLatestByFeatureID(context.Background(), 10)
	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, 1, req.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
