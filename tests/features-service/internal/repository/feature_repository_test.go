package repository_test

import (
	"context"
	"testing"

	"metargb/features-service/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewFeatureRepository(db)
	ctx := context.Background()

	featureID := uint64(1)

	feature, properties, err := repo.FindByID(ctx, featureID)

	require.NoError(t, err)
	require.NotNil(t, feature)
	require.NotNil(t, properties)
	assert.Equal(t, featureID, feature.ID)
}

func TestFeatureRepository_FindByBoundingBox(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewFeatureRepository(db)
	ctx := context.Background()

	// Test bounding box: small area
	points := []string{
		"0.0,0.0",   // minX, minY
		"1.0,0.0",   // maxX, minY
		"1.0,1.0",   // maxX, maxY
		"0.0,1.0",   // minX, maxY
	}

	features, err := repo.FindByBoundingBox(ctx, points, false)

	require.NoError(t, err)
	assert.NotNil(t, features)
	// Note: Actual count depends on test data
}

func TestFeatureRepository_UpdateOwner(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewFeatureRepository(db)
	ctx := context.Background()

	featureID := uint64(1)
	newOwnerID := uint64(100)

	err := repo.UpdateOwner(ctx, featureID, newOwnerID)

	require.NoError(t, err)

	// Verify ownership changed
	feature, _, err := repo.FindByID(ctx, featureID)
	require.NoError(t, err)
	assert.Equal(t, newOwnerID, feature.OwnerID)
}

