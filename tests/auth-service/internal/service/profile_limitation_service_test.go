package service

import (
	"context"
	"database/sql"
	"testing"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProfileLimitationTestService(t *testing.T) (ProfileLimitationService, *sql.DB) {
	dsn := "root@tcp(localhost:3306)/metargb_db_test?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Database not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("Database ping failed: %v", err)
	}

	// Clean up test data
	_, _ = db.Exec("DELETE FROM profile_limitations")
	_, _ = db.Exec("DELETE FROM users WHERE id IN (1, 2, 3)")

	// Create test users
	_, _ = db.Exec("INSERT INTO users (id, name, email, phone, password, code, created_at, updated_at) VALUES (1, 'User 1', 'user1@test.com', '09123456789', 'password', 'USER1', NOW(), NOW())")
	_, _ = db.Exec("INSERT INTO users (id, name, email, phone, password, code, created_at, updated_at) VALUES (2, 'User 2', 'user2@test.com', '09123456790', 'password', 'USER2', NOW(), NOW())")
	_, _ = db.Exec("INSERT INTO users (id, name, email, phone, password, code, created_at, updated_at) VALUES (3, 'User 3', 'user3@test.com', '09123456791', 'password', 'USER3', NOW(), NOW())")

	limitationRepo := repository.NewProfileLimitationRepository(db)
	userRepo := repository.NewUserRepository(db, "")
	service := NewProfileLimitationService(limitationRepo, userRepo)

	return service, db
}

func TestProfileLimitationService_Create(t *testing.T) {
	service, db := setupProfileLimitationTestService(t)
	defer db.Close()

	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		options := models.DefaultOptions()
		options.Follow = false
		options.SendMessage = false

		limitation, err := service.Create(ctx, 1, 2, options, "Test note")
		require.NoError(t, err)
		assert.NotZero(t, limitation.ID)
		assert.Equal(t, uint64(1), limitation.LimiterUserID)
		assert.Equal(t, uint64(2), limitation.LimitedUserID)
		assert.False(t, limitation.Options.Follow)
		assert.False(t, limitation.Options.SendMessage)
		assert.True(t, limitation.Note.Valid)
		assert.Equal(t, "Test note", limitation.Note.String)
	})

	t.Run("duplicate creation fails", func(t *testing.T) {
		options := models.DefaultOptions()
		_, err := service.Create(ctx, 1, 2, options, "")
		assert.Error(t, err)
		assert.Equal(t, ErrProfileLimitationAlreadyExists, err)
	})

	t.Run("invalid limited user fails", func(t *testing.T) {
		options := models.DefaultOptions()
		_, err := service.Create(ctx, 1, 99999, options, "")
		assert.Error(t, err)
		assert.Equal(t, ErrUserNotFound, err)
	})

	t.Run("invalid limiter user fails", func(t *testing.T) {
		options := models.DefaultOptions()
		_, err := service.Create(ctx, 99999, 2, options, "")
		assert.Error(t, err)
		assert.Equal(t, ErrUserNotFound, err)
	})

	t.Run("note too long fails", func(t *testing.T) {
		options := models.DefaultOptions()
		longNote := make([]byte, 501)
		for i := range longNote {
			longNote[i] = 'a'
		}
		_, err := service.Create(ctx, 1, 3, options, string(longNote))
		assert.Error(t, err)
		assert.Equal(t, ErrNoteTooLong, err)
	})
}

func TestProfileLimitationService_Update(t *testing.T) {
	service, db := setupProfileLimitationTestService(t)
	defer db.Close()

	ctx := context.Background()

	// Create a limitation
	options := models.DefaultOptions()
	limitation, err := service.Create(ctx, 1, 2, options, "Original note")
	require.NoError(t, err)

	t.Run("successful update", func(t *testing.T) {
		newOptions := models.DefaultOptions()
		newOptions.Follow = false
		newOptions.SendMessage = false

		updated, err := service.Update(ctx, limitation.ID, 1, newOptions, "Updated note")
		require.NoError(t, err)
		assert.False(t, updated.Options.Follow)
		assert.False(t, updated.Options.SendMessage)
		assert.True(t, updated.Note.Valid)
		assert.Equal(t, "Updated note", updated.Note.String)
	})

	t.Run("unauthorized update fails", func(t *testing.T) {
		newOptions := models.DefaultOptions()
		_, err := service.Update(ctx, limitation.ID, 2, newOptions, "")
		assert.Error(t, err)
		assert.Equal(t, ErrUnauthorized, err)
	})

	t.Run("not found fails", func(t *testing.T) {
		newOptions := models.DefaultOptions()
		_, err := service.Update(ctx, 99999, 1, newOptions, "")
		assert.Error(t, err)
		assert.Equal(t, ErrProfileLimitationNotFound, err)
	})

	t.Run("note too long fails", func(t *testing.T) {
		newOptions := models.DefaultOptions()
		longNote := make([]byte, 501)
		for i := range longNote {
			longNote[i] = 'a'
		}
		_, err := service.Update(ctx, limitation.ID, 1, newOptions, string(longNote))
		assert.Error(t, err)
		assert.Equal(t, ErrNoteTooLong, err)
	})
}

func TestProfileLimitationService_Delete(t *testing.T) {
	service, db := setupProfileLimitationTestService(t)
	defer db.Close()

	ctx := context.Background()

	// Create a limitation
	options := models.DefaultOptions()
	limitation, err := service.Create(ctx, 1, 2, options, "")
	require.NoError(t, err)

	t.Run("successful delete", func(t *testing.T) {
		err := service.Delete(ctx, limitation.ID, 1)
		assert.NoError(t, err)

		// Verify deletion
		_, err = service.GetByID(ctx, limitation.ID)
		assert.Error(t, err)
		assert.Equal(t, ErrProfileLimitationNotFound, err)
	})

	t.Run("unauthorized delete fails", func(t *testing.T) {
		// Create another limitation
		limitation2, err := service.Create(ctx, 1, 3, options, "")
		require.NoError(t, err)

		err = service.Delete(ctx, limitation2.ID, 2)
		assert.Error(t, err)
		assert.Equal(t, ErrUnauthorized, err)
	})

	t.Run("not found fails", func(t *testing.T) {
		err := service.Delete(ctx, 99999, 1)
		assert.Error(t, err)
		assert.Equal(t, ErrProfileLimitationNotFound, err)
	})
}

func TestProfileLimitationService_GetByID(t *testing.T) {
	service, db := setupProfileLimitationTestService(t)
	defer db.Close()

	ctx := context.Background()

	// Create a limitation
	options := models.DefaultOptions()
	options.Follow = false
	limitation, err := service.Create(ctx, 1, 2, options, "Test note")
	require.NoError(t, err)

	t.Run("successful get", func(t *testing.T) {
		found, err := service.GetByID(ctx, limitation.ID)
		require.NoError(t, err)
		assert.Equal(t, limitation.ID, found.ID)
		assert.False(t, found.Options.Follow)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetByID(ctx, 99999)
		assert.Error(t, err)
		assert.Equal(t, ErrProfileLimitationNotFound, err)
	})
}

func TestProfileLimitationService_GetBetweenUsers(t *testing.T) {
	service, db := setupProfileLimitationTestService(t)
	defer db.Close()

	ctx := context.Background()

	// Create a limitation (user 1 limiting user 2)
	options := models.DefaultOptions()
	limitation, err := service.Create(ctx, 1, 2, options, "Test note")
	require.NoError(t, err)

	t.Run("find from limiter perspective", func(t *testing.T) {
		found, err := service.GetBetweenUsers(ctx, 1, 2)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, limitation.ID, found.ID)
	})

	t.Run("find from limited perspective", func(t *testing.T) {
		found, err := service.GetBetweenUsers(ctx, 2, 1)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, limitation.ID, found.ID)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := service.GetBetweenUsers(ctx, 2, 3)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestProfileLimitationService_ValidateOptions(t *testing.T) {
	service, _ := setupProfileLimitationTestService(t)

	t.Run("valid options", func(t *testing.T) {
		options := models.DefaultOptions()
		err := service.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("custom options", func(t *testing.T) {
		options := models.ProfileLimitationOptions{
			Follow:                false,
			SendMessage:           false,
			Share:                 true,
			SendTicket:            true,
			ViewProfileImages:     false,
			ViewFeaturesLocations: true,
		}
		err := service.ValidateOptions(options)
		assert.NoError(t, err)
	})
}
