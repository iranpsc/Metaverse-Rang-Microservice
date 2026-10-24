package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/tests/internal/testutil"
)

func expectEntryLock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT GET_LOCK").WithArgs("buildings:entry:12:8").
		WillReturnRows(sqlmock.NewRows([]string{"lock"}).AddRow(1))
}

func expectEntryUnlock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT RELEASE_LOCK").WithArgs("buildings:entry:12:8").
		WillReturnRows(sqlmock.NewRows([]string{"released"}).AddRow(1))
}

func TestEntrySessionCreateAndReopenSQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	after := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnRows(sqlmock.NewRows([]string{"id", "exited_at"}))
	mock.ExpectExec("UPDATE building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO building_entry_sessions").WithArgs(uint64(12), uint64(8), "0.00", "0.00", nil).
		WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectCommit()
	expectEntryUnlock(mock)
	id, err := r.CreateSession(ctx, models.BuildingEntrySession{FeatureID: 12, UserID: 8}, after)
	if err != nil || id != 19 {
		t.Fatalf("created id=%d err=%v", id, err)
	}

	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(19), uint64(12), uint64(8), after).
		WillReturnRows(sqlmock.NewRows([]string{"exited_at"}).AddRow(after))
	mock.ExpectExec("UPDATE building_entry_sessions").WithArgs(uint64(19), uint64(12), uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectEntryUnlock(mock)
	if err := r.ReopenSession(ctx, 12, 8, 19, after); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEntrySessionCouponLimitAndLifecycleSQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	after := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	coupon := uint64(5)
	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnRows(sqlmock.NewRows([]string{"id", "exited_at"}))
	mock.ExpectExec("UPDATE building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("FROM building_entry_coupons").WithArgs(coupon, uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"max_usage_count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT").WithArgs(coupon).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()
	expectEntryUnlock(mock)
	_, err := r.CreateSession(ctx, models.BuildingEntrySession{FeatureID: 12, UserID: 8, CouponID: &coupon}, after)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("coupon error=%v", err)
	}

	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM building_entry_sessions").WithArgs(uint64(19), uint64(12), uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectEntryUnlock(mock)
	if err := r.DeleteSession(ctx, 12, 8, 19); err != nil {
		t.Fatal(err)
	}

	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE building_entry_sessions").WithArgs(uint64(12), uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectEntryUnlock(mock)
	if err := r.CloseActiveSession(ctx, 12, 8); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEntrySessionCouponSuccessAndMissingSessionSQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	after := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	coupon := uint64(5)
	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnRows(sqlmock.NewRows([]string{"id", "exited_at"}))
	mock.ExpectExec("UPDATE building_entry_sessions").WithArgs(uint64(12), uint64(8), after).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("FROM building_entry_coupons").WithArgs(coupon, uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"max_usage_count"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT").WithArgs(coupon).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO building_entry_sessions").WithArgs(uint64(12), uint64(8), "4.00", "8.00", coupon).
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectCommit()
	expectEntryUnlock(mock)
	id, err := r.CreateSession(ctx, models.BuildingEntrySession{FeatureID: 12, UserID: 8, CouponID: &coupon, FeePSC: "4.00", FeeIRR: "8.00"}, after)
	if err != nil || id != 20 {
		t.Fatalf("id=%d err=%v", id, err)
	}

	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(20), uint64(12), uint64(8), after).
		WillReturnRows(sqlmock.NewRows([]string{"exited_at"}))
	mock.ExpectRollback()
	expectEntryUnlock(mock)
	err = r.ReopenSession(ctx, 12, 8, 20, after)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("reopen error=%v", err)
	}

	expectEntryLock(mock)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM building_entry_sessions").WithArgs(uint64(20), uint64(12), uint64(8)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	expectEntryUnlock(mock)
	err = r.DeleteSession(ctx, 12, 8, 20)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("delete error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEntryUserLockRunsCallback(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	expectEntryLock(mock)
	expectEntryUnlock(mock)
	called := false
	err := r.WithUserLock(context.Background(), 12, 8, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEntryUserLockTimeout(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	mock.ExpectQuery("SELECT GET_LOCK").WithArgs("buildings:entry:12:8").
		WillReturnRows(sqlmock.NewRows([]string{"lock"}).AddRow(0))
	called := false
	err := r.WithUserLock(context.Background(), 12, 8, func(context.Context) error {
		called = true
		return nil
	})
	if err == nil || called {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
