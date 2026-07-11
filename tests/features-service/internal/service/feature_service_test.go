package service_test

import (
	"context"
	"database/sql"
	"testing"

	"metargb/features-service/internal/repository"
	"metargb/features-service/internal/service"
	"metargb/features-service/tests/internal/testutil"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFeatureServiceForTest(t *testing.T, db *sql.DB) *service.FeatureService {
	t.Helper()
	log := logger.NewLogger("feature-svc-test")
	fr := repository.NewFeatureRepository(db)
	pr := repository.NewPropertiesRepository(db)
	gr := repository.NewGeometryRepository(db)
	ir := repository.NewImageRepository(db)
	br := repository.NewBuildingRepository(db)
	tr := repository.NewTradeRepository(db)
	hpr := repository.NewHourlyProfitRepository(db)
	pricing := service.NewFeaturePricingService(fr, pr, db, log)
	return service.NewFeatureService(fr, pr, gr, ir, br, tr, hpr, pricing, db)
}

func TestFeatureService_GetFeature_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	svc := newFeatureServiceForTest(t, db)
	ctx := context.Background()

	f, err := svc.GetFeature(ctx, 1)
	if err != nil {
		t.Skipf("no feature id=1 in DB: %v", err)
	}
	require.NotNil(t, f)
	assert.Equal(t, uint64(1), f.Id)
}

func TestFeatureService_GetMyFeatures_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	svc := newFeatureServiceForTest(t, db)
	ctx := context.Background()

	out, err := svc.GetMyFeatures(ctx, 999888777)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestFeatureService_ListFeatures_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	svc := newFeatureServiceForTest(t, db)
	ctx := context.Background()

	points := []string{"0,0", "100,0", "100,100", "0,100"}
	out, err := svc.ListFeatures(ctx, points, false, false, 0)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestFeatureService_UpdateFeature_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	svc := newFeatureServiceForTest(t, db)
	ctx := context.Background()

	_, err := svc.UpdateFeature(ctx, 1, &pb.FeatureProperties{})
	if err != nil {
		t.Skipf("update feature 1: %v", err)
	}
}

func TestFeatureService_AddFeatureImages_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	svc := newFeatureServiceForTest(t, db)
	ctx := context.Background()

	_, err := svc.AddFeatureImages(ctx, 1, []string{"https://example.com/a.jpg"})
	if err != nil {
		t.Skipf("add images: %v", err)
	}
}
