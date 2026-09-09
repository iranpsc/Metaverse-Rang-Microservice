package service_test

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/repository"
	"metarang/notifications-service/internal/service"
	"metarang/notifications-service/tests/internal/testutil"
)

func TestNewEmailChannel_SendEmailNotImplemented(t *testing.T) {
	ch := service.NewEmailChannel()
	_, err := ch.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
	assert.ErrorIs(t, err, errs.ErrNotImplemented)
}

func TestNewSMSService_NilChannelSendOTP(t *testing.T) {
	svc := service.NewSMSService(nil)
	_, err := svc.SendOTP(context.Background(), models.OTPPayload{Phone: "09120000000", Code: "1234"})
	assert.ErrorIs(t, err, errs.ErrNotImplemented)
}

func TestNotificationService_SendNotification_NilChannelsSkipDelivery(t *testing.T) {
	ctx := context.Background()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewNotificationService(repository.NewNotificationRepository(db), nil, nil)
	expectCreateNotification(mock)

	result, err := svc.SendNotification(ctx, service.SendNotificationInput{
		UserID: 123, Type: "system", Title: "Test", Message: "Message",
		SendSMS: true, SMSPayload: &models.SMSPayload{Phone: "09120000000", Message: "hi"},
		SendEmail: true, EmailPayload: &models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"},
	})
	require.NoError(t, err)
	assert.True(t, result.Sent)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationService_SendNotification_SkipsNilPayloads(t *testing.T) {
	ctx := context.Background()

	t.Run("SendSMS true with nil payload resolves phone and sends", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		var gotPhone, gotMessage, gotTemplate string
		var gotTokens map[string]string
		svc := newNotificationService(db, &testutil.MockSMSChannel{
			SendSMSFunc: func(_ context.Context, payload models.SMSPayload) (string, error) {
				gotPhone = payload.Phone
				gotMessage = payload.Message
				gotTemplate = payload.Template
				gotTokens = payload.Tokens
				return "sms", nil
			},
		}, &testutil.MockEmailChannel{})
		expectCreateNotification(mock)
		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email"}).AddRow("09120000000", "u@example.com"))

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Join request",
			SendSMS: true, SMSPayload: nil,
			SMSTemplate: "buy-land-metarang",
			SMSTokens:   map[string]string{"token": "p1", "token20": "buyer"},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.Equal(t, "09120000000", gotPhone)
		assert.Equal(t, "Join request", gotMessage)
		assert.Equal(t, "buy-land-metarang", gotTemplate)
		assert.Equal(t, "p1", gotTokens["token"])
		assert.Equal(t, "buyer", gotTokens["token20"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SendEmail true with nil payload resolves email and sends", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		var gotTo, gotSubject, gotBody string
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{
			SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
				gotTo = payload.To
				gotSubject = payload.Subject
				gotBody = payload.Body
				return "email", nil
			},
		})
		expectCreateNotification(mock)
		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email"}).AddRow("09120000000", "u@example.com"))

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Dynasty", Message: "Join request",
			SendEmail: true, EmailPayload: nil,
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.Equal(t, "u@example.com", gotTo)
		assert.Equal(t, "Dynasty", gotSubject)
		assert.Equal(t, "Join request", gotBody)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing contact skips channel send", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		smsCalled := false
		svc := newNotificationService(db, &testutil.MockSMSChannel{
			SendSMSFunc: func(context.Context, models.SMSPayload) (string, error) {
				smsCalled = true
				return "sms", nil
			},
		}, &testutil.MockEmailChannel{})
		expectCreateNotification(mock)
		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(123)).
			WillReturnError(sql.ErrNoRows)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
			SendSMS: true,
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.False(t, smsCalled)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestNotificationService_SendNotification_UnimplementedChannelStillSent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := newNotificationService(db, &testutil.MockSMSChannel{
		SendSMSFunc: func(context.Context, models.SMSPayload) (string, error) {
			return "", errs.ErrNotImplemented
		},
	}, &testutil.MockEmailChannel{
		SendEmailFunc: func(context.Context, models.EmailPayload) (string, error) {
			return "", errs.ErrNotImplemented
		},
	})
	expectCreateNotification(mock)

	result, err := svc.SendNotification(context.Background(), service.SendNotificationInput{
		UserID: 123, Type: "system", Title: "Test", Message: "Message",
		SendSMS: true, SMSPayload: &models.SMSPayload{Phone: "09120000000", Message: "hi"},
		SendEmail: true, EmailPayload: &models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"},
	})
	require.NoError(t, err)
	assert.True(t, result.Sent)
	assert.NoError(t, mock.ExpectationsWereMet())
}
