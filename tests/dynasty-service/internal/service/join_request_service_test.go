package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
)

func TestJoinRequestService_SendJoinRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	joinRequestRepo := repository.NewJoinRequestRepository(db)
	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, nil, "localhost:50054")

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)
	relationship := "offspring"

	t.Run("Success", func(t *testing.T) {
		// Check user age (under 18)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))

		// Get dynasty message
		mock.ExpectQuery("SELECT message FROM dynasty_messages").
			WithArgs("receiver_message").
			WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("Test message"))

		// Create join request (status 0 = pending)
		mock.ExpectExec("INSERT INTO join_requests").
			WithArgs(fromUserID, toUserID, 0, relationship, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Create child permissions
		mock.ExpectExec("INSERT INTO children_permissions").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		permissions := &models.ChildPermission{
			BFR: true,
			SF:  false,
			W:   true,
		}

		req, err := service.SendJoinRequest(ctx, fromUserID, toUserID, relationship, nil, permissions)
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, fromUserID, req.FromUser)
		assert.Equal(t, toUserID, req.ToUser)
		assert.Equal(t, relationship, req.Relationship)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_AcceptJoinRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	joinRequestRepo := repository.NewJoinRequestRepository(db)
	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, nil, "localhost:50054")

	ctx := context.Background()
	requestID := uint64(1)
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		// Get join request
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
				AddRow(requestID, fromUserID, toUserID, 0, "offspring", "Test", time.Now(), time.Now()))

		// Update status
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Get dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, time.Now(), time.Now()))

		// Get family
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, time.Now(), time.Now()))

		// Add family member
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), toUserID, "offspring").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Check user ages
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))

		// Get existing permissions
		mock.ExpectQuery("SELECT id, user_id").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "verified", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at"}).
				AddRow(1, toUserID, false, true, false, true, false, false, false, false, false, false, false, time.Now(), time.Now()))

		// Update permissions
		mock.ExpectExec("UPDATE children_permissions").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), toUserID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.AcceptJoinRequest(ctx, requestID, toUserID)
		require.NoError(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_RejectJoinRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	joinRequestRepo := repository.NewJoinRequestRepository(db)
	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, nil, "localhost:50054")

	ctx := context.Background()
	requestID := uint64(1)
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		// Get join request
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
				AddRow(requestID, fromUserID, toUserID, 0, "offspring", "Test", time.Now(), time.Now()))

		// Update status to rejected (-1 per API spec)
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(-1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.RejectJoinRequest(ctx, requestID, toUserID)
		require.NoError(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_DeleteJoinRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	joinRequestRepo := repository.NewJoinRequestRepository(db)
	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, nil, "localhost:50054")

	ctx := context.Background()
	requestID := uint64(1)
	fromUserID := uint64(1)

	t.Run("Success", func(t *testing.T) {
		// Get join request
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
				AddRow(requestID, fromUserID, 2, 0, "offspring", "Test", time.Now(), time.Now()))

		// Delete request
		mock.ExpectExec("DELETE FROM join_requests").
			WithArgs(requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.DeleteJoinRequest(ctx, requestID, fromUserID)
		require.NoError(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
