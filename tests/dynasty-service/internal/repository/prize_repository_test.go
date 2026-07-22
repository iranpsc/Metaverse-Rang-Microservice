package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

func TestPrizeRepository_GetAwardDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewPrizeRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("FROM dynasty_prizes").WithArgs("offspring").WillReturnRows(sqlmock.NewRows([]string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}).AddRow(1, "offspring", 0.1, 0.2, 0.3, 0.4, 1000, now, now))
	p, err := r.GetPrizeByRelationship(ctx, "offspring")
	require.NoError(t, err)
	require.NotNil(t, p)

	mock.ExpectExec("INSERT INTO received_prizes").WithArgs(uint64(1), uint64(1), "msg").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, r.AwardPrize(ctx, 1, 1, "msg"))

	mock.ExpectExec("DELETE FROM received_prizes WHERE id").WithArgs(uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.DeleteReceivedPrize(ctx, 9))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrizeRepository_GetUserReceivedPrizes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewPrizeRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("FROM received_prizes rp").WithArgs(uint64(2)).WillReturnRows(sqlmock.NewRows([]string{"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at", "dp.id", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase", "dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc"}).AddRow(3, 2, 1, "m", now, now, 1, "offspring", 0.1, 0.2, 0.3, 0.4, 1000))
	items, err := r.GetUserReceivedPrizes(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	require.NotNil(t, items[0].Prize)

	require.NoError(t, mock.ExpectationsWereMet())
}
