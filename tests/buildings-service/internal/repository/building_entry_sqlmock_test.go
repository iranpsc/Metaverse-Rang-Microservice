package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/tests/internal/testutil"
)

func TestBuildingEntryConfigRepository(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	config := models.BuildingEntryConfig{FeatureID: 12, FeePSC: "2.50", FeeIRR: "10", About: "museum", IsActive: true}
	mock.ExpectExec("INSERT INTO building_entry_configs").WithArgs(uint64(12), "2.50", "10", "museum", nil, nil, 1).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := r.UpsertConfig(ctx, config); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO building_entry_configs").WillReturnError(errors.New("write failed"))
	if err := r.UpsertConfig(ctx, config); err == nil {
		t.Fatal("expected write failure")
	}
	columns := []string{"feature_id", "fee_psc", "fee_irr", "about", "level_scope_type", "level_slug", "is_active"}
	mock.ExpectQuery("FROM building_entry_configs").WithArgs(uint64(12)).WillReturnRows(sqlmock.NewRows(columns).AddRow(12, "2.50", "10", "museum", "level", "one", 1))
	got, err := r.FindConfig(ctx, 12)
	if err != nil || got == nil || !got.IsActive || got.LevelSlug != "one" {
		t.Fatalf("config=%+v err=%v", got, err)
	}
	mock.ExpectQuery("FROM building_entry_configs").WithArgs(uint64(12)).WillReturnRows(sqlmock.NewRows(columns))
	got, err = r.FindConfig(ctx, 12)
	if err != nil || got != nil {
		t.Fatalf("missing config=%+v err=%v", got, err)
	}
	mock.ExpectQuery("FROM building_entry_configs").WillReturnError(errors.New("read failed"))
	if _, err := r.FindConfig(ctx, 12); err == nil {
		t.Fatal("expected read failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildingEntryCouponRepository(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	columns := []string{"id", "feature_id", "code", "discount_percentage", "max_usage_count", "real_usage_count"}
	row := func() *sqlmock.Rows { return sqlmock.NewRows(columns).AddRow(7, 12, "SAVE", 20, 5, 1) }
	mock.ExpectExec("INSERT INTO building_entry_coupons").WithArgs(uint64(12), "SAVE", int32(20), int32(5)).WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery("FROM building_entry_coupons c").WithArgs(uint64(12), uint64(7)).WillReturnRows(row())
	got, err := r.CreateCoupon(ctx, models.BuildingEntryCoupon{FeatureID: 12, Code: "SAVE", DiscountPercentage: 20, MaxUsageCount: 5})
	if err != nil || got == nil || got.RealUsageCount != 1 {
		t.Fatalf("coupon=%+v err=%v", got, err)
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WithArgs(uint64(12)).WillReturnRows(row())
	list, err := r.ListCoupons(ctx, 12)
	if err != nil || len(list) != 1 || list[0].Code != "SAVE" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WithArgs(uint64(12), "SAVE").WillReturnRows(row())
	got, err = r.FindCouponByCode(ctx, 12, "SAVE")
	if err != nil || got == nil || got.ID != 7 {
		t.Fatalf("coupon=%+v err=%v", got, err)
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WithArgs(uint64(12), "MISS").WillReturnRows(sqlmock.NewRows(columns))
	got, err = r.FindCouponByCode(ctx, 12, "MISS")
	if err != nil || got != nil {
		t.Fatalf("missing coupon=%+v err=%v", got, err)
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WithArgs(uint64(12), uint64(99)).WillReturnRows(sqlmock.NewRows(columns))
	got, err = r.FindCouponByID(ctx, 12, 99)
	if err != nil || got != nil {
		t.Fatalf("missing coupon=%+v err=%v", got, err)
	}
	mock.ExpectExec("INSERT INTO building_entry_coupons").WillReturnError(&mysql.MySQLError{Number: 1062})
	_, err = r.CreateCoupon(ctx, models.BuildingEntryCoupon{FeatureID: 12})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildingEntryCurrentAccess(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	after := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	columns := []string{"id", "exited_at"}
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).WillReturnRows(sqlmock.NewRows(columns).AddRow(4, nil))
	access, err := r.FindCurrentAccess(ctx, 12, 8, after)
	if err != nil || access == nil || !access.Inside {
		t.Fatalf("access=%+v err=%v", access, err)
	}
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).WillReturnRows(sqlmock.NewRows(columns).AddRow(4, after))
	access, err = r.FindCurrentAccess(ctx, 12, 8, after)
	if err != nil || access == nil || access.Inside {
		t.Fatalf("exited access=%+v err=%v", access, err)
	}
	mock.ExpectQuery("FROM building_entry_sessions").WithArgs(uint64(12), uint64(8), after).WillReturnRows(sqlmock.NewRows(columns))
	access, err = r.FindCurrentAccess(ctx, 12, 8, after)
	if err != nil || access != nil {
		t.Fatalf("missing access=%+v err=%v", access, err)
	}
	mock.ExpectQuery("FROM building_entry_sessions").WillReturnError(errors.New("read failed"))
	if _, err := r.FindCurrentAccess(ctx, 12, 8, after); err == nil {
		t.Fatal("expected query error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildingEntryCouponRepositoryFailures(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	r := repository.NewBuildingEntryRepository(db)
	ctx := context.Background()
	columns := []string{"id", "feature_id", "code", "discount_percentage", "max_usage_count", "real_usage_count"}
	mock.ExpectQuery("FROM building_entry_coupons c").WillReturnError(errors.New("query failed"))
	if _, err := r.ListCoupons(ctx, 12); err == nil {
		t.Fatal("list query error ignored")
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WillReturnRows(sqlmock.NewRows(columns).AddRow("bad", 12, "SAVE", 10, 5, 0))
	if _, err := r.ListCoupons(ctx, 12); err == nil {
		t.Fatal("list scan error ignored")
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WillReturnError(errors.New("query failed"))
	if _, err := r.FindCouponByCode(ctx, 12, "SAVE"); err == nil {
		t.Fatal("code query error ignored")
	}
	mock.ExpectQuery("FROM building_entry_coupons c").WillReturnError(errors.New("query failed"))
	if _, err := r.FindCouponByID(ctx, 12, 7); err == nil {
		t.Fatal("id query error ignored")
	}
	mock.ExpectExec("INSERT INTO building_entry_coupons").WillReturnError(errors.New("insert failed"))
	if _, err := r.CreateCoupon(ctx, models.BuildingEntryCoupon{FeatureID: 12}); err == nil {
		t.Fatal("insert error ignored")
	}
	mock.ExpectExec("INSERT INTO building_entry_coupons").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery("FROM building_entry_coupons c").WillReturnRows(sqlmock.NewRows(columns))
	if _, err := r.CreateCoupon(ctx, models.BuildingEntryCoupon{FeatureID: 12}); err == nil {
		t.Fatal("missing created coupon ignored")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
