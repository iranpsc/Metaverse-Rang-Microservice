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

func hourlyProfitJoinCols() []string {
	return []string{
		"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active",
		"created_at", "updated_at", "feature_db_id", "properties_id", "karbari",
	}
}

func hourlyProfitBaseCols() []string {
	return []string{"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active", "created_at", "updated_at"}
}

func TestHourlyProfitRepository_Create_UpdateExisting(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectQuery("SELECT id FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(11)))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(5), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE feature_hourly_profits").
		WithArgs(uint64(2), "yellow", sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id, err := repo.Create(context.Background(), 2, 5, "yellow", 10)
	require.NoError(t, err)
	assert.Equal(t, uint64(11), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_Create_Insert(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectQuery("SELECT id FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(5)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO feature_hourly_profits").
		WithArgs(uint64(2), uint64(5), "yellow", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(12, 1))

	id, err := repo.Create(context.Background(), 2, 5, "yellow", 10)
	require.NoError(t, err)
	assert.Equal(t, uint64(12), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_FindByID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()

	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows(hourlyProfitJoinCols()).
			AddRow(11, 2, 5, "yellow", 1.5, now, true, now, now, 5, "p1", "m"))

	p, err := repo.FindByID(context.Background(), 11)
	require.NoError(t, err)
	assert.Equal(t, "m", p.Karbari)

	mock.ExpectQuery("FROM feature_hourly_profits fhp").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)
	_, err = repo.FindByID(context.Background(), 99)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_FindByUserID_HasMore(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()
	pageSize := int32(2)

	rows := sqlmock.NewRows(hourlyProfitJoinCols())
	for i := 1; i <= 3; i++ {
		rows.AddRow(i, 2, i, "yellow", float64(i), now, true, now, now, i, "p", "m")
	}
	mock.ExpectQuery("WHERE fhp.user_id").
		WithArgs(uint64(2), pageSize+1, int32(0)).
		WillReturnRows(rows)

	profits, hasMore, err := repo.FindByUserID(context.Background(), 2, 1, pageSize)
	require.NoError(t, err)
	assert.True(t, hasMore)
	assert.Len(t, profits, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_GetByFeatureAndUser(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()

	mock.ExpectQuery("WHERE feature_id = \\? AND user_id = \\?").
		WithArgs(uint64(5), uint64(2)).
		WillReturnRows(sqlmock.NewRows(hourlyProfitBaseCols()).
			AddRow(11, 2, 5, "yellow", 3.0, now, true, now, now))

	p, err := repo.GetByFeatureAndUser(context.Background(), 5, 2)
	require.NoError(t, err)
	assert.Equal(t, 3.0, p.Amount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_GetAllByUserAndKarbari(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()

	mock.ExpectQuery("fp.karbari").
		WithArgs(uint64(2), "m").
		WillReturnRows(sqlmock.NewRows(hourlyProfitBaseCols()).
			AddRow(11, 2, 5, "yellow", 3.0, now, true, now, now))

	list, err := repo.GetAllByUserAndKarbari(context.Background(), 2, "m")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_GetTotalsByKarbari(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectQuery("GROUP BY fp.karbari").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"karbari", "total"}).
			AddRow("m", 1.5).
			AddRow("t", 2.25).
			AddRow("a", 0.5))

	m, tej, a, err := repo.GetTotalsByKarbari(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "1.500000", m)
	assert.Equal(t, "2.250000", tej)
	assert.Equal(t, "0.500000", a)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_ResetProfitAndUpdateDeadline(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectExec("SET amount = 0, dead_line").
		WithArgs(sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ResetProfitAndUpdateDeadline(context.Background(), 11, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_TransferProfitToNewOwner(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectQuery("WHERE feature_id = \\? AND user_id = \\?").
		WithArgs(uint64(5), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "asset"}).AddRow(uint64(11), "yellow"))
	mock.ExpectExec("UPDATE feature_hourly_profits").
		WithArgs(uint64(9), "blue", sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(5), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, repo.TransferProfitToNewOwner(context.Background(), 5, 2, 9, "blue", 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_TransferProfitToNewOwnerWithTx(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery("WHERE feature_id = \\? AND user_id = \\?").
		WithArgs(uint64(5), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "asset"}).AddRow(uint64(11), "yellow"))
	mock.ExpectExec("UPDATE feature_hourly_profits").
		WithArgs(uint64(9), "yellow", sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM feature_hourly_profits WHERE feature_id").
		WithArgs(uint64(5), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, repo.TransferProfitToNewOwnerWithTx(context.Background(), tx, 5, 2, 9, "", 10))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_CalculateAndUpdateProfits(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectQuery("SELECT fhp.id, fhp.feature_id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id"}).AddRow(uint64(11), uint64(5)))
	mock.ExpectQuery("SELECT stability FROM feature_properties").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"stability"}).AddRow(100.0))
	mock.ExpectExec("SET amount = amount \\+").
		WithArgs(sqlmock.AnyArg(), uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	n, err := repo.CalculateAndUpdateProfits(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_DeactivateAndActivate(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)

	mock.ExpectExec("SET is_active = 0").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("SET is_active = 1").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.DeactivateProfitsForFeature(context.Background(), 5))
	require.NoError(t, repo.ActivateProfitsForFeature(context.Background(), 5))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHourlyProfitRepository_FindOldestByUserID(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	repo := repository.NewHourlyProfitRepository(db)
	now := time.Now()

	mock.ExpectQuery("WHERE user_id = \\?\\s+ORDER BY dead_line ASC").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows(hourlyProfitBaseCols()).
			AddRow(11, 2, 5, "yellow", 0.0, now, true, now, now))

	p, err := repo.FindOldestByUserID(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, uint64(11), p.ID)

	mock.ExpectQuery("WHERE user_id = \\?\\s+ORDER BY dead_line ASC").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	p, err = repo.FindOldestByUserID(context.Background(), 3)
	require.NoError(t, err)
	assert.Nil(t, p)
	require.NoError(t, mock.ExpectationsWereMet())
}
