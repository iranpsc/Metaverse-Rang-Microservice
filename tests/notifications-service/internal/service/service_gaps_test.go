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
		mock.ExpectQuery(`SELECT phone, email, name, code FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email", "name", "code"}).AddRow("09120000000", "u@example.com", "User", "U1"))

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
		mock.ExpectQuery(`SELECT phone, email, name, code FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email", "name", "code"}).AddRow("09120000000", "u@example.com", "User", "U1"))

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
		mock.ExpectQuery(`SELECT phone, email, name, code FROM users WHERE id = \? LIMIT 1`).
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

	t.Run("BuyFeatureNotification renders feature_purchase HTML email", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		var gotHTML, gotSubject, gotTo string
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{
			SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
				gotTo = payload.To
				gotSubject = payload.Subject
				gotHTML = payload.HTMLBody
				return "email", nil
			},
		})
		expectCreateNotification(mock)
		mock.ExpectQuery(`SELECT phone, email, name, code FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email", "name", "code"}).
				AddRow("09120000000", "buyer@example.com", "علی", "HM-1"))

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 42, Type: "BuyFeatureNotification", Title: "خریداری ملک",
			Message:   "از حساب شما برداشت شد",
			SendEmail: true,
			Data: map[string]string{
				"OwnerCode":          "HM-1",
				"FeatureID":          "VOD-100",
				"FeatureArea":        "120.50",
				"FeatureApplication": "مسکونی",
				"FeatureDensity":     "3",
				"PriceIRR":           "1000000",
				"PricePSC":           "10",
				"PriceIRRLabel":      "IRR",
				"PricePSCLabel":      "PSC",
			},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.Equal(t, "buyer@example.com", gotTo)
		assert.Equal(t, "خریداری ملک", gotSubject)
		assert.Contains(t, gotHTML, "ثبت موفق فضای VOD")
		assert.Contains(t, gotHTML, "VOD-100")
		assert.Contains(t, gotHTML, "مسکونی")
		assert.Contains(t, gotHTML, "1000000")
		assert.NotContains(t, gotHTML, `<div dir="rtl"`)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("BuyFeatureNotification RGB purchase renders PaidAsset HTML not PSC fallback", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		var gotHTML, gotBody string
		svc := newNotificationService(db, &testutil.MockSMSChannel{}, &testutil.MockEmailChannel{
			SendEmailFunc: func(_ context.Context, payload models.EmailPayload) (string, error) {
				gotHTML = payload.HTMLBody
				gotBody = payload.Body
				return "email", nil
			},
		})
		expectCreateNotification(mock)
		mock.ExpectQuery(`SELECT phone, email, name, code FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email", "name", "code"}).
				AddRow("09120000000", "buyer@example.com", "علی", "HM-1"))

		result, err := svc.SendNotification(ctx, service.SendNotificationInput{
			UserID: 42, Type: "BuyFeatureNotification", Title: "خریداری ملک",
			Message:   "1000 لیتر رنگ قرمز برداشت شد",
			SendEmail: true,
			Data: map[string]string{
				"OwnerCode":          "HM-1",
				"FeatureID":          "VOD-200",
				"FeatureArea":        "80.00",
				"FeatureApplication": "تجاری",
				"PaidAssetLabel":     "رنگ قرمز",
				"PaidAssetAmount":    "1000",
				"purchase_type":      "rgb",
			},
		})
		require.NoError(t, err)
		assert.True(t, result.Sent)
		assert.Equal(t, "1000 لیتر رنگ قرمز برداشت شد", gotBody)
		assert.Contains(t, gotHTML, "ثبت موفق فضای VOD")
		assert.Contains(t, gotHTML, "VOD-200")
		assert.Contains(t, gotHTML, "رنگ قرمز")
		assert.Contains(t, gotHTML, "1000")
		assert.NotContains(t, gotHTML, "مبلغ پرداختی (PSC)")
		assert.NotContains(t, gotHTML, "مبلغ پرداختی (IRR)")
		assert.NotContains(t, gotHTML, `<div dir="rtl"`)
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
