package service_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"metarang/dynasty-service/tests/internal/testutil"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"metarang/dynasty-service/internal/service"

	"metarang/dynasty-service/internal/repository"
)

type stubWallet struct {
	pscCalls  int
	pscAmount float64
	pscUser   uint64
	satCalls  int
	pscErr    error
	satErr    error
}

func (s *stubWallet) IncrementWalletPSC(ctx context.Context, userID uint64, amount float64) error {
	s.pscCalls++
	s.pscAmount = amount
	s.pscUser = userID
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
		assert.Equal(t, 10.0, wallet.pscAmount)
		assert.Equal(t, userID, wallet.pscUser)
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

func TestPrizeService_GetAllAndGetPrize(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	prizeCols := []string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}

	t.Run("GetAllPrizes", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)

		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM dynasty_prizes`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs(int32(10), int32(0)).
			WillReturnRows(sqlmock.NewRows(prizeCols).
				AddRow(1, "offspring", 0.1, 0.2, 0.3, 0.4, 1000, now, now))

		prizes, total, err := svc.GetAllPrizes(ctx, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		require.Len(t, prizes, 1)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetPrizeSuccess", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)

		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs(uint64(4)).
			WillReturnRows(sqlmock.NewRows(prizeCols).
				AddRow(4, "brother", 0.1, 0.2, 0.3, 0.4, 1000, now, now))
		prize, err := svc.GetPrize(ctx, 4)
		require.NoError(t, err)
		require.NotNil(t, prize)
		assert.Equal(t, uint64(4), prize.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetPrizeNotFound", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)

		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs(uint64(4)).
			WillReturnError(sql.ErrNoRows)
		prize, err := svc.GetPrize(ctx, 4)
		assert.Error(t, err)
		assert.Nil(t, prize)
		assert.Contains(t, err.Error(), "prize not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetPrizeRepoError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)

		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs(uint64(4)).
			WillReturnError(errors.New("db"))
		_, err = svc.GetPrize(ctx, 4)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get prize")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPrizeService_GetUserReceivedPrizes_PaginationAndError(t *testing.T) {
	ctx := context.Background()
	userID := uint64(1)
	now := time.Now()
	cols := []string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.id", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}

	t.Run("QueryError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(userID).
			WillReturnError(errors.New("db"))
		_, _, err = svc.GetUserReceivedPrizes(ctx, userID, 1, 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user prizes")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PagePastEndReturnsEmpty", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows(cols).
				AddRow(1, userID, 1, "a", now, now, 1, "offspring", 0.1, 0.05, 0.02, 0.03, 1000))
		prizes, total, err := svc.GetUserReceivedPrizes(ctx, userID, 5, 10)
		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Empty(t, prizes)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PartialLastPage", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)
		rows := sqlmock.NewRows(cols).
			AddRow(1, userID, 1, "a", now, now, 1, "offspring", 0.1, 0.05, 0.02, 0.03, 1000).
			AddRow(2, userID, 1, "b", now, now, 1, "offspring", 0.1, 0.05, 0.02, 0.03, 1000).
			AddRow(3, userID, 1, "c", now, now, 1, "offspring", 0.1, 0.05, 0.02, 0.03, 1000)
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(userID).
			WillReturnRows(rows)
		prizes, total, err := svc.GetUserReceivedPrizes(ctx, userID, 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int32(3), total)
		require.Len(t, prizes, 1)
		assert.Equal(t, uint64(3), prizes[0].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPrizeService_GetReceivedPrize(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)
	now := time.Now()

	mock.ExpectQuery("SELECT rp.id, rp.user_id").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
			AddRow(8, 1, 1, "msg", now, now, "offspring", 0.1, 0.05, 0.02, 0.03, 1000))
	got, err := svc.GetReceivedPrize(context.Background(), 8)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(8), got.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrizeService_ClaimPrize_RemainingErrors(t *testing.T) {
	ctx := context.Background()
	receivedPrizeID, userID := uint64(1), uint64(1)
	buildReceivedRows := func(uid uint64) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).
			AddRow(receivedPrizeID, uid, 1, "Congratulations!", time.Now(), time.Now(), "offspring", 0.1, 0.05, 0.02, 0.03, 1000)
	}

	t.Run("DependenciesNotConfigured", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, &stubWallet{})
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prize claim dependencies not configured")
	})

	t.Run("GetReceivedPrizeError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnError(errors.New("db"))
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get received prize")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PSCRateError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))
		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnError(errors.New("rate failed"))
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "psc rate")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SatisfactionWalletError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{satErr: errors.New("sat down")})
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))
		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wallet satisfaction credit")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("BeginTxError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))
		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "begin tx")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ApplyMultipliersError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
		mock.ExpectQuery("SELECT rp.id, rp.user_id").
			WithArgs(receivedPrizeID).
			WillReturnRows(buildReceivedRows(userID))
		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE user_variables SET").
			WillReturnError(errors.New("uv failed"))
		mock.ExpectRollback()
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteReceivedPrizeError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
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
			WillReturnError(errors.New("delete failed"))
		mock.ExpectRollback()
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CommitError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), repository.NewVariableRepository(db), repository.NewUserVariableRepository(db), &stubWallet{})
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
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
		err = svc.ClaimPrize(ctx, receivedPrizeID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "commit prize claim")
		require.NoError(t, mock.ExpectationsWereMet())
	})
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

func TestPrizeService_ClaimPrize_CommercialRate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rate    float64
		err     error
		success bool
	}{
		{"fractional", 400, nil, true}, {"conversion overflow", math.SmallestNonzeroFloat64, nil, false}, {"zero", 0, nil, false}, {"negative", -1, nil, false},
		{"nan", math.NaN(), nil, false}, {"infinite", math.Inf(1), nil, false}, {"unavailable", 0, errors.New("down"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			commercial, dependency := testutil.StartCommercial(t, tc.rate, tc.err)
			svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), commercial, repository.NewUserVariableRepository(db), commercial)
			mock.ExpectQuery("SELECT rp.id, rp.user_id").WithArgs(uint64(1)).WillReturnRows(
				sqlmock.NewRows([]string{"id", "user_id", "prize_id", "message", "created_at", "updated_at", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc"}).
					AddRow(1, 7, 2, "prize", time.Now(), time.Now(), "brother", 0.1, 0.2, 0.3, 0.4, 1000))
			if tc.success {
				mock.ExpectBegin()
				mock.ExpectExec("UPDATE user_variables SET").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("DELETE FROM received_prizes WHERE id").WithArgs(uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = svc.ClaimPrize(context.Background(), 1, 7)
			if tc.success {
				require.NoError(t, err)
				require.Len(t, dependency.Credits, 2)
				credit := <-dependency.Credits
				assert.Equal(t, "psc", credit.Asset)
				assert.Equal(t, uint64(7), credit.UserId)
				assert.Equal(t, 2.5, credit.Amount)
			} else {
				require.Error(t, err)
				assert.Empty(t, dependency.Credits)
			}
			assert.Equal(t, int32(1), dependency.RateCalls.Load())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
