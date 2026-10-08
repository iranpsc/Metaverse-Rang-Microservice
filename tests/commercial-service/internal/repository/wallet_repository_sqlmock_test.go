package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/commercial-service/internal/models"
	"metarang/commercial-service/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWalletRepo(t *testing.T) (repository.WalletRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return repository.NewWalletRepository(db), mock
}

func TestWalletRepository_FindByUserID(t *testing.T) {
	repo, mock := newWalletRepo(t)
	now := time.Now()
	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "psc", "irr", "red", "blue", "yellow", "satisfaction", "effect", "created_at", "updated_at",
		}).AddRow(1, 1, "10", "0", "", "0", "0", "0.10", "1.5", now, now))

	wallet, err := repo.FindByUserID(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, wallet)
	assert.Equal(t, "10", wallet.PSC.String())

	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)
	wallet, err = repo.FindByUserID(context.Background(), 99)
	require.NoError(t, err)
	assert.Nil(t, wallet)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_Create(t *testing.T) {
	repo, mock := newWalletRepo(t)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO wallets").
		WillReturnResult(sqlmock.NewResult(5, 1))

	wallet, err := repo.Create(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), wallet.ID)
	assert.Equal(t, "0.1", wallet.Satisfaction.String())

	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "psc", "irr", "red", "blue", "yellow", "satisfaction", "effect", "created_at", "updated_at",
		}).AddRow(2, 2, "1", "0", "0", "0", "0", "0.10", "0", now, now))

	wallet, err = repo.Create(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), wallet.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_UpdateAndBalanceOps(t *testing.T) {
	repo, mock := newWalletRepo(t)

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Update(context.Background(), &models.Wallet{
		ID: 1, PSC: decimal.NewFromInt(1), IRR: decimal.Zero, Red: decimal.Zero,
		Blue: decimal.Zero, Yellow: decimal.Zero, Satisfaction: decimal.Zero, Effect: decimal.Zero,
	}))

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.DeductBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := repo.DeductBalance(context.Background(), 1, "psc", decimal.NewFromInt(99))
	require.Error(t, err)

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.AddBalance(context.Background(), 1, "psc", decimal.NewFromInt(2)))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_LockAndUnlockBalance(t *testing.T) {
	repo, mock := newWalletRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO locked_assets").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.LockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1), "hold"))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM locked_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.UnlockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_BalanceOpErrors(t *testing.T) {
	repo, mock := newWalletRepo(t)

	mock.ExpectExec("UPDATE wallets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.Update(context.Background(), &models.Wallet{ID: 1}))

	mock.ExpectExec("UPDATE wallets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.AddBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.AddBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectExec("UPDATE wallets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.DeductBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewErrorResult(assert.AnError))
	require.Error(t, repo.DeductBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectBegin().WillReturnError(assert.AnError)
	require.Error(t, repo.LockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1), "hold"))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.LockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1), "hold"))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.LockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1), "hold"))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO locked_assets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.LockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1), "hold"))

	mock.ExpectBegin().WillReturnError(assert.AnError)
	require.Error(t, repo.UnlockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets").
		WillReturnError(assert.AnError)
	require.Error(t, repo.UnlockBalance(context.Background(), 1, "psc", decimal.NewFromInt(1)))

	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(3)).
		WillReturnError(assert.AnError)
	_, err := repo.Create(context.Background(), 3)
	require.Error(t, err)

	mock.ExpectQuery("SELECT id, user_id, psc").
		WithArgs(uint64(4)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO wallets").
		WillReturnError(assert.AnError)
	_, err = repo.Create(context.Background(), 4)
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
