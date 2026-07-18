package repository_test

import (
	"context"
	"testing"
	"time"

	"metarang/features-service/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCitizenFeaturesRepository_EmptyKarbaris(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewCitizenFeaturesRepository(db)
	ctx := context.Background()

	trades, err := repo.ListTradeTimestamps(ctx, 1, "buyer", nil, time.Now().Add(-24*time.Hour), time.Now())
	require.NoError(t, err)
	assert.Empty(t, trades)

	items, total, err := repo.ListOwnedFeatures(ctx, 1, nil, "", 1, 15)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, items)

	markers, err := repo.ListMapMarkers(ctx, 1, []string{})
	require.NoError(t, err)
	assert.Empty(t, markers)

	centers, err := repo.GetFeatureCenters(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, centers)
}

func TestCitizenFeaturesRepository_InvalidTradeRole(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewCitizenFeaturesRepository(db)
	ctx := context.Background()
	now := time.Now()

	_, err := repo.CountTradesByKarbari(ctx, 1, "owner", "t", now.Add(-time.Hour), now)
	require.Error(t, err)

	_, err = repo.ListTradeTimestamps(ctx, 1, "owner", []string{"t"}, now.Add(-time.Hour), now)
	require.Error(t, err)
}

func TestCitizenFeaturesRepository_CountAndListSmoke(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewCitizenFeaturesRepository(db)
	ctx := context.Background()
	now := time.Now()

	count, err := repo.CountOwnedByKarbari(ctx, 1, "t")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int32(0))

	bought, err := repo.CountTradesByKarbari(ctx, 1, "buyer", "t", now.Add(-30*24*time.Hour), now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, bought, int32(0))

	sold, err := repo.CountTradesByKarbari(ctx, 1, "seller", "t", now.Add(-30*24*time.Hour), now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, sold, int32(0))

	_, err = repo.ListTradeTimestamps(ctx, 1, "buyer", []string{"t", "m"}, now.Add(-7*24*time.Hour), now)
	require.NoError(t, err)

	items, total, err := repo.ListOwnedFeatures(ctx, 1, []string{"t"}, "", 1, 15)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 0)
	assert.LessOrEqual(t, len(items), 15)

	markers, err := repo.ListMapMarkers(ctx, 1, []string{"t"})
	require.NoError(t, err)
	assert.NotNil(t, markers)
}
