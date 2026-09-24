package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/repository"
	"metarang/notifications-service/internal/service"
	"metarang/notifications-service/tests/internal/testutil"
)

func newNotificationService(db *sql.DB, sms *testutil.MockSMSChannel, email *testutil.MockEmailChannel) service.NotificationService {
	repo := repository.NewNotificationRepository(db)
	return service.NewNotificationService(repo, sms, email)
}

func expectCreateNotification(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`INSERT INTO notifications`).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestNotificationService_SendNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("successful notification without SMS or Email", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test Title", Message: "Test Message",
			Data: map[string]string{"key": "value"},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.NotZero(t, result.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("successful notification with SMS", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		smsCalled := false
		svc := newNotificationService(db, &testutil.MockSMSChannel{
			SendSMSFunc: func(_ context.Context, payload models.SMSPayload) (string, error) {
				smsCalled = true
				assert.Equal(t, "+1234567890", payload.Phone)
				return "sms-id-123", nil
			},
		}, &testutil.MockEmailChannel{})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
			SendSMS: true, SMSPayload: &models.SMSPayload{Phone: "+1234567890", Message: "Test SMS"},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.True(t, smsCalled)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("successful notification with Email", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		emailCalled := false
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{
			SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
				emailCalled = true
				assert.Equal(t, "test@example.com", payload.To)
				return "email-id-123", nil
			},
		})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
			SendEmail: true, EmailPayload: &models.EmailPayload{To: "test@example.com", Subject: "Sub", Body: "Body"},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.True(t, emailCalled)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("repository error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})
		mock.ExpectExec(`INSERT INTO notifications`).WillReturnError(errors.New("db error"))

		_, err = svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
		})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SMS send failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := newNotificationService(db, &testutil.MockSMSChannel{
			SendSMSFunc: func(context.Context, models.SMSPayload) (string, error) {
				return "", errors.New("sms failed")
			},
		}, &testutil.MockEmailChannel{})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
			SendSMS: true, SMSPayload: &models.SMSPayload{Phone: "+1234567890", Message: "Test SMS"},
		})
		assert.Error(t, err)
		assert.False(t, result.Sent)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("email send failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{
			SendEmailFunc: func(context.Context, models.EmailPayload) (string, error) {
				return "", errors.New("email failed")
			},
		})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "system", Title: "Test", Message: "Message",
			SendEmail: true, EmailPayload: &models.EmailPayload{To: "test@example.com", Subject: "Sub", Body: "Body"},
		})
		assert.Error(t, err)
		assert.False(t, result.Sent)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SMS failure still sends email", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		emailCalled := false
		svc := newNotificationService(db, &testutil.MockSMSChannel{
			SendSMSFunc: func(context.Context, models.SMSPayload) (string, error) {
				return "", errors.New("kavenegar API error: APIError[416]")
			},
		}, &testutil.MockEmailChannel{
			SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
				emailCalled = true
				assert.Equal(t, "buyer@example.com", payload.To)
				return "smtp", nil
			},
		})
		expectCreateNotification(mock)

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 123, Type: "BuyFeatureNotification", Title: "خریداری ملک", Message: "Message",
			SendSMS: true, SMSPayload: &models.SMSPayload{Phone: "+1234567890", Message: "Test SMS"},
			SendEmail: true, EmailPayload: &models.EmailPayload{To: "buyer@example.com", Subject: "Sub", Body: "Body"},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.True(t, emailCalled)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestNotificationService_GetNotifications(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})
	ctx := context.Background()

	mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
		WithArgs("App\\User", uint64(123)).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
		WithArgs("App\\User", uint64(123), 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
			AddRow("id-1", `{"type":"system","title":"Test","message":"Message","data":{}}`, nil, time.Now(), time.Now()))

	notifications, total, err := svc.GetNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Len(t, notifications, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationService_GetNotificationByID(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})

		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
			WithArgs("notif-1", "App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
				AddRow("notif-1", `{"type":"system","title":"Test","message":"Message","data":{}}`, nil, time.Now(), time.Now()))

		notif, err := svc.GetNotificationByID(ctx, "notif-1", 123)
		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, "notif-1", notif.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})

		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
			WithArgs("missing", "App\\User", uint64(123)).
			WillReturnError(sql.ErrNoRows)

		_, err = svc.GetNotificationByID(ctx, "missing", 123)
		assert.ErrorIs(t, err, errs.ErrNotificationNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("repository error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})

		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
			WithArgs("broken", "App\\User", uint64(123)).
			WillReturnError(errors.New("db unavailable"))

		_, err = svc.GetNotificationByID(ctx, "broken", 123)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, errs.ErrNotificationNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestNotificationService_MarkAsRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})
	mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE id = \? AND notifiable_type = \? AND notifiable_id = \?`).
		WithArgs("notif-1", "App\\User", uint64(123)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = svc.MarkAsRead(context.Background(), "notif-1", 123)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{})
	mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE notifiable_type = \? AND notifiable_id = \? AND read_at IS NULL`).
		WithArgs("App\\User", uint64(123)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	err = svc.MarkAllAsRead(context.Background(), 123)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEmailService_SendEmail(t *testing.T) {
	svc := service.NewEmailService(&testutil.MockEmailChannel{
		SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
			assert.Equal(t, "a@b.com", payload.To)
			return "msg-1", nil
		},
	})
	id, err := svc.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
	require.NoError(t, err)
	assert.Equal(t, "msg-1", id)
}

func TestNewSMSService_NilChannelUsesNoop(t *testing.T) {
	svc := service.NewSMSService(nil)
	_, err := svc.SendSMS(context.Background(), models.SMSPayload{Phone: "09120000000", Message: "hi"})
	assert.ErrorIs(t, err, errs.ErrNotImplemented)
}

func TestSMSService_SendSMSAndOTP(t *testing.T) {
	sms := &testutil.MockSMSChannel{
		SendSMSFunc: func(_ context.Context, payload models.SMSPayload) (string, error) {
			assert.Equal(t, "09120000000", payload.Phone)
			return "sms-1", nil
		},
		SendOTPFunc: func(_ context.Context, payload models.OTPPayload) (string, error) {
			assert.Equal(t, "1234", payload.Code)
			return "otp-1", nil
		},
	}
	svc := service.NewSMSService(sms)

	id, err := svc.SendSMS(context.Background(), models.SMSPayload{Phone: "09120000000", Message: "hi"})
	require.NoError(t, err)
	assert.Equal(t, "sms-1", id)

	otpID, err := svc.SendOTP(context.Background(), models.OTPPayload{Phone: "09120000000", Code: "1234"})
	require.NoError(t, err)
	assert.Equal(t, "otp-1", otpID)
}

func TestNewSMSChannel(t *testing.T) {
	t.Run("noop when provider empty", func(t *testing.T) {
		ch := service.NewSMSChannel(service.SMSChannelConfig{})
		_, err := ch.SendSMS(context.Background(), models.SMSPayload{Phone: "1", Message: "m"})
		assert.ErrorIs(t, err, errs.ErrNotImplemented)
	})

	t.Run("noop when kavenegar without api key", func(t *testing.T) {
		ch := service.NewSMSChannel(service.SMSChannelConfig{Provider: "kavenegar"})
		_, err := ch.SendOTP(context.Background(), models.OTPPayload{Phone: "1", Code: "2"})
		assert.ErrorIs(t, err, errs.ErrNotImplemented)
	})

	t.Run("unknown provider uses noop", func(t *testing.T) {
		ch := service.NewSMSChannel(service.SMSChannelConfig{Provider: "unknown"})
		_, err := ch.SendSMS(context.Background(), models.SMSPayload{Phone: "1", Message: "m"})
		assert.ErrorIs(t, err, errs.ErrNotImplemented)
	})
}

func TestNewEmailService_NoopChannel(t *testing.T) {
	svc := service.NewEmailService(nil)
	_, err := svc.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
	assert.ErrorIs(t, err, errs.ErrNotImplemented)
}
