package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"metarang/features-service/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestIsicCodeRow(t *testing.T, db *sql.DB, name string, code *uint64) uint64 {
	t.Helper()
	var result sql.Result
	var err error
	if code != nil {
		result, err = db.Exec(
			`INSERT INTO isic_codes (name, code, verified, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW())`,
			name, *code,
		)
	} else {
		result, err = db.Exec(
			`INSERT INTO isic_codes (name, verified, created_at, updated_at) VALUES (?, 0, NOW(), NOW())`,
			name,
		)
	}
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	return uint64(id)
}

func TestIsicCodeRepository_FindPaginatedAndCount(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewIsicCodeRepository(db)

	prefix := "isic-repo-test-"
	defer func() {
		_, _ = db.Exec(`DELETE FROM isic_codes WHERE name LIKE ?`, prefix+"%")
	}()

	code1311 := uint64(1311)
	code1104 := uint64(1104)
	createTestIsicCodeRow(t, db, prefix+"Manufacture of textiles", &code1311)
	createTestIsicCodeRow(t, db, prefix+"Manufacture of beverages", &code1104)
	createTestIsicCodeRow(t, db, prefix+"Retail trade", nil)

	t.Run("list all paginated", func(t *testing.T) {
		total, err := repo.Count(ctx, prefix)
		require.NoError(t, err)
		assert.Equal(t, 3, total)

		items, err := repo.FindPaginated(ctx, prefix, 2, 0)
		require.NoError(t, err)
		assert.Len(t, items, 2)

		items, err = repo.FindPaginated(ctx, prefix, 2, 2)
		require.NoError(t, err)
		assert.Len(t, items, 1)
	})

	t.Run("search by name", func(t *testing.T) {
		total, err := repo.Count(ctx, prefix+"Manufacture")
		require.NoError(t, err)
		assert.Equal(t, 2, total)

		items, err := repo.FindPaginated(ctx, prefix+"Manufacture", 10, 0)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("search by code", func(t *testing.T) {
		total, err := repo.Count(ctx, "1311")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)

		items, err := repo.FindPaginated(ctx, "1311", 10, 0)
		require.NoError(t, err)
		require.NotEmpty(t, items)
		found := false
		for _, item := range items {
			if item.Name == prefix+"Manufacture of textiles" {
				found = true
				require.NotNil(t, item.Code)
				assert.Equal(t, uint64(1311), *item.Code)
			}
		}
		assert.True(t, found)
	})
}
