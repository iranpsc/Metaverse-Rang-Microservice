package repository_test

import (
	"context"
	"testing"

	"metarang/features-service/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMyFeaturesWhere(t *testing.T) {
	t.Run("owner only", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(42, "", "")
		assert.Equal(t, "f.owner_id = ?", where)
		require.Equal(t, []interface{}{uint64(42)}, args)
	})

	t.Run("search by properties id or address", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(7, "TO111", "")
		assert.Contains(t, where, "fp.id LIKE ?")
		assert.Contains(t, where, "fp.address LIKE ?")
		require.Equal(t, []interface{}{uint64(7), "%TO111%", "%TO111%"}, args)
	})

	t.Run("filter by karbari", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(7, "", "m")
		assert.Contains(t, where, "fp.karbari IN (?)")
		require.Equal(t, []interface{}{uint64(7), "m"}, args)
	})

	t.Run("filter by multiple karbaris", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(7, "", "m,t")
		assert.Contains(t, where, "fp.karbari IN (?,?)")
		require.Equal(t, []interface{}{uint64(7), "m", "t"}, args)
	})

	t.Run("search and filter together", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(3, "block", "t")
		assert.Contains(t, where, "fp.id LIKE ?")
		assert.Contains(t, where, "fp.address LIKE ?")
		assert.Contains(t, where, "fp.karbari IN (?)")
		require.Equal(t, []interface{}{uint64(3), "%block%", "%block%", "t"}, args)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(1, "  TO111  ", "  a  ")
		assert.NotContains(t, where, "  TO111  ")
		require.Equal(t, []interface{}{uint64(1), "%TO111%", "%TO111%", "a"}, args)
	})

	t.Run("trims and ignores empty karbari values", func(t *testing.T) {
		where, args := repository.BuildMyFeaturesWhere(1, "", " , m, , t, ")
		assert.Contains(t, where, "fp.karbari IN (?,?)")
		require.Equal(t, []interface{}{uint64(1), "m", "t"}, args)
	})
}

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
		"0.0,0.0", // minX, minY
		"1.0,0.0", // maxX, minY
		"1.0,1.0", // maxX, maxY
		"0.0,1.0", // minX, maxY
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
