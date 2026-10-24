package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/features-service/internal/repository"
)

func TestIsicCodeRepositorySQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := repository.NewIsicCodeRepository(db)
	ctx := context.Background()
	columns := []string{"id", "name", "code", "verified"}
	mock.ExpectQuery("FROM isic_codes").WithArgs("%metal%", "%metal%", 2, 3).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(7, "Metal", 123, true).AddRow(8, "Other", nil, false))
	items, err := r.FindPaginated(ctx, " metal ", 2, 3)
	if err != nil || len(items) != 2 || items[0].Code == nil || *items[0].Code != 123 || items[1].Code != nil {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	mock.ExpectQuery("FROM isic_codes").WithArgs(10, 0).WillReturnRows(sqlmock.NewRows(columns).AddRow("bad", "Name", 1, true))
	if _, err := r.FindPaginated(ctx, "", 10, 0); err == nil {
		t.Fatal("expected scan error")
	}
	mock.ExpectQuery("FROM isic_codes").WithArgs(10, 0).WillReturnError(errors.New("query failed"))
	if _, err := r.FindPaginated(ctx, "", 10, 0); err == nil {
		t.Fatal("expected query error")
	}
	mock.ExpectQuery("SELECT COUNT").WithArgs("%metal%", "%metal%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := r.Count(ctx, "metal")
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	mock.ExpectQuery("SELECT COUNT").WillReturnError(errors.New("count failed"))
	if _, err := r.Count(ctx, ""); err == nil {
		t.Fatal("expected count error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
