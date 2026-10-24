package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"

	"metarang/dynasty-service/internal/models"
)

func TestPermissionRepository_DefaultAndCreateUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewPermissionRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, BFR, SF, W, JU, DM").WillReturnRows(sqlmock.NewRows([]string{"id", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at"}).AddRow(1, true, true, true, true, true, true, true, true, true, true, now, now))
	def, err := r.GetDefaultPermissions(ctx)
	require.NoError(t, err)
	require.NotNil(t, def)

	perm := &models.ChildPermission{UserID: 4, Verified: true, BFR: true, SF: true, W: true, JU: true, DM: true, PIUP: true, PITC: true, PIC: true, ESOO: true, COTB: true}
	mock.ExpectExec("INSERT INTO children_permissions").WillReturnResult(sqlmock.NewResult(5, 1))
	require.NoError(t, r.CreatePermission(ctx, perm))
	assert.Equal(t, uint64(5), perm.ID)

	mock.ExpectExec("UPDATE children_permissions SET SF").WithArgs(true, uint64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.UpdatePermission(ctx, 4, "SF", true))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPermissionRepository_GetByUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewPermissionRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("FROM children_permissions").WithArgs(uint64(8)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "verified", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at"}).AddRow(1, 8, true, true, true, true, true, true, true, true, true, true, true, now, now))
	perm, err := r.GetByUserID(ctx, 8)
	require.NoError(t, err)
	require.NotNil(t, perm)
	assert.Equal(t, uint64(8), perm.UserID)

	require.NoError(t, mock.ExpectationsWereMet())
}
