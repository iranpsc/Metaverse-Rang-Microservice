package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/features-service/internal/repository"
)

func TestCitizenFeatureCountsSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := repository.NewCitizenFeaturesRepository(db)
	ctx := context.Background()
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	mock.ExpectQuery("FROM features f").WithArgs(uint64(7), "m").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	count, err := r.CountOwnedByKarbari(ctx, 7, "m")
	if err != nil || count != 3 {
		t.Fatalf("owned=%d err=%v", count, err)
	}
	for _, role := range []string{"buyer", "seller"} {
		mock.ExpectQuery("FROM trades t").WithArgs(uint64(7), "m", start, end).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
		count, err = r.CountTradesByKarbari(ctx, 7, role, "m", start, end)
		if err != nil || count != 2 {
			t.Fatalf("%s trades=%d err=%v", role, count, err)
		}
		mock.ExpectQuery("FROM trades t").WithArgs(uint64(7), "m", start, end).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(8, start))
		items, err := r.ListTradeTimestamps(ctx, 7, role, []string{"m"}, start, end)
		if err != nil || len(items) != 1 || items[0].ID != 8 {
			t.Fatalf("%s timestamps=%+v err=%v", role, items, err)
		}
	}
	if _, err := r.CountTradesByKarbari(ctx, 7, "other", "m", start, end); err == nil {
		t.Fatal("invalid role accepted")
	}
	if _, err := r.ListTradeTimestamps(ctx, 7, "other", []string{"m"}, start, end); err == nil {
		t.Fatal("invalid role accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
