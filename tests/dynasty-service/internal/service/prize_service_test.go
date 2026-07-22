package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

type stubWallet struct {
	pscCalls int
	satCalls int
	pscErr   error
	satErr   error
}

func (s *stubWallet) IncrementWalletPSC(ctx context.Context, userID uint64, amount float64) error {
	s.pscCalls++
	return s.pscErr
}

func (s *stubWallet) IncrementSatisfaction(ctx context.Context, userID uint64, amount float64) error {
	s.satCalls++
	return s.satErr
}

func TestPrizeService_GetUserReceivedPrizes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	prizeRepo := repository.NewPrizeRepository(db)
	svc := service.NewPrizeService(db, prizeRepo, nil, nil, nil)

	ctx := context.Background()
	userID := uint64(1)

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.id", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
				AddRow(1, userID, 1, "Congratulations!", time.Now(), time.Now(), 1, "offspring", 0.1, 0.05, 0.02, 0.03, 1000))

		prizes, total, err := svc.GetUserReceivedPrizes(ctx, userID, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, prizes, 1)
		if len(prizes) > 0 {
			assert.Equal(t, userID, prizes[0].UserID)
			assert.NotNil(t, prizes[0].Prize)
		}
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrizeService_ClaimPrize(t *testing.T) {
	ctx := context.Background()
	receivedPrizeID := uint64(1)
	userID := uint64(1)

	buildReceivedRows := func(uid uint64) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
			AddRow(receivedPrizeID, uid, 1, "Congratulations!", time.Now(), time.Now(), "offspring", 0.1, 0.05, 0.02, 0.03, 1000)
	}

	t.Run("Success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		prizeRepo := repository.NewPrizeRepository(db)
		varRepo := repository.NewVariableRepository(db)
		userVarRepo := repository.NewUserVariableRepository(db)
		wallet := &stubWallet{}

		svc := service.NewPrizeService(db, prizeRepo, varRepo, userVarRepo, wallet)

		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))

		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE user_variables SET").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM received_prizes WHERE id").
			WithArgs(receivedPrizeID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		require.NoError(t, err)
		assert.Equal(t, 1, wallet.pscCalls)
		assert.Equal(t, 1, wallet.satCalls)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Unauthorized", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})

		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))

		err = svc.ClaimPrize(ctx, receivedPrizeID, 999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("WalletNotConfigured", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), nil)
		err = svc.ClaimPrize(ctx, 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wallet client not configured")
	})
}

func TestPrizeService_ClaimPrize_SecondClaimNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	receivedPrizeID := uint64(1)
	userID := uint64(1)
	ctx := context.Background()

	prizeRepo := repository.NewPrizeRepository(db)
	varRepo := repository.NewVariableRepository(db)
	userVarRepo := repository.NewUserVariableRepository(db)
	wallet := &stubWallet{}
	svc := service.NewPrizeService(db, prizeRepo, varRepo, userVarRepo, wallet)

	rows := sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
		AddRow(receivedPrizeID, userID, 1, "Congratulations!", time.Now(), time.Now(), "offspring", 0.1, 0.05, 0.02, 0.03, 1000)

	mock.ExpectQuery("SELECT rp.id, rp.user_id").
		WithArgs(receivedPrizeID).
		WillReturnRows(rows)

	mock.ExpectQuery("SELECT price FROM variables WHERE asset").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE user_variables SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM received_prizes WHERE id").
		WithArgs(receivedPrizeID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.ClaimPrize(ctx, receivedPrizeID, userID))

	mock.ExpectQuery("SELECT rp.id, rp.user_id").
		WithArgs(receivedPrizeID).
		WillReturnError(sql.ErrNoRows)

	err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prize not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrizeService_ClaimPrize_WalletErrorBeforeTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	receivedPrizeID := uint64(1)
	userID := uint64(1)
	ctx := context.Background()

	svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{pscErr: errors.New("commercial down")})

	mock.ExpectQuery("SELECT rp.id, rp.user_id").
		WithArgs(receivedPrizeID).
		WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
			AddRow(receivedPrizeID, userID, 1, "Congratulations!", time.Now(), time.Now(), "offspring", 0.1, 0.05, 0.02, 0.03, 1000))

	mock.ExpectQuery("SELECT price FROM variables WHERE asset").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))

	err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wallet psc credit")

	require.NoError(t, mock.ExpectationsWereMet())
}
