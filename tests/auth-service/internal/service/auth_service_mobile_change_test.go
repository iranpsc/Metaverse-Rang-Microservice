package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
)

type mobileChangeHarness struct {
	users     map[uint64]*models.User
	userRepo  *fakeUserRepository
	cacheRepo *fakeCacheRepository
	smsClient *fakeSMSServiceClient
	resetRepo *fakeResetRepository
	svc       service.AuthService
}

func newMobileChangeHarness(users map[uint64]*models.User) *mobileChangeHarness {
	if users == nil {
		users = map[uint64]*models.User{}
	}
	cacheRepo := newFakeCacheRepository()
	smsClient := &fakeSMSServiceClient{}
	userRepo := newFakeUserRepository(users)
	resetRepo := newFakeResetRepository()
	return &mobileChangeHarness{
		users:     users,
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		smsClient: smsClient,
		resetRepo: resetRepo,
		svc: service.NewAuthService(
			userRepo,
			nil,
			cacheRepo,
			newFakeAccountSecurityRepository(),
			newFakeActivityRepository(),
			nil,
			nil,
			smsClient,
			"", "", "", "", "",
			false,
			service.WithResetRepository(resetRepo),
		),
	}
}

func (h *mobileChangeHarness) assertNoSendSlot(t *testing.T, userID uint64) {
	t.Helper()
	if _, limited := h.cacheRepo.mobileChangeSendSlots[userID]; limited {
		t.Fatalf("expected send rate-limit slot not to be consumed for user %d", userID)
	}
}

func (h *mobileChangeHarness) assertNoChallenge(t *testing.T, userID uint64) {
	t.Helper()
	if _, ok := h.cacheRepo.mobileChangeChallenges[userID]; ok {
		t.Fatalf("expected no mobile-change challenge for user %d", userID)
	}
}

func (h *mobileChangeHarness) assertNoSMS(t *testing.T) {
	t.Helper()
	if h.smsClient.lastRequest != nil {
		t.Fatalf("expected no OTP dispatch, got %+v", h.smsClient.lastRequest)
	}
}

func (h *mobileChangeHarness) assertNoReset(t *testing.T) {
	t.Helper()
	if len(h.resetRepo.resets) != 0 {
		t.Fatalf("expected no reset rows, got %+v", h.resetRepo.resets)
	}
}

func TestSendMobileChangeCodeSuccess(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{
		1: {
			ID:    1,
			Phone: sql.NullString{String: "09120000000", Valid: true},
		},
	})

	if err := h.svc.SendMobileChangeCode(ctx, 1, " 09121112233 "); err != nil {
		t.Fatalf("SendMobileChangeCode returned error: %v", err)
	}

	if h.smsClient.lastRequest == nil {
		t.Fatal("expected OTP to be dispatched")
	}
	if h.smsClient.lastRequest.Phone != "09121112233" {
		t.Errorf("expected trimmed mobile, got %q", h.smsClient.lastRequest.Phone)
	}
	if h.smsClient.lastRequest.Reason != "verify" {
		t.Errorf("expected reason verify, got %q", h.smsClient.lastRequest.Reason)
	}
	if !otpCodeRegexMatch(h.smsClient.lastRequest.Code) {
		t.Errorf("expected 6-digit OTP, got %q", h.smsClient.lastRequest.Code)
	}

	challenge := h.cacheRepo.mobileChangeChallenges[1]
	if challenge == nil {
		t.Fatal("expected challenge to be stored")
	}
	if challenge.Phone != "09121112233" {
		t.Errorf("expected pending phone 09121112233, got %q", challenge.Phone)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(challenge.CodeHash), []byte(h.smsClient.lastRequest.Code)); err != nil {
		t.Errorf("stored hash does not match dispatched code: %v", err)
	}
	if challenge.Attempts != 0 {
		t.Errorf("expected attempts 0, got %d", challenge.Attempts)
	}

	if _, limited := h.cacheRepo.mobileChangeSendSlots[1]; !limited {
		t.Fatal("expected send rate-limit slot to be consumed after success")
	}

	if len(h.resetRepo.resets) != 1 {
		t.Fatalf("expected one reset row, got %d", len(h.resetRepo.resets))
	}
	var reset *models.Reset
	for _, item := range h.resetRepo.resets {
		reset = item
	}
	if reset.UserID != 1 {
		t.Errorf("expected reset user_id 1, got %d", reset.UserID)
	}
	if reset.Type != models.ResetTypeMobile {
		t.Errorf("expected reset type mobile, got %q", reset.Type)
	}
	if reset.Value != "09121112233" {
		t.Errorf("expected reset value 09121112233, got %q", reset.Value)
	}
	if reset.Verified {
		t.Error("send must store unverified reset")
	}
	if h.cacheRepo.mobileChangeChallenges[1].ResetID != reset.ID {
		t.Errorf("expected challenge ResetID %d, got %d", reset.ID, h.cacheRepo.mobileChangeChallenges[1].ResetID)
	}

	user := h.users[1]
	if user.Phone.String != "09120000000" {
		t.Errorf("send must not change the current phone, got %q", user.Phone.String)
	}
	if user.PhoneVerifiedAt.Valid {
		t.Errorf("send must not mark phone as verified")
	}
}

func TestSendMobileChangeCodeValidationsDoNotConsumeRateLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("missing mobile", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		err := h.svc.SendMobileChangeCode(ctx, 1, "")
		if !errors.Is(err, service.ErrPhoneRequired) {
			t.Fatalf("expected ErrPhoneRequired, got %v", err)
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("whitespace mobile", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		err := h.svc.SendMobileChangeCode(ctx, 1, "  \t  ")
		if !errors.Is(err, service.ErrPhoneRequired) {
			t.Fatalf("expected ErrPhoneRequired, got %v", err)
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("invalid iranian mobile", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		err := h.svc.SendMobileChangeCode(ctx, 1, "08123456789")
		if !errors.Is(err, service.ErrInvalidPhoneFormat) {
			t.Fatalf("expected ErrInvalidPhoneFormat, got %v", err)
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("taken by another user", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{
			1: {ID: 1, Phone: sql.NullString{String: "09120000000", Valid: true}},
			2: {ID: 2, Phone: sql.NullString{String: "09121112233", Valid: true}},
		})
		err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233")
		if !errors.Is(err, service.ErrPhoneAlreadyTaken) {
			t.Fatalf("expected ErrPhoneAlreadyTaken, got %v", err)
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("uniqueness check error", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		h.userRepo.isPhoneTakenFunc = func(context.Context, string, uint64) (bool, error) {
			return false, errors.New("db down")
		}
		err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233")
		if err == nil {
			t.Fatal("expected uniqueness error")
		}
		if errors.Is(err, service.ErrVerificationRequestRateLimited) {
			t.Fatal("uniqueness error must not be mapped as rate limited")
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("user not found", func(t *testing.T) {
		h := newMobileChangeHarness(nil)
		err := h.svc.SendMobileChangeCode(ctx, 99, "09121112233")
		if !errors.Is(err, service.ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
		h.assertNoSendSlot(t, 99)
		h.assertNoSMS(t)
		h.assertNoReset(t)
	})

	t.Run("sms failure releases rate limit slot", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		h.smsClient.err = errors.New("kavenegar down")
		err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233")
		if err == nil {
			t.Fatal("expected sms error")
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		if len(h.resetRepo.resets) != 0 {
			t.Fatalf("sms failure must delete the unverified reset, got %+v", h.resetRepo.resets)
		}

		h.smsClient.err = nil
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("retry after sms failure should succeed, got %v", err)
		}
	})

	t.Run("reset limit does not consume rate limit", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		for i := 0; i < 3; i++ {
			h.resetRepo.seed(&models.Reset{
				UserID:   1,
				Type:     models.ResetTypeMobile,
				Value:    fmt.Sprintf("0912000000%d", i),
				Verified: true,
			})
		}
		err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233")
		if !errors.Is(err, service.ErrMobileResetLimitExceeded) {
			t.Fatalf("expected ErrMobileResetLimitExceeded, got %v", err)
		}
		h.assertNoSendSlot(t, 1)
		h.assertNoChallenge(t, 1)
		h.assertNoSMS(t)
		if len(h.resetRepo.resets) != 3 {
			t.Fatalf("limit rejection must not insert another reset, got %d", len(h.resetRepo.resets))
		}
	})

	t.Run("unverified resets do not count toward limit", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		for i := 0; i < 3; i++ {
			h.resetRepo.seed(&models.Reset{
				UserID:   1,
				Type:     models.ResetTypeMobile,
				Value:    fmt.Sprintf("0912000001%d", i),
				Verified: false,
			})
		}
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("unverified resets must not block send: %v", err)
		}
	})

	t.Run("email resets do not count toward mobile limit", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		for i := 0; i < 3; i++ {
			h.resetRepo.seed(&models.Reset{
				UserID:   1,
				Type:     "email",
				Value:    fmt.Sprintf("user%d@example.com", i),
				Verified: true,
			})
		}
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("email resets must not block mobile send: %v", err)
		}
	})
}

func TestSendMobileChangeCodeRateLimit(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})

	if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	remaining := time.Until(h.cacheRepo.mobileChangeSendSlots[1])
	if remaining < 2*time.Minute-time.Second || remaining > 2*time.Minute+time.Second {
		t.Fatalf("expected send slot TTL of 2 minutes, got %v", remaining)
	}
	err := h.svc.SendMobileChangeCode(ctx, 1, "09123334455")
	if !errors.Is(err, service.ErrVerificationRequestRateLimited) {
		t.Fatalf("expected ErrVerificationRequestRateLimited, got %v", err)
	}
	if h.cacheRepo.mobileChangeChallenges[1].Phone != "09121112233" {
		t.Fatalf("rate-limited resend must not replace the pending mobile")
	}
}

func TestSendMobileChangeCodeNormalizesPersianDigits(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})

	if err := h.svc.SendMobileChangeCode(ctx, 1, "۰۹۱۲۱۱۱۲۲۳۳"); err != nil {
		t.Fatalf("persian digits should be accepted: %v", err)
	}
	if h.smsClient.lastRequest.Phone != "09121112233" {
		t.Errorf("expected normalized mobile, got %q", h.smsClient.lastRequest.Phone)
	}
}

func TestVerifyMobileChangeSuccess(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{
		1: {
			ID:              1,
			Phone:           sql.NullString{String: "09120000000", Valid: true},
			PhoneVerifiedAt: sql.NullTime{Valid: false},
		},
	})

	if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	code := h.smsClient.lastRequest.Code

	if err := h.svc.VerifyMobileChange(ctx, 1, " "+code+" ", "127.0.0.1", "Mozilla"); err != nil {
		t.Fatalf("VerifyMobileChange returned error: %v", err)
	}

	user := h.users[1]
	if !user.Phone.Valid || user.Phone.String != "09121112233" {
		t.Fatalf("expected phone updated to 09121112233, got %#v", user.Phone)
	}
	if !user.PhoneVerifiedAt.Valid {
		t.Fatal("expected phone to be marked verified")
	}
	if _, ok := h.cacheRepo.mobileChangeChallenges[1]; ok {
		t.Fatal("expected challenge to be deleted after success")
	}

	var verifiedCount int
	for _, reset := range h.resetRepo.resets {
		if reset.Verified {
			verifiedCount++
		}
	}
	if verifiedCount != 1 {
		t.Fatalf("expected one verified reset after success, got %d (%+v)", verifiedCount, h.resetRepo.resets)
	}
}

func TestVerifyMobileChangeValidations(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid code format does not consume attempts", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("send failed: %v", err)
		}
		err := h.svc.VerifyMobileChange(ctx, 1, "abc123", "", "")
		if !errors.Is(err, service.ErrInvalidOTPCode) {
			t.Fatalf("expected ErrInvalidOTPCode, got %v", err)
		}
		if h.cacheRepo.mobileChangeChallenges[1].Attempts != 0 {
			t.Fatalf("format errors must not consume verify attempts")
		}
	})

	t.Run("short code does not consume attempts", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("send failed: %v", err)
		}
		err := h.svc.VerifyMobileChange(ctx, 1, "12345", "", "")
		if !errors.Is(err, service.ErrInvalidOTPCode) {
			t.Fatalf("expected ErrInvalidOTPCode, got %v", err)
		}
		if h.cacheRepo.mobileChangeChallenges[1].Attempts != 0 {
			t.Fatalf("format errors must not consume verify attempts")
		}
	})

	t.Run("missing challenge", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		err := h.svc.VerifyMobileChange(ctx, 1, "123456", "", "")
		if !errors.Is(err, service.ErrOTPNotFound) {
			t.Fatalf("expected ErrOTPNotFound, got %v", err)
		}
	})

	t.Run("expired code", func(t *testing.T) {
		h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
		if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
			t.Fatalf("send failed: %v", err)
		}
		h.cacheRepo.mobileChangeChallenges[1].CreatedAt = time.Now().Add(-121 * time.Second)
		err := h.svc.VerifyMobileChange(ctx, 1, h.smsClient.lastRequest.Code, "", "")
		if !errors.Is(err, service.ErrOTPExpired) {
			t.Fatalf("expected ErrOTPExpired, got %v", err)
		}
		if _, ok := h.cacheRepo.mobileChangeChallenges[1]; ok {
			t.Fatal("expired challenge should be deleted")
		}
	})

	t.Run("user not found", func(t *testing.T) {
		h := newMobileChangeHarness(nil)
		err := h.svc.VerifyMobileChange(ctx, 99, "123456", "", "")
		if !errors.Is(err, service.ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestVerifyMobileChangeAttemptLimit(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{1: {ID: 1}})
	if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	for i := 1; i <= 3; i++ {
		err := h.svc.VerifyMobileChange(ctx, 1, "000000", "", "")
		if !errors.Is(err, service.ErrInvalidOTPCode) {
			t.Fatalf("attempt %d: expected ErrInvalidOTPCode, got %v", i, err)
		}
	}
	if h.cacheRepo.mobileChangeChallenges[1].Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", h.cacheRepo.mobileChangeChallenges[1].Attempts)
	}

	err := h.svc.VerifyMobileChange(ctx, 1, h.smsClient.lastRequest.Code, "", "")
	if !errors.Is(err, service.ErrVerificationAttemptRateLimited) {
		t.Fatalf("expected ErrVerificationAttemptRateLimited after 3 tries, got %v", err)
	}

	user := h.users[1]
	if user.Phone.Valid {
		t.Fatalf("phone must not change after exceeded attempts, got %#v", user.Phone)
	}
}

func TestVerifyMobileChangeRejectsTakenPhoneAtVerifyTime(t *testing.T) {
	ctx := context.Background()
	h := newMobileChangeHarness(map[uint64]*models.User{
		1: {ID: 1},
		2: {ID: 2},
	})
	if err := h.svc.SendMobileChangeCode(ctx, 1, "09121112233"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	code := h.smsClient.lastRequest.Code
	h.users[2].Phone = sql.NullString{String: "09121112233", Valid: true}

	err := h.svc.VerifyMobileChange(ctx, 1, code, "", "")
	if !errors.Is(err, service.ErrPhoneAlreadyTaken) {
		t.Fatalf("expected ErrPhoneAlreadyTaken, got %v", err)
	}
	if h.users[1].Phone.Valid {
		t.Fatal("phone must not change when uniqueness fails at verify time")
	}
}

func otpCodeRegexMatch(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
