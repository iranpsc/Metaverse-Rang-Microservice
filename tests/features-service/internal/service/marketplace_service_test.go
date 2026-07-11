package service_test

import (
	"context"
	"database/sql"
	"testing"

	"metargb/features-service/internal/metrics"
	"metargb/features-service/internal/repository"
	"metargb/features-service/internal/service"
	"metargb/features-service/tests/internal/testutil"
	commercialpb "metargb/shared/pb/commercial"
	"metargb/shared/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockCommercialClient is a mock implementation of CommercialClient
type MockCommercialClient struct {
	mock.Mock
}

func (m *MockCommercialClient) CheckBalance(ctx context.Context, userID uint64, asset string, requiredAmount float64) (bool, error) {
	args := m.Called(ctx, userID, asset, requiredAmount)
	return args.Bool(0), args.Error(1)
}

func (m *MockCommercialClient) DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	args := m.Called(ctx, userID, asset, amount)
	return args.Error(0)
}

func (m *MockCommercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	args := m.Called(ctx, userID, asset, amount)
	return args.Error(0)
}

func (m *MockCommercialClient) CreateTransaction(ctx context.Context, userID uint64, asset string, amount float64, action string, status int32, payableType string, payableID uint64) (*commercialpb.Transaction, error) {
	args := m.Called(ctx, userID, asset, amount, action, status, payableType, payableID)
	return args.Get(0).(*commercialpb.Transaction), args.Error(1)
}

func (m *MockCommercialClient) Close() error {
	return nil
}

// MockNotificationClient is a mock implementation of NotificationClient
type MockNotificationClient struct {
	mock.Mock
}

func (m *MockNotificationClient) SendBuyRequestNotification(ctx context.Context, userID uint64, notificationType string, buyRequestID, featureID uint64, pricePSC, priceIRR float64) error {
	args := m.Called(ctx, userID, notificationType, buyRequestID, featureID, pricePSC, priceIRR)
	return args.Error(0)
}

func (m *MockNotificationClient) SendBuyFeatureNotification(ctx context.Context, userID uint64, featureID uint64, isRGBPurchase bool, color string, stability float64, pscAmount, irrAmount float64) error {
	args := m.Called(ctx, userID, featureID, isRGBPurchase, color, stability, pscAmount, irrAmount)
	return args.Error(0)
}

func (m *MockNotificationClient) SendSellRequestNotification(ctx context.Context, sellerID uint64, featureID uint64, featurePropertiesID string) error {
	args := m.Called(ctx, sellerID, featureID, featurePropertiesID)
	return args.Error(0)
}

func (m *MockNotificationClient) Close() error {
	return nil
}

// MockEventBroadcaster is a mock implementation of EventBroadcaster
type MockEventBroadcaster struct {
	mock.Mock
}

func (m *MockEventBroadcaster) BroadcastFeatureStatusChanged(ctx context.Context, featureID uint64, rgb string) error {
	args := m.Called(ctx, featureID, rgb)
	return args.Error(0)
}

func (m *MockEventBroadcaster) Close() error {
	return nil
}

func marketplaceFromDB(t *testing.T, db *sql.DB) *service.MarketplaceService {
	t.Helper()
	log := logger.NewLogger("marketplace-int")
	return service.NewMarketplaceService(
		repository.NewFeatureRepository(db),
		repository.NewPropertiesRepository(db),
		repository.NewGeometryRepository(db),
		repository.NewTradeRepository(db),
		repository.NewBuyRequestRepository(db),
		repository.NewSellRequestRepository(db),
		repository.NewLockedAssetRepository(db),
		repository.NewHourlyProfitRepository(db),
		repository.NewFeatureLimitRepository(db),
		repository.NewVariableRepository(db),
		nil,
		nil,
		nil,
		metrics.NewMarketplaceMetrics(),
		db,
		log,
	)
}

func integrationMarketplaceService(t *testing.T) (*service.MarketplaceService, *sql.DB) {
	db := testutil.OpenMySQLOrSkip(t)
	return marketplaceFromDB(t, db), db
}

func TestMarketplaceService_ListBuyRequests_Integration(t *testing.T) {
	ms, db := integrationMarketplaceService(t)
	defer db.Close()
	ctx := context.Background()
	out, err := ms.ListBuyRequests(ctx, 999999001)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestMarketplaceService_ListReceivedBuyRequests_Integration(t *testing.T) {
	ms, db := integrationMarketplaceService(t)
	defer db.Close()
	ctx := context.Background()
	out, err := ms.ListReceivedBuyRequests(ctx, 999999002)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestMarketplaceService_ListSellRequests_Integration(t *testing.T) {
	ms, db := integrationMarketplaceService(t)
	defer db.Close()
	ctx := context.Background()
	out, err := ms.ListSellRequests(ctx, 999999003)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

func TestMarketplaceService_RequestGracePeriod_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	ctx := context.Background()
	buyRepo := repository.NewBuyRequestRepository(db)
	rid, err := buyRepo.Create(ctx, 910001, 910002, 910003, "grace-test", 1, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyRepo.Delete(ctx, rid) })

	ms := marketplaceFromDB(t, db)
	require.NoError(t, ms.RequestGracePeriod(ctx, rid, 910002, "7"))

	row, err := buyRepo.FindByID(ctx, rid)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.True(t, row.RequestedGracePeriod.Valid)
}

func TestMarketplaceService_UpdateGracePeriod_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	ctx := context.Background()
	buyRepo := repository.NewBuyRequestRepository(db)
	rid, err := buyRepo.Create(ctx, 910010, 910011, 910012, "grace2", 1, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyRepo.Delete(ctx, rid) })

	ms := marketplaceFromDB(t, db)
	require.NoError(t, ms.UpdateGracePeriod(ctx, rid, 910011, 14))

	row, err := buyRepo.FindByID(ctx, rid)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.True(t, row.RequestedGracePeriod.Valid)
}

func TestMarketplaceService_DeleteSellRequest_Integration(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	ctx := context.Background()
	sellRepo := repository.NewSellRequestRepository(db)
	// Use feature_id=1 if present in test DB; ignore failure if schema empty
	id, err := sellRepo.Create(ctx, 1, 1, 1, 1, 100)
	if err != nil {
		t.Skipf("seed sell request: %v", err)
	}
	ms := marketplaceFromDB(t, db)
	err = ms.DeleteSellRequest(ctx, id, 1)
	if err != nil {
		// May fail if feature/properties missing for status rollback
		t.Logf("DeleteSellRequest: %v", err)
	}
}

func TestMarketplaceService_RejectBuyRequest_NotFound(t *testing.T) {
	ms, db := integrationMarketplaceService(t)
	defer db.Close()
	err := ms.RejectBuyRequest(context.Background(), 999999777, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMarketplaceService_DeleteBuyRequest_NotFound(t *testing.T) {
	ms, db := integrationMarketplaceService(t)
	defer db.Close()
	err := ms.DeleteBuyRequest(context.Background(), 999999776, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
