package repository_test

import (
	"context"
	"testing"
	"time"

	"metarang/features-service/internal/repository"
	"metarang/features-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockedAssetRepository_Create(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewLockedAssetRepository(db)

	mock.ExpectExec("INSERT INTO locked_assets").
		WithArgs(uint64(1), uint64(10), uint64(20), 105.0, 210.0).
		WillReturnResult(sqlmock.NewResult(7, 1))

	id, err := repo.Create(context.Background(), 1, 10, 20, 105, 210)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockedAssetRepository_GetByBuyRequestID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewLockedAssetRepository(db)
	now := time.Now()

	mock.ExpectQuery("SELECT id, buy_feature_request_id, feature_id, psc, irr").
		WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 10, 20, 105.0, 210.0, now, now))

	asset, err := repo.GetByBuyRequestID(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), asset.ID)
	assert.Equal(t, 105.0, asset.PSC)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockedAssetRepository_Delete(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewLockedAssetRepository(db)

	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Delete(context.Background(), 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockedAssetRepository_DeleteAllForFeature(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewLockedAssetRepository(db)

	mock.ExpectExec("DELETE FROM locked_assets WHERE feature_id").
		WithArgs(uint64(20)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	require.NoError(t, repo.DeleteAllForFeature(context.Background(), 20))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockedAssetRepository_DeleteWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewLockedAssetRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM locked_assets WHERE buy_feature_request_id").
		WithArgs(uint64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, repo.DeleteWithTx(context.Background(), tx, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}
