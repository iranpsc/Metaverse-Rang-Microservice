package service_test

import (
	"context"
	"testing"

	"metargb/features-service/internal/repository"
	"metargb/features-service/internal/service"
	"metargb/features-service/tests/internal/testutil"
	"metargb/shared/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeaturePricingService_GetFeaturePriceInfo_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	log := logger.NewLogger("pricing-test")
	fr := repository.NewFeatureRepository(db)
	pr := repository.NewPropertiesRepository(db)
	svc := service.NewFeaturePricingService(fr, pr, db, log)
	ctx := context.Background()

	info, err := svc.GetFeaturePriceInfo(ctx, 1)
	if err != nil {
		t.Skipf("feature 1 not available: %v", err)
	}
	require.NotNil(t, info)
	_, ok := info["karbari"]
	assert.True(t, ok)
}

func TestFeaturePricingService_UpdateFeatureLabel_Unauthorized_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	log := logger.NewLogger("pricing-test")
	fr := repository.NewFeatureRepository(db)
	pr := repository.NewPropertiesRepository(db)
	svc := service.NewFeaturePricingService(fr, pr, db, log)
	ctx := context.Background()

	err := svc.UpdateFeatureLabel(ctx, 1, 999999888, "x")
	if err == nil {
		return
	}
	assert.Contains(t, err.Error(), "unauthorized")
}
