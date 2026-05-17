package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
	notificationspb "metargb/shared/pb/notifications"
)

func TestRequestAccountSecurityCreatesAndDispatchesOTP(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{Valid: false},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	if err := svc.RequestAccountSecurity(ctx, 1, 15, " 09123456789 "); err != nil {
		t.Fatalf("RequestAccountSecurity returned error: %v", err)
	}

	security := accountRepo.records[1]
	if security == nil {
		t.Fatalf("expected account security record to be created")
	}
	if security.Unlocked {
		t.Errorf("expected security to remain locked")
	}
	if security.Length != 15*60 {
		t.Errorf("expected length 900, got %d", security.Length)
	}
	if security.Until.Valid {
		t.Errorf("expected until to be cleared")
	}

	otp := accountRepo.otps[security.ID]
	if otp == nil {
		t.Fatalf("expected otp to be stored")
	}

	if smsClient.lastRequest == nil {
		t.Fatalf("expected SMS client to receive request")
	}
	if smsClient.lastRequest.Phone != "09123456789" {
		t.Errorf("expected trimmed phone, got %q", smsClient.lastRequest.Phone)
	}
	if smsClient.lastRequest.Reason != "verify" {
		t.Errorf("expected reason 'verify', got %q", smsClient.lastRequest.Reason)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otp.Code), []byte(smsClient.lastRequest.Code)); err != nil {
		t.Errorf("stored otp hash does not match dispatched code: %v", err)
	}

	updatedUser := users[1]
	if !updatedUser.Phone.Valid || updatedUser.Phone.String != "09123456789" {
		t.Errorf("expected user phone updated, got %v", updatedUser.Phone)
	}
	if updatedUser.PhoneVerifiedAt.Valid {
		t.Errorf("phone should remain unverified until verification step")
	}

	if accountRepo.createCount != 1 {
		t.Errorf("expected create count 1, got %d", accountRepo.createCount)
	}
	if accountRepo.updateCount != 0 {
		t.Errorf("expected update count 0 for new record, got %d", accountRepo.updateCount)
	}
}

func TestRequestAccountSecurityUpdatesExistingRecord(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09101234567", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	existing := &models.AccountSecurity{
		ID:       42,
		UserID:   1,
		Unlocked: true,
		Until:    sql.NullInt64{Int64: time.Now().Unix() + 300, Valid: true},
		Length:   300,
	}
	accountRepo.records[1] = existing

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	if err := svc.RequestAccountSecurity(ctx, 1, 20, ""); err != nil {
		t.Fatalf("RequestAccountSecurity returned error: %v", err)
	}

	security := accountRepo.records[1]
	if security.Unlocked {
		t.Errorf("expected security to be reset to locked")
	}
	if security.Until.Valid {
		t.Errorf("expected until to be cleared")
	}
	if security.Length != 20*60 {
		t.Errorf("expected updated length, got %d", security.Length)
	}

	if accountRepo.createCount != 0 {
		t.Errorf("expected no new create, got %d", accountRepo.createCount)
	}
	if accountRepo.updateCount != 1 {
		t.Errorf("expected single update, got %d", accountRepo.updateCount)
	}
}

func TestRequestAccountSecurityValidations(t *testing.T) {
	ctx := context.Background()
	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{Valid: false},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
		2: {
			ID:              2,
			Phone:           sql.NullString{String: "09123456789", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	t.Run("invalid duration", func(t *testing.T) {
		err := svc.RequestAccountSecurity(ctx, 1, 3, "09111111111")
		if !errors.Is(err, ErrInvalidUnlockDuration) {
			t.Fatalf("expected ErrInvalidUnlockDuration, got %v", err)
		}
	})

	t.Run("missing phone", func(t *testing.T) {
		err := svc.RequestAccountSecurity(ctx, 1, 10, "")
		if !errors.Is(err, ErrPhoneRequired) {
			t.Fatalf("expected ErrPhoneRequired, got %v", err)
		}
	})

	t.Run("duplicate phone", func(t *testing.T) {
		err := svc.RequestAccountSecurity(ctx, 1, 10, "09123456789")
		if !errors.Is(err, ErrPhoneAlreadyTaken) {
			t.Fatalf("expected ErrPhoneAlreadyTaken, got %v", err)
		}
	})
}

func TestRequestAccountSecurityNotificationError(t *testing.T) {
	ctx := context.Background()
	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{Valid: false},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{err: errors.New("dispatch failure")}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	err := svc.RequestAccountSecurity(ctx, 1, 15, "09122223333")
	if err == nil || err.Error() == "" {
		t.Fatalf("expected wrapped notification error, got %v", err)
	}
}

func TestVerifyAccountSecuritySuccess(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09100000000", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	security := &models.AccountSecurity{
		ID:       10,
		UserID:   1,
		Unlocked: false,
		Length:   600,
	}
	accountRepo.records[1] = security

	plainCode := "654321"
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainCode), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash otp: %v", err)
	}
	accountRepo.otps[security.ID] = &models.Otp{
		ID:           99,
		UserID:       1,
		VerifiableID: security.ID,
		Code:         string(hashed),
	}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	err = svc.VerifyAccountSecurity(ctx, 1, plainCode, " 127.0.0.1 ", " Mozilla/5.0 ")
	if err != nil {
		t.Fatalf("VerifyAccountSecurity returned error: %v", err)
	}

	updatedSecurity := accountRepo.records[1]
	if !updatedSecurity.Unlocked {
		t.Fatalf("expected account security unlocked")
	}
	if !updatedSecurity.Until.Valid {
		t.Fatalf("expected unlock window to be set")
	}
	if updatedSecurity.Until.Int64 < time.Now().Unix() {
		t.Fatalf("expected unlock window in the future, got %d", updatedSecurity.Until.Int64)
	}

	if _, found := accountRepo.otps[security.ID]; found {
		t.Fatalf("expected otp to be deleted after verification")
	}

	updatedUser := users[1]
	if !updatedUser.PhoneVerifiedAt.Valid {
		t.Fatalf("expected phone to be marked verified")
	}

	if len(activityRepo.events) != 1 {
		t.Fatalf("expected one user event, got %d", len(activityRepo.events))
	}
	event := activityRepo.events[0]
	if event.Event != "غیر فعال سازی امنیت حساب کاربری" {
		t.Fatalf("unexpected event message: %q", event.Event)
	}
	if event.IP != "127.0.0.1" {
		t.Fatalf("expected trimmed IP, got %q", event.IP)
	}
	if event.Device != "Mozilla/5.0" {
		t.Fatalf("expected trimmed user agent, got %q", event.Device)
	}
}

func TestVerifyAccountSecurityFailures(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09100000000", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
		2: {
			ID:              2,
			Phone:           sql.NullString{String: "09111111111", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
		3: {
			ID:              3,
			Phone:           sql.NullString{String: "09122222222", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	security := &models.AccountSecurity{
		ID:       5,
		UserID:   1,
		Unlocked: false,
		Length:   300,
	}
	accountRepo.records[1] = security

	hashed, err := bcrypt.GenerateFromPassword([]byte("111111"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash otp: %v", err)
	}
	accountRepo.otps[security.ID] = &models.Otp{ID: 7, UserID: 1, VerifiableID: security.ID, Code: string(hashed)}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	t.Run("invalid code format - non-numeric", func(t *testing.T) {
		err := svc.VerifyAccountSecurity(ctx, 1, "abc123", "", "")
		if !errors.Is(err, ErrInvalidOTPCode) {
			t.Fatalf("expected ErrInvalidOTPCode, got %v", err)
		}
	})

	t.Run("invalid code format - wrong length", func(t *testing.T) {
		err := svc.VerifyAccountSecurity(ctx, 1, "12345", "", "")
		if !errors.Is(err, ErrInvalidOTPCode) {
			t.Fatalf("expected ErrInvalidOTPCode, got %v", err)
		}
	})

	t.Run("invalid code - wrong digits", func(t *testing.T) {
		err := svc.VerifyAccountSecurity(ctx, 1, "000000", "", "")
		if !errors.Is(err, ErrInvalidOTPCode) {
			t.Fatalf("expected ErrInvalidOTPCode, got %v", err)
		}
	})

	t.Run("missing security record", func(t *testing.T) {
		accountRepo.records = map[uint64]*models.AccountSecurity{}
		err := svc.VerifyAccountSecurity(ctx, 2, "111111", "", "")
		if !errors.Is(err, ErrAccountSecurityNotFound) {
			t.Fatalf("expected ErrAccountSecurityNotFound, got %v", err)
		}
	})

	t.Run("already unlocked", func(t *testing.T) {
		alreadyUnlockedSecurity := &models.AccountSecurity{
			ID:       10,
			UserID:   3,
			Unlocked: true,
			Until:    sql.NullInt64{Int64: time.Now().Unix() + 300, Valid: true},
			Length:   300,
		}
		accountRepo.records[3] = alreadyUnlockedSecurity

		err := svc.VerifyAccountSecurity(ctx, 3, "111111", "", "")
		if !errors.Is(err, ErrAccountSecurityAlreadyUnlocked) {
			t.Fatalf("expected ErrAccountSecurityAlreadyUnlocked, got %v", err)
		}
	})

	t.Run("missing OTP", func(t *testing.T) {
		securityNoOtp := &models.AccountSecurity{
			ID:       15,
			UserID:   2,
			Unlocked: false,
			Length:   300,
		}
		accountRepo.records[2] = securityNoOtp
		// No OTP in accountRepo.otps

		err := svc.VerifyAccountSecurity(ctx, 2, "123456", "", "")
		if !errors.Is(err, ErrAccountSecurityNotFound) {
			t.Fatalf("expected ErrAccountSecurityNotFound when OTP missing, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		security := &models.AccountSecurity{
			ID:       20,
			UserID:   999,
			Unlocked: false,
			Length:   300,
		}
		accountRepo.records[999] = security

		err := svc.VerifyAccountSecurity(ctx, 999, "123456", "", "")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestRequestAccountSecurityPhoneHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("phone optional when already verified", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:              1,
				Phone:           sql.NullString{String: "09123456789", Valid: true},
				PhoneVerifiedAt: sql.NullTime{Valid: true, Time: time.Now()},
			},
		}

		userRepo := newFakeUserRepository(users)
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		smsClient := &fakeSMSServiceClient{}

		svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

		err := svc.RequestAccountSecurity(ctx, 1, 15, "")
		if err != nil {
			t.Fatalf("RequestAccountSecurity should succeed without phone when already verified: %v", err)
		}

		if smsClient.lastRequest == nil {
			t.Fatalf("expected SMS to be sent to existing phone")
		}
		if smsClient.lastRequest.Phone != "09123456789" {
			t.Errorf("expected SMS to existing phone, got %q", smsClient.lastRequest.Phone)
		}
	})

	t.Run("phone trimmed correctly", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:              1,
				Phone:           sql.NullString{Valid: false},
				PhoneVerifiedAt: sql.NullTime{Valid: false},
			},
		}

		userRepo := newFakeUserRepository(users)
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		smsClient := &fakeSMSServiceClient{}

		svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

		err := svc.RequestAccountSecurity(ctx, 1, 15, "  09123456789  ")
		if err != nil {
			t.Fatalf("RequestAccountSecurity failed: %v", err)
		}

		if !users[1].Phone.Valid || users[1].Phone.String != "09123456789" {
			t.Errorf("expected phone to be trimmed, got %v", users[1].Phone)
		}
	})

	t.Run("invalid phone format", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:              1,
				Phone:           sql.NullString{Valid: false},
				PhoneVerifiedAt: sql.NullTime{Valid: false},
			},
		}

		userRepo := newFakeUserRepository(users)
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		smsClient := &fakeSMSServiceClient{}

		svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

		testCases := []struct {
			name  string
			phone string
		}{
			{"too short", "09123"},
			{"too long", "091234567890"},
			{"wrong prefix", "08123456789"},
			{"non-numeric", "0912345abc"},
			{"with spaces", "09 1234 5678"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := svc.RequestAccountSecurity(ctx, 1, 15, tc.phone)
				if !errors.Is(err, ErrInvalidPhoneFormat) {
					t.Fatalf("expected ErrInvalidPhoneFormat for %q, got %v", tc.phone, err)
				}
			})
		}
	})

	t.Run("duration boundary values", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:              1,
				Phone:           sql.NullString{String: "09123456789", Valid: true},
				PhoneVerifiedAt: sql.NullTime{Valid: true},
			},
		}

		userRepo := newFakeUserRepository(users)
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		smsClient := &fakeSMSServiceClient{}

		svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

		t.Run("minimum duration (5 minutes)", func(t *testing.T) {
			err := svc.RequestAccountSecurity(ctx, 1, 5, "")
			if err != nil {
				t.Fatalf("expected 5 minutes to be valid: %v", err)
			}
			if accountRepo.records[1].Length != 5*60 {
				t.Errorf("expected length 300, got %d", accountRepo.records[1].Length)
			}
		})

		t.Run("maximum duration (60 minutes)", func(t *testing.T) {
			err := svc.RequestAccountSecurity(ctx, 1, 60, "")
			if err != nil {
				t.Fatalf("expected 60 minutes to be valid: %v", err)
			}
			if accountRepo.records[1].Length != 60*60 {
				t.Errorf("expected length 3600, got %d", accountRepo.records[1].Length)
			}
		})

		t.Run("below minimum (4 minutes)", func(t *testing.T) {
			err := svc.RequestAccountSecurity(ctx, 1, 4, "")
			if !errors.Is(err, ErrInvalidUnlockDuration) {
				t.Fatalf("expected ErrInvalidUnlockDuration for 4 minutes, got %v", err)
			}
		})

		t.Run("above maximum (61 minutes)", func(t *testing.T) {
			err := svc.RequestAccountSecurity(ctx, 1, 61, "")
			if !errors.Is(err, ErrInvalidUnlockDuration) {
				t.Fatalf("expected ErrInvalidUnlockDuration for 61 minutes, got %v", err)
			}
		})
	})
}

func TestIsProductionEnv(t *testing.T) {
	tests := map[string]bool{
		"production": true,
		"prod":       true,
		"PRODUCTION": true,
		"local":      false,
		"staging":    false,
		"":           false,
	}

	for env, want := range tests {
		if got := IsProductionEnv(env); got != want {
			t.Errorf("IsProductionEnv(%q) = %v, want %v", env, got, want)
		}
	}
}

func TestRequestAccountSecurityVerificationRateLimit(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09123456789", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	cacheRepo := newFakeCacheRepository()
	smsClient := &fakeSMSServiceClient{}

	t.Run("disabled outside production", func(t *testing.T) {
		svc := NewAuthService(userRepo, nil, cacheRepo, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

		if err := svc.RequestAccountSecurity(ctx, 1, 15, ""); err != nil {
			t.Fatalf("first request failed: %v", err)
		}
		if err := svc.RequestAccountSecurity(ctx, 1, 15, ""); err != nil {
			t.Fatalf("second request should not be rate limited outside production: %v", err)
		}
	})

	t.Run("enabled in production", func(t *testing.T) {
		svc := NewAuthService(userRepo, nil, cacheRepo, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", true)

		if err := svc.RequestAccountSecurity(ctx, 1, 15, ""); err != nil {
			t.Fatalf("first request failed: %v", err)
		}
		err := svc.RequestAccountSecurity(ctx, 1, 15, "")
		if !errors.Is(err, ErrVerificationRequestRateLimited) {
			t.Fatalf("expected ErrVerificationRequestRateLimited, got %v", err)
		}
	})
}

func TestVerifyAccountSecurityEventLogging(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09100000000", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	security := &models.AccountSecurity{
		ID:       10,
		UserID:   1,
		Unlocked: false,
		Length:   600,
	}
	accountRepo.records[1] = security

	plainCode := "123456"
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainCode), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash otp: %v", err)
	}
	accountRepo.otps[security.ID] = &models.Otp{
		ID:           99,
		UserID:       1,
		VerifiableID: security.ID,
		Code:         string(hashed),
	}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	err = svc.VerifyAccountSecurity(ctx, 1, plainCode, "192.168.1.100", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	if err != nil {
		t.Fatalf("VerifyAccountSecurity returned error: %v", err)
	}

	if len(activityRepo.events) != 1 {
		t.Fatalf("expected one user event, got %d", len(activityRepo.events))
	}

	event := activityRepo.events[0]
	if event.UserID != 1 {
		t.Errorf("expected user ID 1, got %d", event.UserID)
	}
	if event.Event != "غیر فعال سازی امنیت حساب کاربری" {
		t.Errorf("expected Farsi event message, got %q", event.Event)
	}
	if event.IP != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %q", event.IP)
	}
	if event.Device != "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" {
		t.Errorf("expected full user agent, got %q", event.Device)
	}
	if event.Status != 1 {
		t.Errorf("expected status 1, got %d", event.Status)
	}
}

func TestVerifyAccountSecurityUnlockWindow(t *testing.T) {
	ctx := context.Background()

	users := map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09100000000", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: true},
		},
	}

	userRepo := newFakeUserRepository(users)
	accountRepo := newFakeAccountSecurityRepository()
	activityRepo := newFakeActivityRepository()
	smsClient := &fakeSMSServiceClient{}

	security := &models.AccountSecurity{
		ID:       10,
		UserID:   1,
		Unlocked: false,
		Length:   900, // 15 minutes
	}
	accountRepo.records[1] = security

	plainCode := "888888"
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainCode), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash otp: %v", err)
	}
	accountRepo.otps[security.ID] = &models.Otp{
		ID:           99,
		UserID:       1,
		VerifiableID: security.ID,
		Code:         string(hashed),
	}

	svc := NewAuthService(userRepo, nil, nil, accountRepo, activityRepo, nil, nil, smsClient, "", "", "", "", "", false)

	beforeTime := time.Now().Unix()
	err = svc.VerifyAccountSecurity(ctx, 1, plainCode, "", "")
	if err != nil {
		t.Fatalf("VerifyAccountSecurity returned error: %v", err)
	}
	afterTime := time.Now().Unix()

	updatedSecurity := accountRepo.records[1]
	if !updatedSecurity.Unlocked {
		t.Fatalf("expected account security to be unlocked")
	}
	if !updatedSecurity.Until.Valid {
		t.Fatalf("expected unlock window to be set")
	}

	expectedMin := beforeTime + 900
	expectedMax := afterTime + 900

	if updatedSecurity.Until.Int64 < expectedMin || updatedSecurity.Until.Int64 > expectedMax {
		t.Errorf("expected unlock window between %d and %d, got %d", expectedMin, expectedMax, updatedSecurity.Until.Int64)
	}
}

// --- test fakes ---

type fakeUserRepository struct {
	users map[uint64]*models.User
}

func newFakeUserRepository(users map[uint64]*models.User) *fakeUserRepository {
	return &fakeUserRepository{users: users}
}

func (f *fakeUserRepository) Create(context.Context, *models.User) error {
	panic("unexpected call to Create")
}

func (f *fakeUserRepository) FindByEmail(context.Context, string) (*models.User, error) {
	panic("unexpected call to FindByEmail")
}

func (f *fakeUserRepository) FindByID(_ context.Context, id uint64) (*models.User, error) {
	if user, ok := f.users[id]; ok {
		return user, nil
	}
	return nil, nil
}

func (f *fakeUserRepository) Update(context.Context, *models.User) error {
	panic("unexpected call to Update")
}

func (f *fakeUserRepository) UpdateLastSeen(_ context.Context, userID uint64) error {
	if _, ok := f.users[userID]; ok {
		return nil
	}
	return nil
}

func (f *fakeUserRepository) FindByCode(context.Context, string) (*models.User, error) {
	panic("unexpected call to FindByCode")
}

func (f *fakeUserRepository) GetSettings(context.Context, uint64) (*models.Settings, error) {
	panic("unexpected call to GetSettings")
}

func (f *fakeUserRepository) CreateSettings(context.Context, *models.Settings) error {
	panic("unexpected call to CreateSettings")
}

func (f *fakeUserRepository) GetKYC(context.Context, uint64) (*models.KYC, error) {
	panic("unexpected call to GetKYC")
}

func (f *fakeUserRepository) GetUnreadNotificationsCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetUnreadNotificationsCount")
}

func (f *fakeUserRepository) MarkEmailAsVerified(context.Context, uint64) error {
	panic("unexpected call to MarkEmailAsVerified")
}

func (f *fakeUserRepository) UpdatePhone(_ context.Context, userID uint64, phone string) error {
	if user, ok := f.users[userID]; ok {
		user.Phone = sql.NullString{String: phone, Valid: phone != ""}
		return nil
	}
	return fmt.Errorf("user %d not found", userID)
}

func (f *fakeUserRepository) MarkPhoneAsVerified(_ context.Context, userID uint64) error {
	if user, ok := f.users[userID]; ok {
		user.PhoneVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		return nil
	}
	return fmt.Errorf("user %d not found", userID)
}

func (f *fakeUserRepository) IsPhoneTaken(_ context.Context, phone string, excludeUserID uint64) (bool, error) {
	for id, user := range f.users {
		if id == excludeUserID {
			continue
		}
		if user.Phone.Valid && user.Phone.String == phone {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUserRepository) ListUsers(context.Context, string, string, int32, int32) ([]*repository.UserWithRelations, int32, error) {
	panic("unexpected call to ListUsers")
}

func (f *fakeUserRepository) GetFollowersCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetFollowersCount")
}

func (f *fakeUserRepository) GetFollowingCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetFollowingCount")
}

func (f *fakeUserRepository) GetLatestProfilePhotoURL(context.Context, uint64) (string, error) {
	panic("unexpected call to GetLatestProfilePhotoURL")
}

func (f *fakeUserRepository) GetAllProfilePhotoURLs(context.Context, uint64) ([]string, error) {
	panic("unexpected call to GetAllProfilePhotoURLs")
}

func (f *fakeUserRepository) GetUserLatestLevel(context.Context, uint64) (*repository.UserLevel, error) {
	panic("unexpected call to GetUserLatestLevel")
}

func (f *fakeUserRepository) GetLevelsBelowScore(context.Context, int32) ([]*repository.UserLevel, error) {
	panic("unexpected call to GetLevelsBelowScore")
}

func (f *fakeUserRepository) GetNextLevelScore(context.Context, int32) (int32, error) {
	panic("unexpected call to GetNextLevelScore")
}

func (f *fakeUserRepository) GetFeatureCounts(context.Context, uint64) (int32, int32, int32, error) {
	panic("unexpected call to GetFeatureCounts")
}

var _ repository.UserRepository = (*fakeUserRepository)(nil)

type fakeAccountSecurityRepository struct {
	nextID      uint64
	nextOtpID   uint64
	records     map[uint64]*models.AccountSecurity
	otps        map[uint64]*models.Otp
	createCount int
	updateCount int
}

func newFakeAccountSecurityRepository() *fakeAccountSecurityRepository {
	return &fakeAccountSecurityRepository{
		nextID:    100,
		nextOtpID: 200,
		records:   make(map[uint64]*models.AccountSecurity),
		otps:      make(map[uint64]*models.Otp),
	}
}

func (f *fakeAccountSecurityRepository) GetByUserID(_ context.Context, userID uint64) (*models.AccountSecurity, error) {
	if security, ok := f.records[userID]; ok {
		return security, nil
	}
	return nil, nil
}

func (f *fakeAccountSecurityRepository) Create(_ context.Context, security *models.AccountSecurity) error {
	f.createCount++
	if security.ID == 0 {
		security.ID = f.nextID
		f.nextID++
	}
	now := time.Now()
	security.CreatedAt = now
	security.UpdatedAt = now
	f.records[security.UserID] = security
	return nil
}

func (f *fakeAccountSecurityRepository) Update(_ context.Context, security *models.AccountSecurity) error {
	f.updateCount++
	security.UpdatedAt = time.Now()
	f.records[security.UserID] = security
	return nil
}

func (f *fakeAccountSecurityRepository) GetOtpByAccountSecurity(_ context.Context, accountSecurityID uint64) (*models.Otp, error) {
	if otp, ok := f.otps[accountSecurityID]; ok {
		return otp, nil
	}
	return nil, nil
}

func (f *fakeAccountSecurityRepository) UpsertOtp(_ context.Context, otp *models.Otp) error {
	if otp.ID == 0 {
		otp.ID = f.nextOtpID
		f.nextOtpID++
	}
	now := time.Now()
	otp.CreatedAt = now
	otp.UpdatedAt = now
	otp.VerifiableType = "App\\Models\\AccountSecurity"
	f.otps[otp.VerifiableID] = otp
	return nil
}

func (f *fakeAccountSecurityRepository) DeleteOtp(_ context.Context, otpID uint64) error {
	for key, otp := range f.otps {
		if otp.ID == otpID {
			delete(f.otps, key)
			return nil
		}
	}
	return nil
}

var _ repository.AccountSecurityRepository = (*fakeAccountSecurityRepository)(nil)

type fakeActivityRepository struct {
	events []*models.UserEvent
}

func newFakeActivityRepository() *fakeActivityRepository {
	return &fakeActivityRepository{}
}

func (f *fakeActivityRepository) CreateUserEvent(_ context.Context, event *models.UserEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeActivityRepository) CreateActivity(context.Context, *models.UserActivity) error {
	panic("unexpected call to CreateActivity")
}

func (f *fakeActivityRepository) GetLatestActivity(context.Context, uint64) (*models.UserActivity, error) {
	panic("unexpected call to GetLatestActivity")
}

func (f *fakeActivityRepository) UpdateActivity(context.Context, *models.UserActivity) error {
	panic("unexpected call to UpdateActivity")
}

func (f *fakeActivityRepository) GetTotalActivityMinutes(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetTotalActivityMinutes")
}

func (f *fakeActivityRepository) GetUserLog(context.Context, uint64) (*models.UserLog, error) {
	panic("unexpected call to GetUserLog")
}

func (f *fakeActivityRepository) CreateUserLog(context.Context, *models.UserLog) error {
	panic("unexpected call to CreateUserLog")
}

func (f *fakeActivityRepository) UpdateUserLog(context.Context, *models.UserLog) error {
	panic("unexpected call to UpdateUserLog")
}

func (f *fakeActivityRepository) IncrementLogField(context.Context, uint64, string, float64) error {
	panic("unexpected call to IncrementLogField")
}

func (f *fakeActivityRepository) CloseUserEventReport(context.Context, uint64) error {
	panic("unexpected call to CloseUserEventReport")
}

func (f *fakeActivityRepository) CreateUserEventReport(context.Context, *models.UserEventReport) error {
	panic("unexpected call to CreateUserEventReport")
}

func (f *fakeActivityRepository) CreateUserEventReportResponse(context.Context, *models.UserEventReportResponse) error {
	panic("unexpected call to CreateUserEventReportResponse")
}

func (f *fakeActivityRepository) GetUserEventByID(context.Context, uint64, uint64) (*models.UserEvent, error) {
	panic("unexpected call to GetUserEventByID")
}

func (f *fakeActivityRepository) GetUserEventsByUserID(context.Context, uint64, int32) ([]*models.UserEvent, error) {
	panic("unexpected call to GetUserEventsByUserID")
}

func (f *fakeActivityRepository) GetUserEventReportByEventID(context.Context, uint64) (*models.UserEventReport, error) {
	return nil, nil
}

func (f *fakeActivityRepository) UpdateUserEventReportStatus(context.Context, uint64, int32) error {
	panic("unexpected call to UpdateUserEventReportStatus")
}

func (f *fakeActivityRepository) GetUserEventReportResponses(context.Context, uint64) ([]*models.UserEventReportResponse, error) {
	panic("unexpected call to GetUserEventReportResponses")
}

var _ repository.ActivityRepository = (*fakeActivityRepository)(nil)

type fakeSMSServiceClient struct {
	lastRequest *notificationspb.SendOTPRequest
	err         error
}

func (f *fakeSMSServiceClient) SendSMS(context.Context, *notificationspb.SendSMSRequest, ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	panic("unexpected call to SendSMS")
}

func (f *fakeSMSServiceClient) SendOTP(_ context.Context, req *notificationspb.SendOTPRequest, _ ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	f.lastRequest = req
	if f.err != nil {
		return nil, f.err
	}
	return &notificationspb.SMSResponse{Sent: true}, nil
}

var _ notificationspb.SMSServiceClient = (*fakeSMSServiceClient)(nil)
