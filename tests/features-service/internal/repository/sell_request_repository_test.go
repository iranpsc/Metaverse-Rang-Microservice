package repository_test

import (
	"context"
	"testing"

	"metargb/features-service/internal/repository"
	"metargb/features-service/tests/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSellRequestRepository_CreateFindListDelete(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewSellRequestRepository(db)
	ctx := context.Background()

	sellerID := uint64(900001)
	featureID := uint64(900002)

	id, err := repo.Create(ctx, sellerID, featureID, 10.5, 1000000, 100)
	require.NoError(t, err)
	assert.Greater(t, id, uint64(0))

	found, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sellerID, found.SellerID)
	assert.Equal(t, featureID, found.FeatureID)

	list, err := repo.ListBySellerID(ctx, sellerID)
	require.NoError(t, err)
	foundInList := false
	for _, r := range list {
		if r.ID == id {
			foundInList = true
			break
		}
	}
	assert.True(t, foundInList, "created sell request should appear in list")

	require.NoError(t, repo.Delete(ctx, id))

	after, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, after)
}

func TestSellRequestRepository_FindByID_NotFound(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()
	repo := repository.NewSellRequestRepository(db)
	ctx := context.Background()
	row, err := repo.FindByID(ctx, 999999999)
	require.NoError(t, err)
	assert.Nil(t, row)
}
