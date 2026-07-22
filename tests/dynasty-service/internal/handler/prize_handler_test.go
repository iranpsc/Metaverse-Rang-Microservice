package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"

	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	dynastypb "metarang/shared/pb/dynasty"
)

type walletStub struct{}

func (w *walletStub) IncrementWalletPSC(ctx context.Context, userID uint64, amount float64) error {
	return nil
}
func (w *walletStub) IncrementSatisfaction(ctx context.Context, userID uint64, amount float64) error {
	return nil
}

func TestPrizeHandler_NilService(t *testing.T) {
	h := handler.NewPrizeHandler(nil)
	_, err := h.GetPrizes(context.Background(), &dynastypb.GetPrizesRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestPrizeHandler_GetPrizeAndClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	now := time.Now()

	pr := repository.NewPrizeRepository(db)
	vr := repository.NewVariableRepository(db)
	uv := repository.NewUserVariableRepository(db)
	svc := service.NewPrizeService(db, pr, vr, uv, &walletStub{})
	h := handler.NewPrizeHandler(svc)

	mock.ExpectQuery("SELECT rp.id, rp.user_id").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).AddRow(1, 2, 7, "msg", now, now, "offspring", 0.1, 0.2, 0.3, 0.4, 1000))
	resp, err := h.GetPrize(ctx, &dynastypb.GetPrizeRequest{PrizeId: 1})
	require.NoError(t, err)
	require.NotNil(t, resp.Prize)

	mock.ExpectQuery("SELECT rp.id, rp.user_id").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).AddRow(1, 2, 7, "msg", now, now, "offspring", 0.1, 0.2, 0.3, 0.4, 1000))
	mock.ExpectQuery("SELECT price FROM variables WHERE asset").WithArgs("psc").WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE user_variables SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM received_prizes WHERE id").WithArgs(uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	_, err = h.ClaimPrize(ctx, &dynastypb.ClaimPrizeRequest{PrizeId: 1, UserId: 2})
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
