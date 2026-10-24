package repository_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/features-service/internal/repository"
)

func TestCitizenFeatureListingSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := repository.NewCitizenFeaturesRepository(db)
	ctx := context.Background()
	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(7), "m", "%Main%", "%Main%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT f.id").WithArgs(uint64(7), "m", "%Main%", "%Main%", 15, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "vod_id", "address", "area", "density", "karbari", "owner_code", "price_psc", "price_irr", "label"}).
			AddRow(10, "VOD", "Main Street", 2, 3, "m", "U7", "5", "10", "Home"))
	mock.ExpectQuery("FROM coordinates c").WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "cx", "cy"}).AddRow(10, 1.5, 2.5))
	mock.ExpectQuery("FROM images").WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "imageable_id", "url"}).AddRow(3, 10, "/house.png"))
	items, total, err := r.ListOwnedFeatures(ctx, 7, []string{"m"}, "Main", 0, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].Center == nil || len(items[0].Images) != 1 {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	mock.ExpectQuery("SELECT f.id").WithArgs(uint64(7), "m").
		WillReturnRows(sqlmock.NewRows([]string{"id", "karbari"}).AddRow(10, "m"))
	mock.ExpectQuery("FROM coordinates c").WithArgs(uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "cx", "cy"}).AddRow(10, 1.5, 2.5))
	markers, err := r.ListMapMarkers(ctx, 7, []string{"m"})
	if err != nil || len(markers) != 1 || markers[0].Center == nil {
		t.Fatalf("markers=%+v err=%v", markers, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
