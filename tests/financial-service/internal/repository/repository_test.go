package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/repository"
)

func TestOrderRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	ctx := context.Background()
	order := &models.Order{UserID: 1, Asset: "psc", Amount: 100, Status: -138}

	mock.ExpectExec("INSERT INTO orders").
		WithArgs(order.UserID, order.Asset, order.Amount, order.Status, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(42, 1))

	require.NoError(t, repo.Create(ctx, order))
	assert.Equal(t, uint64(42), order.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_FindByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "asset", "amount", "status", "created_at", "updated_at"}).
		AddRow(1, 5, "psc", 100.0, 1, now, now)
	mock.ExpectQuery("SELECT id, user_id, asset, amount, status, created_at, updated_at FROM orders WHERE id = ?").
		WithArgs(uint64(1)).
		WillReturnRows(rows)

	order, err := repo.FindByID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, uint64(1), order.ID)
	assert.Equal(t, "psc", order.Asset)

	mock.ExpectQuery("SELECT id, user_id").WithArgs(uint64(99)).WillReturnError(sql.ErrNoRows)
	order, err = repo.FindByID(ctx, 99)
	require.NoError(t, err)
	assert.Nil(t, order)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_FindByIDWithUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	ctx := context.Background()
	birth := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "asset", "amount", "status", "created_at", "updated_at",
		"id", "name", "email", "phone", "birthdate",
	}).AddRow(1, 5, "psc", 100.0, 1, time.Now(), time.Now(), 5, "User", "u@x.com", "0912", birth)
	mock.ExpectQuery(regexp.QuoteMeta("FROM orders o")).
		WithArgs(uint64(1)).
		WillReturnRows(rows)

	order, user, err := repo.FindByIDWithUser(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, user)
	assert.Equal(t, "0912", user.Phone)
	require.NotNil(t, user.Birthdate)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_UpdateAndDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	ctx := context.Background()
	order := &models.Order{ID: 1, Status: 2}

	mock.ExpectExec("UPDATE orders SET status = \\?, updated_at = \\? WHERE id = \\?").
		WithArgs(order.Status, sqlmock.AnyArg(), order.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Update(ctx, order))

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	mock.ExpectExec("UPDATE orders SET status = \\?, updated_at = \\? WHERE id = \\?").
		WithArgs(order.Status, sqlmock.AnyArg(), order.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateWithTx(ctx, tx, order))

	mock.ExpectExec("DELETE FROM orders WHERE id = \\?").
		WithArgs(uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(ctx, 1))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderRepository_ClaimUnpaidWithTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)

	claimSQL := "UPDATE orders SET status = \\?, updated_at = \\? WHERE id = \\? AND status <> \\?"
	mock.ExpectExec(claimSQL).
		WithArgs(int32(0), sqlmock.AnyArg(), uint64(7), int32(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	claimed, err := repo.ClaimUnpaidWithTx(ctx, tx, 7, 0)
	require.NoError(t, err)
	assert.True(t, claimed)

	mock.ExpectExec(claimSQL).
		WithArgs(int32(0), sqlmock.AnyArg(), uint64(7), int32(0)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	claimed, err = repo.ClaimUnpaidWithTx(ctx, tx, 7, 0)
	require.NoError(t, err)
	assert.False(t, claimed)

	mock.ExpectExec(claimSQL).
		WillReturnError(errors.New("claim fail"))
	_, err = repo.ClaimUnpaidWithTx(ctx, tx, 7, 0)
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVariableRepository_GetRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewVariableRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"price"}).AddRow(int64(1500))
	mock.ExpectQuery("SELECT price FROM variables WHERE asset = \\?").
		WithArgs("psc").
		WillReturnRows(rows)

	rate, err := repo.GetRate(ctx, "psc")
	require.NoError(t, err)
	assert.Equal(t, float64(1500), rate)

	mock.ExpectQuery("SELECT price FROM variables WHERE asset = \\?").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	_, err = repo.GetRate(ctx, "missing")
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()
	payment := &models.Payment{UserID: 1, RefID: 12345, CardPan: "1234", Gateway: "sadad", Amount: 100, Product: "psc"}

	mock.ExpectExec("INSERT INTO payments").
		WillReturnResult(sqlmock.NewResult(7, 1))
	require.NoError(t, repo.Create(ctx, payment))
	assert.Equal(t, uint64(7), payment.ID)

	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	mock.ExpectExec("INSERT INTO payments").
		WillReturnResult(sqlmock.NewResult(8, 1))
	require.NoError(t, repo.CreateWithTx(ctx, tx, payment))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepository_CRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewTransactionRepository(db)
	ctx := context.Background()
	token := int64(1)
	refID := int64(2)
	payableType := "order"
	payableID := uint64(5)
	txModel := &models.Transaction{
		ID: "tx-1", UserID: 1, Asset: "psc", Amount: 10, Action: "deposit",
		Status: 1, Token: &token, RefID: &refID, PayableType: &payableType, PayableID: &payableID,
	}

	mock.ExpectExec("INSERT INTO transactions").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(ctx, txModel))

	mock.ExpectExec("UPDATE transactions SET status = \\?, ref_id = \\?, token = \\?, updated_at = \\? WHERE id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Update(ctx, txModel))

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "asset", "amount", "action", "status", "token", "ref_id", "payable_type", "payable_id", "created_at", "updated_at",
	}).AddRow("tx-1", 1, "psc", 10.0, "deposit", 1, token, refID, payableType, payableID, now, now)
	mock.ExpectQuery("SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at FROM transactions WHERE id = \\?").
		WithArgs("tx-1").
		WillReturnRows(rows)

	found, err := repo.FindByID(ctx, "tx-1")
	require.NoError(t, err)
	require.NotNil(t, found)

	payableRows := sqlmock.NewRows([]string{
		"id", "user_id", "asset", "amount", "action", "status", "token", "ref_id", "payable_type", "payable_id", "created_at", "updated_at",
	}).AddRow("tx-1", 1, "psc", 10.0, "deposit", 1, token, refID, payableType, payableID, now, now)
	mock.ExpectQuery("SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at FROM transactions WHERE payable_type = \\? AND payable_id = \\?").
		WithArgs("order", uint64(5)).
		WillReturnRows(payableRows)
	found, err = repo.FindByPayable(ctx, "order", 5)
	require.NoError(t, err)
	require.NotNil(t, found)

	mock.ExpectExec("DELETE FROM transactions WHERE id = \\?").
		WithArgs("tx-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(ctx, "tx-1"))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOptionRepository_FindByCodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewOptionRepository(db)
	ctx := context.Background()

	empty, err := repo.FindByCodes(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	note := "note"
	rows := sqlmock.NewRows([]string{"id", "code", "asset", "amount", "note", "created_at", "updated_at"}).
		AddRow(1, "aa", "psc", 10.0, note, time.Now(), time.Now())
	mock.ExpectQuery("SELECT id, code, asset, amount, note, created_at, updated_at FROM options WHERE code IN").
		WithArgs("aa", "bb").
		WillReturnRows(rows)

	options, err := repo.FindByCodes(ctx, []string{"aa", "bb"})
	require.NoError(t, err)
	require.Len(t, options, 1)
	require.NotNil(t, options[0].Note)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageRepository_FindImageURLByImageable(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewImageRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"url"}).AddRow("https://cdn/img.png")
	mock.ExpectQuery("SELECT url FROM images WHERE imageable_type = \\? AND imageable_id = \\?").
		WithArgs("option", uint64(1)).
		WillReturnRows(rows)

	url, err := repo.FindImageURLByImageable(ctx, "option", 1)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn/img.png", url)

	mock.ExpectQuery("SELECT url FROM images WHERE imageable_type = \\? AND imageable_id = \\?").
		WithArgs("option", uint64(2)).
		WillReturnError(sql.ErrNoRows)
	url, err = repo.FindImageURLByImageable(ctx, "option", 2)
	require.NoError(t, err)
	assert.Empty(t, url)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFirstOrderRepository_CreateAndCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewFirstOrderRepository(db)
	ctx := context.Background()
	fo := &models.FirstOrder{UserID: 1, Type: "psc", Amount: 10, Date: "1403/01/01", Bonus: 1}

	mock.ExpectExec("INSERT INTO first_orders").
		WillReturnResult(sqlmock.NewResult(3, 1))
	require.NoError(t, repo.Create(ctx, fo))
	assert.Equal(t, uint64(3), fo.ID)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM first_orders WHERE user_id = \\?").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.Count(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEligibilityRepository(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewEligibilityRepository(db)
	ctx := context.Background()
	birth := time.Date(1990, 5, 5, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"birthdate"}).AddRow(birth)
	mock.ExpectQuery("SELECT birthdate FROM kycs WHERE user_id = \\?").
		WithArgs(uint64(1)).
		WillReturnRows(rows)

	got, err := repo.GetUserBirthdate(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	permRows := sqlmock.NewRows([]string{"verified", "BFR"}).AddRow(true, false)
	mock.ExpectQuery("SELECT verified, BFR FROM child_permissions WHERE user_id = \\?").
		WithArgs(uint64(2)).
		WillReturnRows(permRows)

	verified, bfr, found, err := repo.GetChildPermissions(ctx, 2)
	require.NoError(t, err)
	assert.True(t, verified)
	assert.False(t, bfr)
	assert.True(t, found)

	mock.ExpectQuery("SELECT verified, BFR FROM child_permissions WHERE user_id = \\?").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	verified, bfr, found, err = repo.GetChildPermissions(ctx, 3)
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, mock.ExpectationsWereMet())
}
