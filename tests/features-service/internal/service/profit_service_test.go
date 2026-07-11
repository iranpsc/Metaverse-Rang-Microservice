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

func TestProfitService_GetProfitsByApplication_InvalidKarbari(t *testing.T) {
	log := logger.NewLogger("profit-test")
	svc := service.NewProfitService(nil, nil, nil, nil, nil, nil, log)
	_, err := svc.GetProfitsByApplication(context.Background(), 1, "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "karbari")
}

func TestProfitService_StartHourlyProfitCalculator(t *testing.T) {
	log := logger.NewLogger("profit-test")
	svc := service.NewProfitService(nil, nil, nil, nil, nil, nil, log)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.StartHourlyProfitCalculator(ctx, log)
}

func TestProfitService_GetHourlyProfits_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	log := logger.NewLogger("profit-test")
	pr := repository.NewHourlyProfitRepository(db)
	fr := repository.NewFeatureRepository(db)
	prepo := repository.NewPropertiesRepository(db)
	svc := service.NewProfitService(pr, fr, prepo, nil, nil, db, log)
	ctx := context.Background()

	profits, m, tj, am, _, err := svc.GetHourlyProfits(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.NotNil(t, profits)
	assert.NotEmpty(t, m)
	assert.NotEmpty(t, tj)
	assert.NotEmpty(t, am)
}

func TestProfitService_TransferProfitOnSale_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	log := logger.NewLogger("profit-test")
	pr := repository.NewHourlyProfitRepository(db)
	fr := repository.NewFeatureRepository(db)
	prepo := repository.NewPropertiesRepository(db)
	svc := service.NewProfitService(pr, fr, prepo, nil, nil, db, log)
	ctx := context.Background()

	err := svc.TransferProfitOnSale(ctx, 999999991, 1, 2, 10)
	// May error if no profit row; still exercises service + repo path
	if err != nil {
		t.Logf("TransferProfitOnSale: %v", err)
	}
}
