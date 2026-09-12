package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
	"metarang/features-service/tests/internal/testutil"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/logger"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mapCols() []string {
	return []string{
		"id", "name", "karbari", "publish_date", "publisher_name", "polygon_count",
		"total_area", "first_id", "last_id", "status", "fileName",
		"central_point_coordinates", "border_coordinates", "polygon_area",
		"polygon_address", "polygon_color",
	}
}

func TestMapService_ListGetBorder_SQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	svc := service.NewMapService(repository.NewMapRepository(db), repository.NewFeatureRepository(db))
	now := time.Now()

	mock.ExpectQuery("FROM maps").
		WillReturnRows(sqlmock.NewRows(mapCols()).AddRow(
			uint64(1), "City", "m", now, "pub", int64(10),
			int64(100), "a", "z", 1, "f",
			"[1,2]", "[[0,0]]", uint64(50),
			"addr", "red",
		))
	mock.ExpectQuery("WHERE f.map_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "karbari"}).
			AddRow(uint64(1), uint64(1), "m").
			AddRow(uint64(2), uint64(9), "t").
			AddRow(uint64(3), uint64(8), "a"))

	list, err := svc.ListMaps(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "66.67", list[0].SoldFeaturesPercentage)
	assert.Equal(t, "red", list[0].Color)

	mock.ExpectQuery("FROM maps").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(mapCols()).AddRow(
			uint64(1), "City", "m", now, "pub", int64(10),
			int64(100), "a", "z", 1, "f",
			"[1,2]", "[[0,0]]", uint64(50),
			"addr", "red",
		))
	mock.ExpectQuery("WHERE f.map_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "karbari"}).
			AddRow(uint64(2), uint64(9), "m"))
	detail, err := svc.GetMap(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "100.00", detail.SoldFeaturesPercentage)
	require.NotNil(t, detail.Features)
	assert.Equal(t, int32(1), detail.Features.Maskoni.Sold)

	mock.ExpectQuery("FROM maps").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(mapCols()).AddRow(
			uint64(1), "City", "m", now, "pub", int64(10),
			int64(100), "a", "z", 1, "f",
			"[1,2]", "[[0,0]]", uint64(50),
			"addr", "red",
		))
	border, err := svc.GetMapBorder(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "[[0,0]]", border)

	mock.ExpectQuery("FROM maps").
		WithArgs(uint64(99)).
		WillReturnError(sql.ErrNoRows)
	_, err = svc.GetMap(context.Background(), 99)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeaturePricingService_SQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	log := logger.NewLogger("pricing-test")
	svc := service.NewFeaturePricingService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		db, log,
	)

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	info, err := svc.GetFeaturePriceInfo(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "m", info["karbari"])

	expectFeatureFindByID(mock, 99, "m", 1000, 80)
	err = svc.UpdateFeaturePricing(context.Background(), 1, 3, 90)
	require.Error(t, err)

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT birthdate FROM kycs").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	err = svc.UpdateFeaturePricing(context.Background(), 1, 3, 70)
	require.Error(t, err)

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectQuery("SELECT birthdate FROM kycs").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("yellow").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectQuery("SELECT value FROM variables").
		WithArgs("psc").
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1.0))
	mock.ExpectExec("SET price_psc").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.UpdateFeaturePricing(context.Background(), 1, 3, 90))

	expectFeatureFindByID(mock, 3, "m", 1000, 80)
	mock.ExpectExec("UPDATE feature_properties SET").
		WithArgs("home", uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.UpdateFeatureLabel(context.Background(), 1, 3, "home"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureService_GetAndList_SQLMock(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	svc := service.NewFeatureService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewGeometryRepository(db),
		repository.NewImageRepository(db),
		repository.NewBuildingRepository(db),
		repository.NewTradeRepository(db),
		repository.NewHourlyProfitRepository(db),
		nil, db, nil, "https://app.test",
	)
	now := time.Now()

	_, err := svc.ListFeatures(context.Background(), []string{"1,1"}, false, false, 0)
	require.Error(t, err)

	mock.ExpectQuery("FROM coordinates c").
		WillReturnRows(sqlmock.NewRows([]string{"geometry_id"}))
	empty, err := svc.ListFeatures(context.Background(), []string{"0,1", "1,1", "0,0", "1,0"}, false, false, 2)
	require.NoError(t, err)
	assert.Empty(t, empty)

	mock.ExpectQuery("FROM coordinates c").
		WillReturnRows(sqlmock.NewRows([]string{"geometry_id"}).AddRow(uint64(1)))
	mock.ExpectQuery("WHERE f.id IN").
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	mock.ExpectQuery("FROM geometries g").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "type", "created_at", "updated_at"}).
			AddRow(uint64(3), uint64(1), "Polygon", now, now))
	mock.ExpectQuery("INNER JOIN geometries g ON g.id = c.geometry_id").
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "id", "geometry_id", "x", "y"}).
			AddRow(uint64(1), uint64(4), uint64(3), "1.5", "2.5"))
	mock.ExpectQuery("FROM buildings").
		WillReturnRows(sqlmock.NewRows([]string{
			"feature_id", "id", "construction_start_date", "construction_end_date", "launched_satisfaction",
			"rotation", "position", "bubble_diameter", "information",
			"model_id", "model_model_id", "model_name", "model_sku", "model_images",
			"model_attributes", "model_file", "model_required_satisfaction",
		}))
	listed, err := svc.ListFeatures(context.Background(), []string{"0,1", "1,1", "0,0", "1,0"}, true, false, 2)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.True(t, listed[0].IsOwnedByAuthUser)
	require.NotNil(t, listed[0].Geometry)

	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	err = svc.UpdateMyFeature(context.Background(), 2, 1, 90)
	require.Error(t, err)

	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnError(sql.ErrNoRows)
	err = svc.RemoveMyFeatureImage(context.Background(), 2, 1, 4)
	require.Error(t, err)

	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	_, err = svc.AddMyFeatureImages(context.Background(), 2, 1, [][]byte{{1}}, []string{"a.jpg"}, []string{"image/jpeg"})
	require.Error(t, err)

	mock.ExpectQuery("FROM coordinates c").
		WillReturnRows(sqlmock.NewRows([]string{"geometry_id"}).AddRow(uint64(1)))
	mock.ExpectQuery("WHERE f.id IN").
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	mock.ExpectQuery("FROM geometries g").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "type", "created_at", "updated_at"}).
			AddRow(uint64(3), uint64(1), "Polygon", now, now))
	mock.ExpectQuery("INNER JOIN geometries g").
		WillReturnRows(sqlmock.NewRows([]string{"feature_id", "id", "geometry_id", "x", "y"}).
			AddRow(uint64(1), uint64(4), uint64(3), "1.5", "2.5"))
	mock.ExpectQuery("FROM buildings").
		WillReturnRows(sqlmock.NewRows([]string{
			"feature_id", "id", "construction_start_date", "construction_end_date", "launched_satisfaction",
			"rotation", "position", "bubble_diameter", "information",
			"model_id", "model_model_id", "model_name", "model_sku", "model_images",
			"model_attributes", "model_file", "model_required_satisfaction",
		}).AddRow(
			uint64(1), uint64(8), "s", "e", "1",
			"0", "1,2", "3", "{}",
			uint64(7), uint64(101), "n", "sku", "[]",
			"[]", "{}", 1.5,
		))
	boxed, err := svc.ListFeatures(context.Background(), []string{"0,1", "1,1", "0,0", "1,0"}, true, false, 2)
	require.NoError(t, err)
	require.Len(t, boxed, 1)
	assert.True(t, boxed[0].IsOwnedByAuthUser)

	expectFeatureFindByID(mock, 2, "m", 10, 80)
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "created_at", "updated_at"}).
			AddRow(uint64(3), "Polygon", now, now))
	mock.ExpectQuery("SELECT c.x, c.y").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"x", "y"}).AddRow(1.5, 2.5))
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(uint64(4), "https://img"))
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "feature_id", "buyer_id", "seller_id", "irr_amount", "psc_amount", "date", "created_at", "updated_at",
			"seller_user_id", "seller_name", "seller_code",
		}).AddRow(8, 1, 2, 3, 1.0, 2.0, now, now, now, uint64(3), "Ali", "hm-3"))
	mock.ExpectQuery("FROM feature_hourly_profits").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "feature_id", "asset", "amount", "dead_line", "is_active", "created_at", "updated_at",
		}).AddRow(11, 2, 1, "yellow", 1.5, now, true, now, now))
	mock.ExpectQuery("FROM buildings").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "construction_start_date", "construction_end_date", "launched_satisfaction",
			"rotation", "position", "bubble_diameter", "information",
			"model_id", "model_model_id", "model_name", "model_sku", "model_images",
			"model_attributes", "model_file", "model_required_satisfaction",
		}))

	feat, err := svc.GetFeature(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), feat.Id)
	assert.True(t, feat.IsHourlyProfitActive)
	require.NotNil(t, feat.Seller)
	assert.Equal(t, "Ali", feat.Seller.Name)

	mock.ExpectQuery("LIMIT").
		WithArgs(uint64(2), 5, 0).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	list, err := svc.ListMyFeatures(context.Background(), 2, 0, "", "")
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mine, err := svc.GetMyFeature(context.Background(), 2, 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), mine.Id)

	mock.ExpectQuery("FROM features").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id", "map_id", "type", "created_at", "updated_at"}).
			AddRow(1, 2, 1, "polygon", now, now))
	owned, err := svc.GetMyFeatures(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, owned, 1)

	mock.ExpectExec("UPDATE feature_properties SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectFeatureFindByID(mock, 2, "m", 10, 80)
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM feature_hourly_profits").
		WithArgs(uint64(1), uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM buildings").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	updated, err := svc.UpdateFeature(context.Background(), 1, &pb.FeatureProperties{
		Karbari: "m", Rgb: "d", Owner: "o", Label: "l", PricePsc: "1", PriceIrr: "2", MinimumPricePercentage: 90,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), updated.Id)

	expectFeatureFindByID(mock, 2, "m", 10, 80)
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM feature_hourly_profits").
		WithArgs(uint64(1), uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM buildings").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrConnDone)
	added, err := svc.AddFeatureImages(context.Background(), 1, []string{"https://x"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), added.Id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceService_DeleteBuyRequest_SQLMock(t *testing.T) {
	stub := testutil.NewCommercialStub()
	svc, mock := newMarketplaceWithWallet(t, stub)
	now := time.Now()

	mock.ExpectQuery("FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnRows(buyRequestFindRows(3, 2, 0))
	mock.ExpectQuery("FROM locked_assets").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "buy_feature_request_id", "feature_id", "psc", "irr", "created_at", "updated_at",
		}).AddRow(7, 9, 1, 10.0, 20.0, now, now))
	mock.ExpectExec("DELETE FROM transactions").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM locked_assets").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM buy_feature_requests").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, svc.DeleteBuyRequest(context.Background(), 9, 2))
	require.NoError(t, mock.ExpectationsWereMet())
}

type stubFileStorage struct{}

func (stubFileStorage) UploadChunk(context.Context, string, string, string, string, []byte) (string, error) {
	return "uploads/features/1/image_1.jpg", nil
}

func TestFeatureService_AddMyFeatureImages_Uploads(t *testing.T) {
	db, mock := testutil.NewSQLMock(t)
	svc := service.NewFeatureService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewGeometryRepository(db),
		repository.NewImageRepository(db),
		repository.NewBuildingRepository(db),
		repository.NewTradeRepository(db),
		repository.NewHourlyProfitRepository(db),
		nil, db, stubFileStorage{}, "https://app.test",
	)
	now := time.Now()
	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	mock.ExpectExec("INSERT INTO images").
		WillReturnResult(sqlmock.NewResult(4, 1))
	mock.ExpectQuery("f.owner_id").
		WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows(featureFindCols()).AddRow(
			1, 2, 1, "polygon", now, now,
			"p1", 1, "m", "d", "o", "l", "addr",
			10.0, 1, 10.0, "0", "0", 80, now, now,
		))
	mock.ExpectQuery("FROM geometries g").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(uint64(4), "https://app.test/uploads/features/1/image_1.jpg"))
	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	out, err := svc.AddMyFeatureImages(context.Background(), 2, 1, [][]byte{{1, 2, 3}}, nil, []string{"image/png"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), out.Id)
	require.NoError(t, mock.ExpectationsWereMet())
}
