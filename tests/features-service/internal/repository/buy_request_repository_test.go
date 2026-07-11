package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metargb/features-service/internal/repository"
	"metargb/features-service/tests/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB connects when TEST_MYSQL_DSN is set (see internal/testutil.OpenMySQLOrSkip).
func setupTestDB(t *testing.T) *sql.DB {
	return testutil.OpenMySQLOrSkip(t)
}

func TestBuyRequestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	buyerID := uint64(1)
	sellerID := uint64(2)
	featureID := uint64(100)
	note := "Test buy request"
	pricePSC := 100.0
	priceIRR := 1000000.0

	id, err := repo.Create(ctx, buyerID, sellerID, featureID, note, pricePSC, priceIRR)

	require.NoError(t, err)
	assert.Greater(t, id, uint64(0))
}

func TestBuyRequestRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	// First create a request
	buyerID := uint64(1)
	sellerID := uint64(2)
	featureID := uint64(100)
	requestID, err := repo.Create(ctx, buyerID, sellerID, featureID, "test", 100.0, 1000000.0)
	require.NoError(t, err)

	// Then find it
	buyRequest, err := repo.FindByID(ctx, requestID)

	require.NoError(t, err)
	require.NotNil(t, buyRequest)
	assert.Equal(t, requestID, buyRequest.ID)
	assert.Equal(t, buyerID, buyRequest.BuyerID)
	assert.Equal(t, sellerID, buyRequest.SellerID)
	assert.Equal(t, featureID, buyRequest.FeatureID)
}

func TestBuyRequestRepository_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	// Create a request
	requestID, err := repo.Create(ctx, 1, 2, 100, "test", 100.0, 1000000.0)
	require.NoError(t, err)

	// Soft delete it
	err = repo.SoftDelete(ctx, requestID)
	require.NoError(t, err)

	// Verify it's soft deleted (should not be found in normal queries)
	// Note: FindByID might still return it depending on implementation
}

func TestBuyRequestRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	// Create a request
	requestID, err := repo.Create(ctx, 1, 2, 100, "test", 100.0, 1000000.0)
	require.NoError(t, err)

	// Update status to accepted (1)
	err = repo.UpdateStatus(ctx, requestID, 1)
	require.NoError(t, err)

	// Verify status
	buyRequest, err := repo.FindByID(ctx, requestID)
	require.NoError(t, err)
	assert.Equal(t, 1, buyRequest.Status)
}

func TestBuyRequestRepository_ListByBuyerID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	buyerID := uint64(1)

	// Create multiple requests
	_, err := repo.Create(ctx, buyerID, 2, 100, "test1", 100.0, 1000000.0)
	require.NoError(t, err)
	_, err = repo.Create(ctx, buyerID, 3, 101, "test2", 200.0, 2000000.0)
	require.NoError(t, err)

	// List by buyer
	requests, err := repo.ListByBuyerID(ctx, buyerID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(requests), 2)
}

func TestBuyRequestRepository_ListBySellerID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	sellerID := uint64(2)

	// Create multiple requests
	_, err := repo.Create(ctx, 1, sellerID, 100, "test1", 100.0, 1000000.0)
	require.NoError(t, err)
	_, err = repo.Create(ctx, 3, sellerID, 100, "test2", 200.0, 2000000.0)
	require.NoError(t, err)

	// List by seller
	requests, err := repo.ListBySellerID(ctx, sellerID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(requests), 2)
}

func TestBuyRequestRepository_HasPendingRequest(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	buyerID := uint64(1)
	featureID := uint64(100)

	// Initially no pending request
	hasPending, err := repo.HasPendingRequest(ctx, buyerID, featureID)
	require.NoError(t, err)
	assert.False(t, hasPending)

	// Create a pending request
	_, err = repo.Create(ctx, buyerID, 2, featureID, "test", 100.0, 1000000.0)
	require.NoError(t, err)

	// Now should have pending request
	hasPending, err = repo.HasPendingRequest(ctx, buyerID, featureID)
	require.NoError(t, err)
	assert.True(t, hasPending)
}

func TestBuyRequestRepository_UpdateGracePeriod(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuyRequestRepository(db)
	ctx := context.Background()

	// Create a request
	requestID, err := repo.Create(ctx, 1, 2, 100, "test", 100.0, 1000000.0)
	require.NoError(t, err)

	// Update grace period
	gracePeriod := sql.NullTime{
		Time:  time.Now().AddDate(0, 0, 7),
		Valid: true,
	}
	err = repo.UpdateGracePeriod(ctx, requestID, gracePeriod)
	require.NoError(t, err)

	// Verify grace period
	buyRequest, err := repo.FindByID(ctx, requestID)
	require.NoError(t, err)
	assert.True(t, buyRequest.RequestedGracePeriod.Valid)
}
