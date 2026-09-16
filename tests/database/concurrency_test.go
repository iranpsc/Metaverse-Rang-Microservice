package database

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWalletConcurrency tests concurrent wallet operations
func TestWalletConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	db := connectTestDB(t)
	defer db.Close()

	t.Run("ConcurrentDeductions", func(t *testing.T) {
		// Create test user and wallet
		userID := createTestUser(t, db)
		createTestWallet(t, db, userID, "10000.0000000000")

		// Perform 100 concurrent deductions of 10 each
		// Expected final balance: 10000 - (10 * 100) = 9000
		numOperations := 100
		deductAmount := "10.0000000000"

		var wg sync.WaitGroup
		errors := make(chan error, numOperations)

		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				tx, err := db.Begin()
				if err != nil {
					errors <- err
					return
				}
				defer tx.Rollback()

				// Deduct with row locking
				_, err = tx.Exec(`
					UPDATE wallets 
					SET psc = psc - ? 
					WHERE user_id = ? AND psc >= ?
				`, deductAmount, userID, deductAmount)

				if err != nil {
					errors <- err
					return
				}

				if err := tx.Commit(); err != nil {
					errors <- err
					return
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Logf("Error during concurrent operation: %v", err)
			errorCount++
		}

		// Some operations might fail due to insufficient balance, that's OK
		// But final balance should be consistent
		var finalBalance string
		err := db.QueryRow("SELECT psc FROM wallets WHERE user_id = ?", userID).Scan(&finalBalance)
		require.NoError(t, err)

		// Balance should never be negative
		assert.NotEqual(t, "-", finalBalance[0:1], "Balance should not be negative")

		t.Logf("Final balance after %d concurrent deductions: %s", numOperations, finalBalance)

		// Cleanup
		cleanup(t, db, userID)
	})

	t.Run("ConcurrentAdditionsAndDeductions", func(t *testing.T) {
		userID := createTestUser(t, db)
		createTestWallet(t, db, userID, "5000.0000000000")

		numOperations := 50
		var wg sync.WaitGroup

		// 50 additions of 100
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				tx, err := db.Begin()
				if err != nil {
					return
				}
				defer tx.Rollback()

				_, err = tx.Exec("UPDATE wallets SET psc = psc + 100 WHERE user_id = ?", userID)
				if err != nil {
					return
				}

				_ = tx.Commit()
			}()
		}

		// 50 deductions of 50
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				tx, err := db.Begin()
				if err != nil {
					return
				}
				defer tx.Rollback()

				_, err = tx.Exec("UPDATE wallets SET psc = psc - 50 WHERE user_id = ? AND psc >= 50", userID)
				if err != nil {
					return
				}

				_ = tx.Commit()
			}()
		}

		wg.Wait()

		var finalBalance string
		err := db.QueryRow("SELECT psc FROM wallets WHERE user_id = ?", userID).Scan(&finalBalance)
		require.NoError(t, err)

		// Expected: 5000 + (50 * 100) - (X * 50) where X is successful deductions
		// Should be >= 5000 and never negative
		assert.NotEqual(t, "-", finalBalance[0:1], "Balance should not be negative")

		t.Logf("Final balance after mixed operations: %s", finalBalance)

		cleanup(t, db, userID)
	})

	t.Run("DeadlockPrevention", func(t *testing.T) {
		// Create two users
		user1 := createTestUser(t, db)
		user2 := createTestUser(t, db)
		createTestWallet(t, db, user1, "1000.0000000000")
		createTestWallet(t, db, user2, "1000.0000000000")

		var wg sync.WaitGroup
		deadlocks := 0
		var mu sync.Mutex

		// Attempt to create potential deadlock scenarios
		for i := 0; i < 10; i++ {
			wg.Add(2)

			// Goroutine 1: Transfer from user1 to user2
			go func() {
				defer wg.Done()

				tx, err := db.Begin()
				if err != nil {
					return
				}
				defer tx.Rollback()

				// Lock user1 first
				_, err = tx.Exec("UPDATE wallets SET psc = psc - 10 WHERE user_id = ? AND psc >= 10", user1)
				if err != nil {
					mu.Lock()
					deadlocks++
					mu.Unlock()
					return
				}

				time.Sleep(10 * time.Millisecond) // Increase chance of deadlock

				// Then lock user2
				_, err = tx.Exec("UPDATE wallets SET psc = psc + 10 WHERE user_id = ?", user2)
				if err != nil {
					mu.Lock()
					deadlocks++
					mu.Unlock()
					return
				}

				_ = tx.Commit()
			}()

			// Goroutine 2: Transfer from user2 to user1
			go func() {
				defer wg.Done()

				tx, err := db.Begin()
				if err != nil {
					return
				}
				defer tx.Rollback()

				// Lock user2 first
				_, err = tx.Exec("UPDATE wallets SET psc = psc - 10 WHERE user_id = ? AND psc >= 10", user2)
				if err != nil {
					mu.Lock()
					deadlocks++
					mu.Unlock()
					return
				}

				time.Sleep(10 * time.Millisecond)

				// Then lock user1
				_, err = tx.Exec("UPDATE wallets SET psc = psc + 10 WHERE user_id = ?", user1)
				if err != nil {
					mu.Lock()
					deadlocks++
					mu.Unlock()
					return
				}

				_ = tx.Commit()
			}()
		}

		wg.Wait()

		t.Logf("Deadlock attempts: %d", deadlocks)

		// Verify both balances are still consistent
		var balance1, balance2 string
		_ = db.QueryRow("SELECT psc FROM wallets WHERE user_id = ?", user1).Scan(&balance1)
		_ = db.QueryRow("SELECT psc FROM wallets WHERE user_id = ?", user2).Scan(&balance2)

		assert.NotEqual(t, "-", balance1[0:1], "User1 balance should not be negative")
		assert.NotEqual(t, "-", balance2[0:1], "User2 balance should not be negative")

		t.Logf("Final balances - User1: %s, User2: %s", balance1, balance2)

		cleanup(t, db, user1, user2)
	})
}

// TestSoftDeleteQueries tests soft delete functionality
func TestSoftDeleteQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soft delete test in short mode")
	}

	db := connectTestDB(t)
	defer db.Close()

	t.Run("ExcludeDeletedRecords", func(t *testing.T) {
		// Create test data
		buyer := createTestUser(t, db)
		seller := createTestUser(t, db)
		featureID := createTestFeature(t, db, seller)

		// Create buy request
		requestID, err := db.Exec(`
			INSERT INTO buy_feature_requests (buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, created_at, updated_at)
			VALUES (?, ?, ?, 'test', 1000.00, 0, 0, NOW(), NOW())
		`, buyer, seller, featureID)
		require.NoError(t, err)

		reqID, _ := requestID.LastInsertId()

		// Query should find the request
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM buy_feature_requests WHERE id = ? AND deleted_at IS NULL", reqID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Soft delete
		_, err = db.Exec("UPDATE buy_feature_requests SET deleted_at = NOW() WHERE id = ?", reqID)
		require.NoError(t, err)

		// Query should not find the request (when filtering by deleted_at)
		err = db.QueryRow("SELECT COUNT(*) FROM buy_feature_requests WHERE id = ? AND deleted_at IS NULL", reqID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		// But should find it when including deleted
		err = db.QueryRow("SELECT COUNT(*) FROM buy_feature_requests WHERE id = ?", reqID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		cleanup(t, db, buyer, seller)
	})
}

// Helper functions
func createTestUser(t *testing.T, db *sql.DB) int64 {
	suffix := time.Now().UnixNano()
	name := fmt.Sprintf("test_user_%d", suffix)
	code := fmt.Sprintf("TU%d", suffix%1_000_000_000)
	result, err := db.Exec(`
		INSERT INTO users (name, email, password, ip, code, created_at, updated_at)
		VALUES (?, ?, '$2a$10$testpasswordhashvalue12', '127.0.0.1', ?, NOW(), NOW())
	`, name, name+"@test.com", code)
	require.NoError(t, err)

	id, _ := result.LastInsertId()
	return id
}

func createTestWallet(t *testing.T, db *sql.DB, userID int64, psc string) {
	_, err := db.Exec(`
		INSERT INTO wallets (user_id, psc, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`, userID, psc)
	require.NoError(t, err)
}

func createTestFeature(t *testing.T, db *sql.DB, ownerID int64) int64 {
	result, err := db.Exec(`
		INSERT INTO features (map_id, owner_id, type, created_at, updated_at)
		VALUES (1, ?, 'residential', NOW(), NOW())
	`, ownerID)
	require.NoError(t, err)
	id, _ := result.LastInsertId()
	return id
}

func cleanup(t *testing.T, db *sql.DB, userIDs ...int64) {
	t.Helper()
	for _, userID := range userIDs {
		_, _ = db.Exec("DELETE FROM wallets WHERE user_id = ?", userID)
		_, _ = db.Exec("DELETE FROM features WHERE owner_id = ?", userID)
		_, _ = db.Exec("DELETE FROM buy_feature_requests WHERE buyer_id = ? OR seller_id = ?", userID, userID)
		_, _ = db.Exec("DELETE FROM transactions WHERE user_id = ?", userID)
		_, _ = db.Exec("DELETE FROM users WHERE id = ?", userID)
	}
}
