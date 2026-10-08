package repository_test

import (
	"context"
	"database/sql"
	"metarang/auth-service/internal/repository"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchRepository_SearchUsers(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSearchRepository(db)
	ctx := context.Background()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test users
	user1ID := createTestUser(t, db, "john", "john@example.com", "USR001")
	user2ID := createTestUser(t, db, "jane doe", "jane@example.com", "USR002")
	_ = createTestUser(t, db, "bob smith", "bob@example.com", "USR003")

	// Create test KYC for user1
	createTestKYC(t, db, user1ID, "John", "Smith", 1)

	// Create test profile photo for user1
	createTestProfilePhoto(t, db, user1ID, "http://example.com/photo1.jpg")

	// Create test follower relationship
	createTestFollower(t, db, user1ID, user2ID)

	tests := []struct {
		name       string
		searchTerm string
		wantCount  int
		checkFunc  func(t *testing.T, results []*repository.SearchUserResult)
	}{
		{
			name:       "search by name",
			searchTerm: "john",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.Equal(t, user1ID, results[0].User.ID)
				assert.Equal(t, "john", results[0].User.Name)
			},
		},
		{
			name:       "search by name without KYC",
			searchTerm: "jane",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.Equal(t, user2ID, results[0].User.ID)
				assert.Nil(t, results[0].KYC)
			},
		},
		{
			name:       "search by code",
			searchTerm: "USR001",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.Equal(t, "USR001", results[0].User.Code)
			},
		},
		{
			name:       "search by KYC first name",
			searchTerm: "John",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.NotNil(t, results[0].KYC)
				assert.Equal(t, "John", results[0].KYC.Fname)
			},
		},
		{
			name:       "search by KYC last name",
			searchTerm: "Smith",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.NotNil(t, results[0].KYC)
				assert.Equal(t, "Smith", results[0].KYC.Lname)
			},
		},
		{
			name:       "search with multiple terms",
			searchTerm: "jane doe",
			wantCount:  1,
			checkFunc: func(t *testing.T, results []*repository.SearchUserResult) {
				assert.Len(t, results, 1)
				assert.Equal(t, user2ID, results[0].User.ID)
			},
		},
		{
			name:       "empty search term",
			searchTerm: "",
			wantCount:  0,
		},
		{
			name:       "no matches",
			searchTerm: "nonexistent",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.SearchUsers(ctx, tt.searchTerm)
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)

			if tt.checkFunc != nil {
				tt.checkFunc(t, results)
			}
		})
	}

	// Test that results are limited to 5
	t.Run("results limited to 5", func(t *testing.T) {
		// Create 10 test users
		for i := 0; i < 10; i++ {
			createTestUser(t, db, "testuser", "test"+string(rune(i))+"@example.com", "TEST"+string(rune(i)))
		}

		results, err := repo.SearchUsers(ctx, "testuser")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 5)
	})
}

func TestSearchRepository_SearchIsicCodes(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSearchRepository(db)
	ctx := context.Background()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test ISIC codes
	isic1ID := createTestIsicCode(t, db, "Manufacture of textiles", 1311)
	isic2ID := createTestIsicCode(t, db, "Manufacture of beverages", 1104)
	createTestIsicCode(t, db, "Retail trade", 4711)

	tests := []struct {
		name       string
		searchTerm string
		wantIDs    []uint64
	}{
		{
			name:       "search by name",
			searchTerm: "Manufacture",
			wantIDs:    []uint64{isic1ID, isic2ID},
		},
		{
			name:       "search by partial name",
			searchTerm: "textiles",
			wantIDs:    []uint64{isic1ID},
		},
		{
			name:       "empty search term",
			searchTerm: "",
			wantIDs:    []uint64{},
		},
		{
			name:       "no matches",
			searchTerm: "nonexistent",
			wantIDs:    []uint64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.SearchIsicCodes(ctx, tt.searchTerm)
			require.NoError(t, err)

			gotIDs := make([]uint64, len(results))
			for i, r := range results {
				gotIDs[i] = r.ID
			}

			if len(tt.wantIDs) == 0 {
				assert.Empty(t, gotIDs)
			} else {
				assert.ElementsMatch(t, tt.wantIDs, gotIDs)
			}
		})
	}
}

// Helper functions

func setupTestDB(t *testing.T) *sql.DB {
	// Use the same DB setup as other tests
	// Adjust connection string as needed
	dsn := "root:@tcp(localhost:3306)/metarang_db?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("database not available: %v", err)
	}

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		t.Skipf("database not available: %v", err)
	}

	return db
}

func cleanupTestData(t *testing.T, db *sql.DB) {
	// Clean up test data
	_, _ = db.Exec("DELETE FROM follows WHERE follower_id LIKE 'TEST%' OR following_id LIKE 'TEST%'")
	_, _ = db.Exec("DELETE FROM images WHERE imageable_id LIKE 'TEST%'")
	_, _ = db.Exec("DELETE FROM kycs WHERE user_id IN (SELECT id FROM users WHERE name LIKE 'test%' OR name = 'john' OR name = 'jane doe' OR name = 'bob smith')")
	_, _ = db.Exec("DELETE FROM users WHERE name LIKE 'test%' OR name = 'john' OR name = 'jane doe' OR name = 'bob smith'")
	_, _ = db.Exec("DELETE FROM isic_codes WHERE name LIKE '%test%' OR name LIKE 'Manufacture%' OR name = 'Retail trade'")
}

func createTestUser(t *testing.T, db *sql.DB, name, email, code string) uint64 {
	query := `
		INSERT INTO users (name, email, code, password, ip, created_at, updated_at)
		VALUES (?, ?, ?, 'hashed_password', '127.0.0.1', NOW(), NOW())
	`
	result, err := db.Exec(query, name, email, code)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	return uint64(id)
}

func createTestKYC(t *testing.T, db *sql.DB, userID uint64, fname, lname string, status int32) {
	query := `
		INSERT INTO kycs (user_id, fname, lname, melli_code, melli_card, province, status, created_at, updated_at)
		VALUES (?, ?, ?, '1234567890', 'card.jpg', 'Tehran', ?, NOW(), NOW())
	`
	_, err := db.Exec(query, userID, fname, lname, status)
	require.NoError(t, err)
}

func createTestProfilePhoto(t *testing.T, db *sql.DB, userID uint64, url string) {
	query := `
		INSERT INTO images (imageable_type, imageable_id, url, created_at, updated_at)
		VALUES ('App\\Models\\User', ?, ?, NOW(), NOW())
	`
	_, err := db.Exec(query, userID, url)
	require.NoError(t, err)
}

func createTestFollower(t *testing.T, db *sql.DB, followerID, followingID uint64) {
	query := `
		INSERT INTO follows (follower_id, following_id, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`
	_, err := db.Exec(query, followerID, followingID)
	require.NoError(t, err)
}

func createTestIsicCode(t *testing.T, db *sql.DB, name string, code uint64) uint64 {
	query := `
		INSERT INTO isic_codes (name, code, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`
	result, err := db.Exec(query, name, code)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)

	return uint64(id)
}
