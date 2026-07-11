package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"metargb/features-service/internal/repository"
	"metargb/features-service/tests/internal/testutil"

	_ "github.com/go-sql-driver/mysql"
)

func setupTestDBForMap(t *testing.T) *sql.DB {
	return testutil.OpenMySQLOrSkip(t)
}

func TestMapRepository_FindAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDBForMap(t)
	defer db.Close()

	repo := repository.NewMapRepository(db)
	ctx := context.Background()

	t.Run("find all maps", func(t *testing.T) {
		maps, err := repo.FindAll(ctx)
		if err != nil {
			t.Fatalf("FindAll failed: %v", err)
		}

		// Should return at least 0 maps (could be empty)
		if maps == nil {
			t.Error("FindAll returned nil slice")
		}
	})
}

func TestMapRepository_FindByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDBForMap(t)
	defer db.Close()

	repo := repository.NewMapRepository(db)
	ctx := context.Background()

	t.Run("find existing map", func(t *testing.T) {
		// First, create a test map
		_, err := db.ExecContext(ctx, `
			INSERT INTO maps (name, karbari, publish_date, publisher_name, polygon_count, 
			                  total_area, first_id, last_id, status, fileName)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "Test Map", "m", time.Now(), "Test Publisher", 100, 1000, "1", "100", 1, "test.json")
		if err != nil {
			t.Fatalf("Failed to create test map: %v", err)
		}

		// Get the inserted ID
		var mapID uint64
		err = db.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&mapID)
		if err != nil {
			t.Fatalf("Failed to get inserted ID: %v", err)
		}

		// Clean up
		defer db.ExecContext(ctx, "DELETE FROM maps WHERE id = ?", mapID)

		// Test FindByID
		m, err := repo.FindByID(ctx, mapID)
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}

		if m == nil {
			t.Fatal("FindByID returned nil for existing map")
		}

		if m.ID != mapID {
			t.Errorf("Expected map ID %d, got %d", mapID, m.ID)
		}

		if m.Name != "Test Map" {
			t.Errorf("Expected name 'Test Map', got '%s'", m.Name)
		}
	})

	t.Run("find non-existing map", func(t *testing.T) {
		m, err := repo.FindByID(ctx, 999999)
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}

		if m != nil {
			t.Error("FindByID returned non-nil for non-existing map")
		}
	})
}

func TestMapRepository_FindFeaturesByMapID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDBForMap(t)
	defer db.Close()

	repo := repository.NewMapRepository(db)
	ctx := context.Background()

	t.Run("find features for map", func(t *testing.T) {
		// Create a test map
		_, err := db.ExecContext(ctx, `
			INSERT INTO maps (name, karbari, publish_date, publisher_name, polygon_count, 
			                  total_area, first_id, last_id, status, fileName)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "Test Map", "m", time.Now(), "Test Publisher", 100, 1000, "1", "100", 1, "test.json")
		if err != nil {
			t.Fatalf("Failed to create test map: %v", err)
		}

		var mapID uint64
		err = db.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&mapID)
		if err != nil {
			t.Fatalf("Failed to get inserted ID: %v", err)
		}

		defer db.ExecContext(ctx, "DELETE FROM maps WHERE id = ?", mapID)

		// Test FindFeaturesByMapID (should work even with no features)
		features, err := repo.FindFeaturesByMapID(ctx, mapID)
		if err != nil {
			t.Fatalf("FindFeaturesByMapID failed: %v", err)
		}

		if features == nil {
			t.Error("FindFeaturesByMapID returned nil slice")
		}
	})
}
