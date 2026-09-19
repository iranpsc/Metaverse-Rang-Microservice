package service_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
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

// recordingNotificationPort captures NotificationPort.SendNotification calls.
type recordingNotificationPort struct {
	mu    sync.Mutex
	calls []notificationCall
	err   error
}

type notificationCall struct {
	UserID           uint64
	NotificationType string
	Title            string
	Message          string
	Data             map[string]string
	SendSMS          bool
	SendEmail        bool
	CtxErr           error
}

func (r *recordingNotificationPort) SendNotification(
	ctx context.Context,
	userID uint64,
	notificationType, title, message string,
	data map[string]string,
	sendSMS, sendEmail bool,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := map[string]string{}
	for k, v := range data {
		copied[k] = v
	}
	r.calls = append(r.calls, notificationCall{
		UserID:           userID,
		NotificationType: notificationType,
		Title:            title,
		Message:          message,
		Data:             copied,
		SendSMS:          sendSMS,
		SendEmail:        sendEmail,
		CtxErr:           ctx.Err(),
	})
	return r.err
}

func (r *recordingNotificationPort) snapshot() []notificationCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]notificationCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func expectUserBasicInfo(mock sqlmock.Sqlmock, userID uint64, code, name string) {
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(userID, code, name))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
}

func expectJoinNotificationChannels(mock sqlmock.Sqlmock, userID uint64, phone, email string, phoneVerified, emailVerified bool) {
	now := time.Now()
	phoneVerifiedAt := sql.NullTime{}
	if phoneVerified {
		phoneVerifiedAt = sql.NullTime{Time: now, Valid: true}
	}
	emailVerifiedAt := sql.NullTime{}
	if emailVerified {
		emailVerifiedAt = sql.NullTime{Time: now, Valid: true}
	}
	mock.ExpectQuery("join_notification_channels").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"phone", "phone_verified_at", "email", "email_verified_at"}).
			AddRow(phone, phoneVerifiedAt, email, emailVerifiedAt))
}

func TestJoinRequestService_SendJoinRequest_NilNotificationClient_SucceedsWithoutPanic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		nil,
		"",
	)

	expectSendJoinRequestRules(mock, 1, 2, "brother")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("tmpl"))
	expectUserBasicInfo(mock, 1, "S1", "Sender")
	expectUserBasicInfo(mock, 2, "R2", "Receiver")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(uint64(1), uint64(2), 0, "brother", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := svc.SendJoinRequest(context.Background(), 1, 2, "brother", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_NotifiesSenderAndReceiver_WithSendSMSAndEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"localhost:50054",
	)

	fromUserID, toUserID := uint64(10), uint64(20)
	relationship := "brother"
	receiverTemplate := "Hi [sender-name] ([sender-code]) wants [relationship] with [reciever-name]"
	senderTemplate := "You asked [reciever-name] ([reciever-code]) for [relationship]"

	expectSendJoinRequestRules(mock, fromUserID, toUserID, relationship)
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(receiverTemplate))
	expectUserBasicInfo(mock, fromUserID, "S10", "Sender")
	expectUserBasicInfo(mock, toUserID, "R20", "Receiver")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(fromUserID, toUserID, 0, relationship, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_confirmation_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(senderTemplate))
	expectJoinNotificationChannels(mock, fromUserID, "09120000001", "sender@example.com", true, true)
	expectJoinNotificationChannels(mock, toUserID, "09120000002", "receiver@example.com", true, true)

	req, err := svc.SendJoinRequest(context.Background(), fromUserID, toUserID, relationship, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotNil(t, req.Message)
	assert.Equal(t, "Hi Sender (S10) wants برادر with Receiver", *req.Message)

	calls := notif.snapshot()
	require.Len(t, calls, 2)

	assert.Equal(t, fromUserID, calls[0].UserID)
	assert.Equal(t, "dynasty_join_request", calls[0].NotificationType)
	assert.Equal(t, "درخواست پیوستن ارسال شد", calls[0].Title)
	assert.Contains(t, calls[0].Message, "Receiver")
	assert.Contains(t, calls[0].Message, "برادر")
	assert.Equal(t, "sent", calls[0].Data["side"])
	assert.Equal(t, relationship, calls[0].Data["relationship"])
	assert.Equal(t, "S10", calls[0].Data["RequesterCode"])
	assert.Equal(t, "Sender", calls[0].Data["RequesterName"])
	assert.Equal(t, "Receiver", calls[0].Data["ReceiverName"])
	assert.Equal(t, "R20", calls[0].Data["ReceiverCode"])
	assert.Equal(t, "Sender", calls[0].Data["RecipientName"])
	assert.Equal(t, "Sender", calls[0].Data["DynastyName"])
	assert.True(t, calls[0].SendSMS, "SendJoinRequest must request SMS delivery")
	assert.True(t, calls[0].SendEmail, "SendJoinRequest must request email delivery")
	assert.NoError(t, calls[0].CtxErr)

	assert.Equal(t, toUserID, calls[1].UserID)
	assert.Equal(t, "dynasty_join_request", calls[1].NotificationType)
	assert.Equal(t, "درخواست پیوستن جدید", calls[1].Title)
	assert.Contains(t, calls[1].Message, "Sender")
	assert.Equal(t, "received", calls[1].Data["side"])
	assert.Equal(t, relationship, calls[1].Data["relationship"])
	assert.Equal(t, "Receiver", calls[1].Data["RecipientName"])
	assert.Equal(t, "Receiver", calls[1].Data["OwnerName"])
	assert.Equal(t, "Sender", calls[1].Data["RequesterName"])
	assert.Equal(t, "S10", calls[1].Data["RequesterCode"])
	assert.True(t, calls[1].SendSMS)
	assert.True(t, calls[1].SendEmail)
	assert.NoError(t, calls[1].CtxErr)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_SkipsSMSEmailWhenContactsUnverified(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	fromUserID, toUserID := uint64(10), uint64(20)
	relationship := "brother"

	expectSendJoinRequestRules(mock, fromUserID, toUserID, relationship)
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("recv [sender-name]"))
	expectUserBasicInfo(mock, fromUserID, "S10", "Sender")
	expectUserBasicInfo(mock, toUserID, "R20", "Receiver")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(fromUserID, toUserID, 0, relationship, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_confirmation_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("sent [reciever-name]"))
	expectJoinNotificationChannels(mock, fromUserID, "09120000001", "sender@example.com", false, false)
	expectJoinNotificationChannels(mock, toUserID, "09120000002", "receiver@example.com", false, false)

	_, err = svc.SendJoinRequest(context.Background(), fromUserID, toUserID, relationship, nil, nil)
	require.NoError(t, err)

	calls := notif.snapshot()
	require.Len(t, calls, 2)
	assert.False(t, calls[0].SendSMS)
	assert.False(t, calls[0].SendEmail)
	assert.False(t, calls[1].SendSMS)
	assert.False(t, calls[1].SendEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_EmptyTemplates_UsesFallbackMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	expectSendJoinRequestRules(mock, 1, 2, "sister")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	expectUserBasicInfo(mock, 1, "A", "Alice")
	expectUserBasicInfo(mock, 2, "B", "Bob")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(uint64(1), uint64(2), 0, "sister", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_confirmation_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	expectJoinNotificationChannels(mock, uint64(1), "09120000001", "alice@example.com", true, true)
	expectJoinNotificationChannels(mock, uint64(2), "09120000002", "bob@example.com", true, false)

	_, err = svc.SendJoinRequest(context.Background(), 1, 2, "sister", nil, nil)
	require.NoError(t, err)

	calls := notif.snapshot()
	require.Len(t, calls, 2)
	assert.Contains(t, calls[0].Message, "B")
	assert.Contains(t, calls[0].Message, "خواهر")
	assert.True(t, calls[0].SendSMS)
	assert.True(t, calls[0].SendEmail)
	assert.Contains(t, calls[1].Message, "A")
	assert.True(t, calls[1].SendSMS)
	assert.False(t, calls[1].SendEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_ReceiverInfoFailure_SkipsNotifications(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	expectSendJoinRequestRules(mock, 1, 2, "wife")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("tmpl"))
	expectUserBasicInfo(mock, 1, "A", "Alice")
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(uint64(2)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(uint64(1), uint64(2), 0, "wife", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = svc.SendJoinRequest(context.Background(), 1, 2, "wife", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, notif.snapshot())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_UserInfoFetchFailure_SkipsNotifications(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	expectSendJoinRequestRules(mock, 1, 2, "wife")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("tmpl"))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)
	expectUserBasicInfo(mock, 2, "B", "Bob")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(uint64(1), uint64(2), 0, "wife", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = svc.SendJoinRequest(context.Background(), 1, 2, "wife", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, notif.snapshot())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_SendJoinRequest_SendNotificationFailure_DoesNotFailMainFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{err: errors.New("sms/email/provider boom")}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	expectSendJoinRequestRules(mock, 1, 2, "husband")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("recv [sender-name]"))
	expectUserBasicInfo(mock, 1, "S", "Sender")
	expectUserBasicInfo(mock, 2, "R", "Receiver")
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(uint64(1), uint64(2), 0, "husband", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_confirmation_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("sent [reciever-name]"))
	expectJoinNotificationChannels(mock, uint64(1), "09120000001", "sender@example.com", true, true)
	expectJoinNotificationChannels(mock, uint64(2), "09120000002", "receiver@example.com", true, true)

	req, err := svc.SendJoinRequest(context.Background(), 1, 2, "husband", nil, nil)
	require.NoError(t, err, "SendNotification errors must be best-effort")
	require.NotNil(t, req)
	assert.Len(t, notif.snapshot(), 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_AcceptJoinRequest_NotifiesWithAcceptType_UsesNotificationChannels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		nil, // skip prize path
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	requestID, fromUserID, toUserID := uint64(1), uint64(10), uint64(20)
	now := time.Now()

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(requestID, fromUserID, toUserID, 0, "brother", "hi", now, now))
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

	expectUserBasicInfo(mock, fromUserID, "F10", "From")
	expectUserBasicInfo(mock, toUserID, "T20", "To")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_accept_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("accepted by [reciever-name]"))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_accept_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("you accepted [sender-name]"))
	expectJoinNotificationChannels(mock, fromUserID, "09120000001", "from@example.com", false, false)
	expectJoinNotificationChannels(mock, toUserID, "09120000002", "to@example.com", false, false)

	require.NoError(t, svc.AcceptJoinRequest(context.Background(), requestID, toUserID))

	calls := notif.snapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "dynasty_join_request_accept", calls[0].NotificationType)
	assert.Equal(t, fromUserID, calls[0].UserID)
	assert.Equal(t, "پیوستن به خاندان تأیید شد", calls[0].Title)
	assert.Equal(t, "From", calls[0].Data["DynastyName"])
	assert.False(t, calls[0].SendSMS)
	assert.False(t, calls[0].SendEmail)
	assert.Equal(t, "dynasty_join_request_accept", calls[1].NotificationType)
	assert.Equal(t, toUserID, calls[1].UserID)
	assert.False(t, calls[1].SendSMS)
	assert.False(t, calls[1].SendEmail)
	assert.NoError(t, calls[0].CtxErr)
	assert.NoError(t, calls[1].CtxErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_AcceptJoinRequest_EmptyTemplates_UsesFallbackMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		nil,
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	requestID, fromUserID, toUserID := uint64(1), uint64(10), uint64(20)
	now := time.Now()

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(requestID, fromUserID, toUserID, 0, "brother", "hi", now, now))
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

	expectUserBasicInfo(mock, fromUserID, "F10", "From")
	expectUserBasicInfo(mock, toUserID, "T20", "To")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_accept_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_accept_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	expectJoinNotificationChannels(mock, fromUserID, "09120000001", "from@example.com", true, true)
	expectJoinNotificationChannels(mock, toUserID, "09120000002", "to@example.com", true, true)

	require.NoError(t, svc.AcceptJoinRequest(context.Background(), requestID, toUserID))

	calls := notif.snapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "dynasty_join_request_accept", calls[0].NotificationType)
	assert.Contains(t, calls[0].Message, "T20")
	assert.True(t, calls[0].SendSMS)
	assert.True(t, calls[0].SendEmail)
	assert.Equal(t, toUserID, calls[1].UserID)
	assert.Contains(t, calls[1].Message, "F10")
	assert.True(t, calls[1].SendSMS)
	assert.True(t, calls[1].SendEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_RejectJoinRequest_UsesDefaultTemplatesWhenDBEmpty_UsesNotificationChannels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	requestID, fromUserID, toUserID := uint64(3), uint64(11), uint64(22)
	now := time.Now()

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(requestID, fromUserID, toUserID, 0, "sister", nil, now, now))
	mock.ExpectExec("UPDATE join_requests SET status").
		WithArgs(-1, requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	expectUserBasicInfo(mock, fromUserID, "REQ11", "Requester")
	expectUserBasicInfo(mock, toUserID, "RCV22", "Rejector")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_reject_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_reject_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow(""))
	expectJoinNotificationChannels(mock, fromUserID, "", "", false, false)
	expectJoinNotificationChannels(mock, toUserID, "", "", false, false)

	require.NoError(t, svc.RejectJoinRequest(context.Background(), requestID, toUserID))

	calls := notif.snapshot()
	require.Len(t, calls, 2)
	assert.Equal(t, "dynasty_join_request_reject", calls[0].NotificationType)
	assert.Equal(t, fromUserID, calls[0].UserID)
	assert.Contains(t, calls[0].Message, "RCV22")
	assert.Equal(t, "Requester", calls[0].Data["DynastyName"])
	assert.False(t, calls[0].SendSMS)
	assert.False(t, calls[0].SendEmail)
	assert.NoError(t, calls[0].CtxErr)

	assert.Equal(t, "dynasty_join_request_reject", calls[1].NotificationType)
	assert.Equal(t, toUserID, calls[1].UserID)
	assert.Contains(t, calls[1].Message, "REQ11")
	assert.False(t, calls[1].SendSMS)
	assert.False(t, calls[1].SendEmail)
	assert.NoError(t, calls[1].CtxErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_RejectJoinRequest_SendNotificationFailure_DoesNotFailMainFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	notif := &recordingNotificationPort{err: errors.New("notify failed")}
	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		notif,
		"",
	)

	requestID, fromUserID, toUserID := uint64(4), uint64(1), uint64(2)
	now := time.Now()
	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(requestID, fromUserID, toUserID, 0, "mother", nil, now, now))
	mock.ExpectExec("UPDATE join_requests SET status").
		WithArgs(-1, requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectUserBasicInfo(mock, fromUserID, "A", "A")
	expectUserBasicInfo(mock, toUserID, "B", "B")
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("requester_reject_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("rejected [reciever-code]"))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_reject_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("you rejected [sender-code]"))
	expectJoinNotificationChannels(mock, fromUserID, "09120000001", "a@example.com", true, false)
	expectJoinNotificationChannels(mock, toUserID, "09120000002", "b@example.com", false, true)

	require.NoError(t, svc.RejectJoinRequest(context.Background(), requestID, toUserID))
	calls := notif.snapshot()
	assert.Len(t, calls, 2)
	assert.True(t, calls[0].SendSMS)
	assert.False(t, calls[0].SendEmail)
	assert.False(t, calls[1].SendSMS)
	assert.True(t, calls[1].SendEmail)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_FormatRelationshipMessage_AndGetUserBasicInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		nil,
		"",
	)

	formatted := svc.FormatRelationshipMessage("{sender} -> {receiver} ({relationship}) @ {date}", "A", "B", "brother", "today")
	assert.Equal(t, "A -> B (brother) @ today", formatted)

	expectUserBasicInfo(mock, 9, "C9", "Name9")
	info, err := svc.GetUserBasicInfo(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "C9", info.Code)
	assert.Equal(t, models.UserBasic{ID: 9, Code: "C9", Name: "Name9"}, *info)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestService_NotificationPort_ContractAcceptsSMSEmailFlags(t *testing.T) {
	port := &recordingNotificationPort{}
	var _ service.NotificationPort = port

	err := port.SendNotification(context.Background(), 1, "custom", "T", "M", map[string]string{"k": "v"}, true, true)
	require.NoError(t, err)
	calls := port.snapshot()
	require.Len(t, calls, 1)
	assert.True(t, calls[0].SendSMS)
	assert.True(t, calls[0].SendEmail)
}
