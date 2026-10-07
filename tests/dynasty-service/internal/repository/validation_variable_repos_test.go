package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

func TestValidationRepository(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewValidationRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	ok, err := repo.CheckPendingRequest(ctx, 1, 2)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	ok, err = repo.CheckRejectedRequest(ctx, 1, 2)
	require.NoError(t, err)
	assert.False(t, ok)

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	ok, err = repo.CheckUserInFamily(ctx, 3)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(9), "brother").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	n, err := repo.CountFamilyMembersByRelationship(ctx, 9, "brother")
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	mock.ExpectQuery("SELECT id FROM families").WithArgs(uint64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(44))
	fid, err := repo.GetFamilyByDynastyID(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, uint64(44), fid)

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	ok, err = repo.CheckUserHasDynasty(ctx, 5)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT verified, DM").WithArgs(uint64(6)).
		WillReturnRows(sqlmock.NewRows([]string{"verified", "DM"}).AddRow(false, true))
	ok, err = repo.CheckUserDMPermission(ctx, 6)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT verified, DM").WithArgs(uint64(7)).
		WillReturnError(sql.ErrNoRows)
	ok, err = repo.CheckUserDMPermission(ctx, 7)
	require.NoError(t, err)
	assert.True(t, ok)

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(1), uint64(2)).
		WillReturnError(errors.New("db"))
	_, err = repo.CheckPendingRequest(ctx, 1, 2)
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVariableRepository_GetPriceByAsset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewVariableRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT price FROM variables").WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(int64(250)))
	price, err := repo.GetPriceByAsset(ctx, "psc")
	require.NoError(t, err)
	assert.Equal(t, 250.0, price)

	mock.ExpectQuery("SELECT price FROM variables").WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	price, err = repo.GetPriceByAsset(ctx, "missing")
	require.NoError(t, err)
	assert.Equal(t, 1.0, price)

	mock.ExpectQuery("SELECT price FROM variables").WithArgs("zero").
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(int64(0)))
	price, err = repo.GetPriceByAsset(ctx, "zero")
	require.NoError(t, err)
	assert.Equal(t, 1.0, price)

	mock.ExpectQuery("SELECT price FROM variables").WithArgs("err").
		WillReturnError(errors.New("boom"))
	_, err = repo.GetPriceByAsset(ctx, "err")
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserVariableRepository_ApplyDynastyPrizeMultipliers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserVariableRepository(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE user_variables SET").
		WithArgs(0.1, 0.2, 0.3, uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.ApplyDynastyPrizeMultipliers(ctx, nil, 5, 0.1, 0.2, 0.3))

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	mock.ExpectExec("UPDATE user_variables SET").
		WithArgs(0.1, 0.2, 0.3, uint64(6)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.ApplyDynastyPrizeMultipliers(ctx, tx, 6, 0.1, 0.2, 0.3))

	mock.ExpectExec("UPDATE user_variables SET").
		WithArgs(0.1, 0.2, 0.3, uint64(7)).
		WillReturnError(errors.New("fail"))
	err = repo.ApplyDynastyPrizeMultipliers(ctx, nil, 7, 0.1, 0.2, 0.3)
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
