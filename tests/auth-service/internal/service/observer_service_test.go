package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/pubsub"
	"metarang/auth-service/internal/service"
	notificationspb "metarang/shared/pb/notifications"
)

type fakePublisher struct {
	calls []bool
}

func (f *fakePublisher) PublishUserStatusChanged(_ context.Context, _ uint64, online bool) error {
	f.calls = append(f.calls, online)
	return nil
}
func (f *fakePublisher) Close() error { return nil }

var _ pubsub.RedisPublisher = (*fakePublisher)(nil)

func TestObserverService_LogoutCreatedScore(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepository(map[uint64]*models.User{
		1: {ID: 1, Email: "a@x.com", IP: "1.1.1.1", Score: 1},
	})
	users.updateFunc = func(_ context.Context, u *models.User) error {
		users.users[u.ID] = u
		return nil
	}
	act := newFakeActivityRepository()
	act.latestActivity[1] = &models.UserActivity{
		ID: 1, UserID: 1, Start: time.Now().Add(-2 * time.Hour),
	}
	act.userLogs[1] = &models.UserLog{UserID: 1, Score: 0}
	pub := &fakePublisher{}

	svc := service.NewObserverService(users, act, pub)

	t.Run("logout", func(t *testing.T) {
		user := users.users[1]
		if err := svc.OnUserLogout(ctx, user, "1.1.1.1", "ua"); err != nil {
			t.Fatal(err)
		}
		if len(pub.calls) == 0 || pub.calls[len(pub.calls)-1] {
			t.Fatalf("expected offline publish, calls=%v", pub.calls)
		}
	})

	t.Run("created", func(t *testing.T) {
		user := &models.User{ID: 2, Email: "b@x.com", IP: "2.2.2.2"}
		users.users[2] = user
		if err := svc.OnUserCreated(ctx, user); err != nil {
			t.Fatal(err)
		}
		if !user.EmailVerifiedAt.Valid && users.users[2].EmailVerifiedAt.Valid {
			// marked via repo
		}
		if act.userLogs[2] == nil {
			t.Fatal("expected user log")
		}
	})

	t.Run("hour reached and score", func(t *testing.T) {
		user := users.users[1]
		if err := svc.OnHourReached(ctx, user); err != nil {
			t.Fatal(err)
		}
		if err := svc.CalculateScore(ctx, user); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("login with publisher", func(t *testing.T) {
		user := users.users[1]
		user.Phone = sql.NullString{String: "0912", Valid: true}
		user.PhoneVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		if err := svc.OnUserLogin(ctx, user, "1.1.1.1", "ua"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestObserverService_LoginNotificationPayload(t *testing.T) {
	ctx := context.Background()
	const loginIP = "203.0.113.10"

	newLoginUser := func(verifiedPhone bool) *models.User {
		u := &models.User{
			ID:    7,
			Email: "login@x.com",
			Code:  "HM-77",
			IP:    loginIP,
		}
		if verifiedPhone {
			u.Phone = sql.NullString{String: "09121234567", Valid: true}
			u.PhoneVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		}
		return u
	}

	newSvc := func(user *models.User, notifs map[string]bool, client notificationspb.NotificationServiceClient) service.ObserverService {
		users := newFakeUserRepository(map[uint64]*models.User{user.ID: user})
		act := newFakeActivityRepository()
		settings := newTrackingSettingsRepository()
		settings.created = &models.Settings{UserID: user.ID, Notifications: notifs}
		return service.NewObserverServiceWithSettings(users, settings, act, &fakePublisher{}, client)
	}

	assertJalaliFields := func(t *testing.T, data map[string]string) {
		t.Helper()
		if data["login_date"] == "" {
			t.Fatal("expected login_date")
		}
		if data["login_date"] != service.FormatJalaliDate(time.Now()) {
			t.Errorf("login_date=%q want today's jalali date", data["login_date"])
		}
		if data["login_time"] == "" {
			t.Fatal("expected login_time")
		}
	}

	t.Run("sms and email include user_code login stamp and kavenegar tokens", func(t *testing.T) {
		user := newLoginUser(true)
		client := newFakeNotificationServiceClient()
		svc := newSvc(user, map[string]bool{
			"login_verification_sms":   true,
			"login_verification_email": true,
		}, client)

		if err := svc.OnUserLogin(ctx, user, loginIP, "ua"); err != nil {
			t.Fatal(err)
		}
		if client.lastRequest == nil {
			t.Fatal("expected SendNotification")
		}
		req := client.lastRequest
		if req.UserId != user.ID || req.Type != "login" {
			t.Fatalf("unexpected identity: user=%d type=%q", req.UserId, req.Type)
		}
		if req.Data["user_code"] != "HM-77" {
			t.Errorf("user_code=%q", req.Data["user_code"])
		}
		if req.Data["ip"] != loginIP {
			t.Errorf("ip=%q", req.Data["ip"])
		}
		assertJalaliFields(t, req.Data)
		if !req.SendSms || !req.SendEmail {
			t.Fatalf("SendSms=%v SendEmail=%v", req.SendSms, req.SendEmail)
		}
		if req.SmsTemplate != "login" {
			t.Errorf("SmsTemplate=%q", req.SmsTemplate)
		}
		if req.SmsTokens["token"] != loginIP {
			t.Errorf("SmsTokens=%v", req.SmsTokens)
		}
	})

	t.Run("email only omits sms template and tokens", func(t *testing.T) {
		user := newLoginUser(true)
		client := newFakeNotificationServiceClient()
		svc := newSvc(user, map[string]bool{
			"login_verification_sms":   false,
			"login_verification_email": true,
		}, client)

		if err := svc.OnUserLogin(ctx, user, loginIP, "ua"); err != nil {
			t.Fatal(err)
		}
		req := client.lastRequest
		if req == nil {
			t.Fatal("expected SendNotification")
		}
		if req.Data["user_code"] != "HM-77" {
			t.Errorf("user_code=%q", req.Data["user_code"])
		}
		assertJalaliFields(t, req.Data)
		if req.SendSms || !req.SendEmail {
			t.Fatalf("SendSms=%v SendEmail=%v", req.SendSms, req.SendEmail)
		}
		if req.SmsTemplate != "" {
			t.Errorf("expected empty SmsTemplate, got %q", req.SmsTemplate)
		}
		if len(req.SmsTokens) != 0 {
			t.Errorf("expected no SmsTokens, got %v", req.SmsTokens)
		}
	})

	t.Run("unverified phone disables sms even when setting enabled", func(t *testing.T) {
		user := newLoginUser(false)
		client := newFakeNotificationServiceClient()
		svc := newSvc(user, map[string]bool{
			"login_verification_sms":   true,
			"login_verification_email": true,
		}, client)

		if err := svc.OnUserLogin(ctx, user, loginIP, "ua"); err != nil {
			t.Fatal(err)
		}
		req := client.lastRequest
		if req == nil {
			t.Fatal("expected SendNotification")
		}
		if req.SendSms {
			t.Fatal("expected SendSms false without verified phone")
		}
		if req.SmsTemplate != "" || len(req.SmsTokens) != 0 {
			t.Fatalf("sms template/tokens should be empty: template=%q tokens=%v", req.SmsTemplate, req.SmsTokens)
		}
		if !req.SendEmail {
			t.Fatal("expected SendEmail true")
		}
	})

	t.Run("nil notification client still completes login", func(t *testing.T) {
		user := newLoginUser(true)
		svc := newSvc(user, map[string]bool{
			"login_verification_sms":   true,
			"login_verification_email": true,
		}, nil)
		if err := svc.OnUserLogin(ctx, user, loginIP, "ua"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("notification send error does not fail login", func(t *testing.T) {
		user := newLoginUser(true)
		client := newFakeNotificationServiceClient()
		client.err = errors.New("notification unavailable")
		svc := newSvc(user, map[string]bool{
			"login_verification_sms":   true,
			"login_verification_email": true,
		}, client)

		if err := svc.OnUserLogin(ctx, user, loginIP, "ua"); err != nil {
			t.Fatalf("login should succeed, got %v", err)
		}
		if client.lastRequest == nil {
			t.Fatal("expected SendNotification to be attempted")
		}
	})
}

