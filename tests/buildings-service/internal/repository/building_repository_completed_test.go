package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"metarang/buildings-service/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildingRepository_FindCompleted_OnlyPastEndDate(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	repo := repository.NewBuildingRepository(db)
	ctx := context.Background()
	now := time.Now()

	completedModelID := uint64(910001)
	inProgressModelID := uint64(910002)

	require.NoError(t, repo.UpsertBuildingModel(ctx, completedModelID, "Completed", "SKU-C", `[]`,
		`[{"slug":"length","value":30},{"slug":"width","value":50}]`, `{}`, 10))
	require.NoError(t, repo.UpsertBuildingModel(ctx, inProgressModelID, "In Progress", "SKU-I", `[]`, `[]`, `{}`, 10))

	featureCompleted := uint64(91001)
	featureInProgress := uint64(91002)

	require.NoError(t, repo.CreateBuilding(ctx, featureCompleted, uint64(1), fmt.Sprintf("%d", completedModelID), "10", "0", "0,0", "",
		now.Add(-48*time.Hour), now.Add(-1*time.Hour), 100))
	require.NoError(t, repo.CreateBuilding(ctx, featureInProgress, uint64(1), fmt.Sprintf("%d", inProgressModelID), "10", "0", "0,0", "",
		now.Add(-1*time.Hour), now.Add(48*time.Hour), 100))

	rows, err := repo.FindCompleted(ctx, now, 10, 0)
	require.NoError(t, err)

	foundCompleted := false
	for _, row := range rows {
		assert.NotEqual(t, featureInProgress, row.FeatureID, "in-progress building must not appear")
		if row.FeatureID == featureCompleted {
			foundCompleted = true
			assert.Contains(t, row.AttributesJSON, `"length"`)
			assert.Contains(t, row.AttributesJSON, `"width"`)
		}
	}
	assert.True(t, foundCompleted, "expected completed building in results")

	count, err := repo.CountCompleted(ctx, now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)
}
