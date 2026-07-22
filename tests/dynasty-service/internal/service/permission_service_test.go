package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

func TestPermissionService_UpdateChildPermission(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	permissionRepo := repository.NewPermissionRepository(db)
	joinRequestRepo := repository.NewJoinRequestRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	dynastyRepo := repository.NewDynastyRepository(db)
	service := service.NewPermissionService(permissionRepo, joinRequestRepo, familyRepo, dynastyRepo)

	ctx := context.Background()
	parentUserID := uint64(1)
	childUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		// Check user age (under 18)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(childUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))

		// Get dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(parentUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, parentUserID, 100, time.Now(), time.Now()))

		// Get family
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, time.Now(), time.Now()))

		// Get family members (with pagination: page=1, perPage=1000)
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))
		mock.ExpectQuery("SELECT id, family_id, user_id").
			WithArgs(1, 1000, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).
				AddRow(1, 1, parentUserID, "owner", time.Now(), time.Now()).
				AddRow(2, 1, childUserID, "offspring", time.Now(), time.Now()))

		// Get existing permissions
		mock.ExpectQuery("SELECT id, user_id").
			WithArgs(childUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "verified", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at"}).
				AddRow(1, childUserID, true, false, false, false, false, false, false, false, false, false, false, time.Now(), time.Now()))

		// Update specific permission
		mock.ExpectExec("UPDATE children_permissions SET BFR = \\?").
			WithArgs(true, childUserID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.UpdateChildPermission(ctx, parentUserID, childUserID, "BFR", true)
		require.NoError(t, err)
	})

	t.Run("CannotControlSelf", func(t *testing.T) {
		canControl, err := service.CanControlPermissions(ctx, parentUserID, parentUserID)
		require.NoError(t, err)
		assert.False(t, canControl)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
