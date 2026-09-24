package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	"metarang/dynasty-service/internal/validation"
)

func newJoinRequestService(t *testing.T, notif service.NotificationPort, prizeRepo *repository.PrizeRepository) (sqlmock.Sqlmock, *sql.DB, *service.JoinRequestService) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	if prizeRepo == nil {
		prizeRepo = repository.NewPrizeRepository(db)
	}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		prizeRepo,
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)
	return mock, db, svc
}

func expectSendJoinRequestRules(mock sqlmock.Sqlmock, fromUser, toUser uint64, relationship string) {
	expectSendJoinRequestRulesWithSender(mock, fromUser, toUser, relationship, false, true)
}

func expectSendJoinRequestRulesWithSender(mock sqlmock.Sqlmock, fromUser, toUser uint64, relationship string, senderUnder18, hasDM bool) {
	mock.ExpectQuery("SELECT TIMESTAMPDIFF").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(senderUnder18))
	if senderUnder18 {
		mock.ExpectQuery("SELECT verified AND DM").
			WithArgs(fromUser).
			WillReturnRows(sqlmock.NewRows([]string{"has_permission"}).AddRow(hasDM))
	}
	if senderUnder18 && !hasDM {
		return
	}
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser, toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser, toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	now := time.Now()
	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
			AddRow(uint64(1), fromUser, uint64(100), now, now))
	mock.ExpectQuery("SELECT id, dynasty_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
			AddRow(uint64(1), uint64(1), now, now))
	switch relationship {
	case "father", "mother", "husband", "wife", "offspring":
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members`).
			WithArgs(uint64(1), relationship).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
}

func joinRequestRow(id, fromUser, toUser uint64, relationship string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
		AddRow(id, fromUser, toUser, 0, relationship, "hi", now, now)
}

func TestJoinRequestService_SendJoinRequest_ValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	perms := &models.ChildPermission{BFR: true}

	t.Run("InvalidRelationship", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		_, err := svc.SendJoinRequest(ctx, 1, 2, "cousin", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "نوع رابطه نامعتبر است")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CannotAddSelf", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		_, err := svc.SendJoinRequest(ctx, 1, 1, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "خودتان")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SenderHasNoDynasty", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "سلسله")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DuplicatePendingRequest", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ارسال")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PreviouslyRejectedRequest", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "رد")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ReceiverAlreadyInFamily", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "اضافه")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Under18SenderWithoutDM", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		expectSendJoinRequestRulesWithSender(mock, 1, 2, "brother", true, false)
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "مجاز به مدیریت")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RelationshipLimitExceeded", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(1), uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		now := time.Now()
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(uint64(1), uint64(1), uint64(100), now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(uint64(1), uint64(1), now, now))
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members`).
			WithArgs(uint64(1), "father").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "father", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "پدر")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("OffspringOver18Rejected", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		expectSendJoinRequestRules(mock, 1, 2, "offspring")
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "offspring", nil, perms)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set permissions for offspring over 18")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CheckAgeError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(1)).
			WillReturnError(errors.New("age query failed"))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "offspring", nil, perms)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check user age")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DynastyMessageError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		expectSendJoinRequestRules(mock, 1, 2, "brother")
		mock.ExpectQuery("SELECT message FROM dynasty_messages").
			WithArgs("reciever_message").
			WillReturnError(errors.New("msg failed"))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get dynasty message")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateJoinRequestError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		expectSendJoinRequestRules(mock, 1, 2, "brother")
		mock.ExpectQuery("SELECT message FROM dynasty_messages").
			WithArgs("reciever_message").
			WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("tmpl"))
		expectUserBasicInfo(mock, 1, "S1", "Sender")
		expectUserBasicInfo(mock, 2, "R2", "Receiver")
		mock.ExpectExec("INSERT INTO join_requests").
			WillReturnError(errors.New("insert failed"))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "brother", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create join request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateChildPermissionError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		expectSendJoinRequestRules(mock, 1, 2, "offspring")
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(uint64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))
		mock.ExpectQuery("SELECT message FROM dynasty_messages").
			WithArgs("reciever_message").
			WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("tmpl"))
		expectUserBasicInfo(mock, 1, "S1", "Sender")
		expectUserBasicInfo(mock, 2, "R2", "Receiver")
		mock.ExpectExec("INSERT INTO join_requests").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO children_permissions").
			WillReturnError(errors.New("perm insert failed"))
		_, err := svc.SendJoinRequest(ctx, 1, 2, "offspring", nil, perms)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create child permissions")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestJoinRequestService_GetRequests(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	mock, _, svc := newJoinRequestService(t, nil, nil)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM join_requests WHERE from_user`).
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM join_requests").
		WithArgs(uint64(1), int32(10), int32(0)).
		WillReturnRows(joinRequestRow(1, 1, 2, "brother", now))
	sent, total, err := svc.GetSentRequests(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	require.Len(t, sent, 1)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM join_requests WHERE to_user`).
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM join_requests").
		WithArgs(uint64(2), int32(10), int32(0)).
		WillReturnRows(joinRequestRow(2, 1, 2, "sister", now))
	received, total, err := svc.GetReceivedRequests(ctx, 2, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	require.Len(t, received, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_GetJoinRequest(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("SuccessAsSender", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(uint64(5)).
			WillReturnRows(joinRequestRow(5, 1, 2, "brother", now))
		req, err := svc.GetJoinRequest(ctx, 5, 1)
		require.NoError(t, err)
		require.NotNil(t, req)
		assert.Equal(t, uint64(5), req.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SuccessAsReceiver", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(uint64(5)).
			WillReturnRows(joinRequestRow(5, 1, 2, "brother", now))
		req, err := svc.GetJoinRequest(ctx, 5, 2)
		require.NoError(t, err)
		require.NotNil(t, req)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("NotFound", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(uint64(9)).
			WillReturnError(sql.ErrNoRows)
		_, err := svc.GetJoinRequest(ctx, 9, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "join request not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RepoError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(uint64(9)).
			WillReturnError(errors.New("db"))
		_, err := svc.GetJoinRequest(ctx, 9, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get join request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Unauthorized", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(uint64(5)).
			WillReturnRows(joinRequestRow(5, 1, 2, "brother", now))
		_, err := svc.GetJoinRequest(ctx, 5, 99)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized to view this request")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestJoinRequestService_AcceptJoinRequest_ErrorAndFatherPath(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	requestID, fromUserID, toUserID := uint64(1), uint64(10), uint64(20)

	t.Run("NotFound", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(sql.ErrNoRows)
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "join request not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RepoError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(errors.New("db"))
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get join request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Unauthorized", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		err := svc.AcceptJoinRequest(ctx, requestID, fromUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized to accept this request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateStatusError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnError(errors.New("update failed"))
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update request status")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetDynastyError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnError(errors.New("dynasty query failed"))
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RequesterHasNoDynasty", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnError(sql.ErrNoRows)
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requester does not have a dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetFamilyError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnError(errors.New("family query failed"))
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get family")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AddMemberError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectExec("INSERT INTO family_members").
			WillReturnError(errors.New("member insert failed"))
		err := svc.AcceptJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add family member")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("FatherUnder18CreatesDefaultPermissionsAndAwardsPrize", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "father", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), toUserID, "father").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT id, BFR, SF, W, JU, DM").
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at",
			}).AddRow(1, true, true, true, true, true, true, true, true, true, true, now, now))
		mock.ExpectExec("INSERT INTO children_permissions").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs("father").
			WillReturnRows(sqlmock.NewRows([]string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}).
				AddRow(8, "father", 0.1, 0.2, 0.3, 0.4, 1000, now, now))
		mock.ExpectExec("INSERT INTO received_prizes").
			WithArgs(toUserID, uint64(8), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		require.NoError(t, svc.AcceptJoinRequest(ctx, requestID, toUserID))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PrizeAwardFailureDoesNotFailAccept", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "brother", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), toUserID, "brother").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs("brother").
			WillReturnRows(sqlmock.NewRows([]string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}).
				AddRow(8, "brother", 0.1, 0.2, 0.3, 0.4, 1000, now, now))
		mock.ExpectExec("INSERT INTO received_prizes").
			WillReturnError(errors.New("award failed"))
		require.NoError(t, svc.AcceptJoinRequest(ctx, requestID, toUserID))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AcceptNotificationUserInfoFailureSkipsSend", func(t *testing.T) {
		notif := &recordingNotificationPort{}
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		svc := service.NewJoinRequestService(
			repository.NewJoinRequestRepository(db),
			repository.NewDynastyRepository(db),
			repository.NewFamilyRepository(db),
			nil,
			validation.NewFamilyValidator(repository.NewValidationRepository(db)),
			notif,
			"",
		)

		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "cousin", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), toUserID, "cousin").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT u.id, u.code, u.name").
			WithArgs(fromUserID).
			WillReturnError(sql.ErrNoRows)
		require.NoError(t, svc.AcceptJoinRequest(ctx, requestID, toUserID))
		assert.Empty(t, notif.snapshot())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AcceptNotificationReceiverInfoFailureSkipsSend", func(t *testing.T) {
		notif := &recordingNotificationPort{}
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		svc := service.NewJoinRequestService(
			repository.NewJoinRequestRepository(db),
			repository.NewDynastyRepository(db),
			repository.NewFamilyRepository(db),
			nil,
			validation.NewFamilyValidator(repository.NewValidationRepository(db)),
			notif,
			"",
		)

		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "cousin", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, fromUserID, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), toUserID, "cousin").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
		expectUserBasicInfo(mock, fromUserID, "F10", "From")
		mock.ExpectQuery("SELECT u.id, u.code, u.name").
			WithArgs(toUserID).
			WillReturnError(sql.ErrNoRows)
		require.NoError(t, svc.AcceptJoinRequest(ctx, requestID, toUserID))
		assert.Empty(t, notif.snapshot())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestJoinRequestService_RejectAndDelete_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	requestID, fromUserID, toUserID := uint64(3), uint64(1), uint64(2)

	t.Run("RejectNotFound", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(sql.ErrNoRows)
		err := svc.RejectJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "join request not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RejectRepoError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(errors.New("db"))
		err := svc.RejectJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get join request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RejectUnauthorized", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		err := svc.RejectJoinRequest(ctx, requestID, fromUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized to reject this request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RejectUpdateError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(-1, requestID).
			WillReturnError(errors.New("update failed"))
		err := svc.RejectJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update request status")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RejectNotificationUserInfoFailureSkipsSend", func(t *testing.T) {
		notif := &recordingNotificationPort{}
		mock, _, svc := newJoinRequestService(t, notif, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(-1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT u.id, u.code, u.name").
			WithArgs(fromUserID).
			WillReturnError(sql.ErrNoRows)
		require.NoError(t, svc.RejectJoinRequest(ctx, requestID, toUserID))
		assert.Empty(t, notif.snapshot())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RejectNotificationReceiverInfoFailureSkipsSend", func(t *testing.T) {
		notif := &recordingNotificationPort{}
		mock, _, svc := newJoinRequestService(t, notif, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		mock.ExpectExec("UPDATE join_requests SET status").
			WithArgs(-1, requestID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		expectUserBasicInfo(mock, fromUserID, "A", "Alice")
		mock.ExpectQuery("SELECT u.id, u.code, u.name").
			WithArgs(toUserID).
			WillReturnError(sql.ErrNoRows)
		require.NoError(t, svc.RejectJoinRequest(ctx, requestID, toUserID))
		assert.Empty(t, notif.snapshot())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(sql.ErrNoRows)
		err := svc.DeleteJoinRequest(ctx, requestID, fromUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "join request not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteRepoError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnError(errors.New("db"))
		err := svc.DeleteJoinRequest(ctx, requestID, fromUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get join request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteUnauthorized", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		err := svc.DeleteJoinRequest(ctx, requestID, toUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized to delete this request")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteExecError", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, from_user, to_user").
			WithArgs(requestID).
			WillReturnRows(joinRequestRow(requestID, fromUserID, toUserID, "sister", now))
		mock.ExpectExec("DELETE FROM join_requests").
			WithArgs(requestID).
			WillReturnError(errors.New("delete failed"))
		err := svc.DeleteJoinRequest(ctx, requestID, fromUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete request")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestJoinRequestService_GetPrizeAndDefaultPermissions(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("NilPrizeRepo", func(t *testing.T) {
		svc := service.NewJoinRequestService(nil, nil, nil, nil, nil, nil, "")
		prize, err := svc.GetPrizeByRelationship(ctx, "brother")
		require.NoError(t, err)
		assert.Nil(t, prize)
	})

	t.Run("GetPrizeByRelationship", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("FROM dynasty_prizes").
			WithArgs("sister").
			WillReturnRows(sqlmock.NewRows([]string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}).
				AddRow(2, "sister", 0.1, 0.2, 0.3, 0.4, 500, now, now))
		prize, err := svc.GetPrizeByRelationship(ctx, "sister")
		require.NoError(t, err)
		require.NotNil(t, prize)
		assert.Equal(t, "sister", prize.Member)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetDefaultPermissions", func(t *testing.T) {
		mock, _, svc := newJoinRequestService(t, nil, nil)
		mock.ExpectQuery("SELECT id, BFR, SF, W, JU, DM").
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at",
			}).AddRow(1, true, false, true, false, true, false, true, false, true, false, now, now))
		perm, err := svc.GetDefaultPermissions(ctx)
		require.NoError(t, err)
		require.NotNil(t, perm)
		assert.True(t, perm.BFR)
		assert.False(t, perm.SF)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestJoinRequestService_SendJoinRequest_UnknownRelationshipTitle(t *testing.T) {
	mock, _, svc := newJoinRequestService(t, nil, nil)

	_, err := svc.SendJoinRequest(context.Background(), 1, 2, "cousin", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "نوع رابطه نامعتبر است")
	require.NoError(t, mock.ExpectationsWereMet())
}
