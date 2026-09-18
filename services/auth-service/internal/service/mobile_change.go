package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"metarang/auth-service/internal/repository"
	notificationspb "metarang/shared/pb/notifications"
	"metarang/shared/pkg/helpers"
)

const (
	mobileChangeSendPeriod        = 2 * time.Minute
	mobileChangeOTPTTL            = 120 * time.Second
	mobileChangeMaxVerifyAttempts = 3
)

func (s *authService) SendMobileChangeCode(ctx context.Context, userID uint64, mobile string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return ErrUserNotFound
	}

	sanitizedMobile, err := sanitizeUniqueIranianMobile(ctx, s.userRepo, userID, mobile)
	if err != nil {
		return err
	}

	if err := s.enforceMobileChangeSendRateLimit(ctx, userID); err != nil {
		return err
	}

	rollback := true
	defer func() {
		if !rollback {
			return
		}
		if s.cacheRepo == nil {
			return
		}
		_ = s.cacheRepo.ReleaseMobileChangeSendSlot(ctx, userID)
		_ = s.cacheRepo.DeleteMobileChangeChallenge(ctx, userID)
	}()

	code, err := generateOtpCode()
	if err != nil {
		return fmt.Errorf("failed to generate otp: %w", err)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash otp: %w", err)
	}

	if s.cacheRepo == nil {
		return fmt.Errorf("mobile change cache is not configured")
	}
	challenge := &repository.MobileChangeChallenge{
		Phone:     sanitizedMobile,
		CodeHash:  string(hashed),
		CreatedAt: time.Now(),
		Attempts:  0,
	}
	if err := s.cacheRepo.SaveMobileChangeChallenge(ctx, userID, challenge, mobileChangeOTPTTL); err != nil {
		return fmt.Errorf("failed to persist mobile change challenge: %w", err)
	}

	if err := s.dispatchMobileChangeOTP(ctx, sanitizedMobile, code); err != nil {
		return err
	}

	rollback = false
	return nil
}

func (s *authService) VerifyMobileChange(ctx context.Context, userID uint64, code, ip, userAgent string) error {
	sanitizedCode := strings.TrimSpace(helpers.NormalizePersianNumbers(code))
	if !otpCodeRegex.MatchString(sanitizedCode) {
		return ErrInvalidOTPCode
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return ErrUserNotFound
	}

	if s.cacheRepo == nil {
		return fmt.Errorf("mobile change cache is not configured")
	}

	challenge, err := s.cacheRepo.GetMobileChangeChallenge(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load mobile change challenge: %w", err)
	}
	if challenge == nil {
		return ErrOTPNotFound
	}

	if time.Since(challenge.CreatedAt) >= mobileChangeOTPTTL {
		_ = s.cacheRepo.DeleteMobileChangeChallenge(ctx, userID)
		return ErrOTPExpired
	}

	if challenge.Attempts >= mobileChangeMaxVerifyAttempts {
		return ErrVerificationAttemptRateLimited
	}

	if err := bcrypt.CompareHashAndPassword([]byte(challenge.CodeHash), []byte(sanitizedCode)); err != nil {
		challenge.Attempts++
		remaining := mobileChangeOTPTTL - time.Since(challenge.CreatedAt)
		if remaining <= 0 {
			_ = s.cacheRepo.DeleteMobileChangeChallenge(ctx, userID)
			return ErrOTPExpired
		}
		if err := s.cacheRepo.SaveMobileChangeChallenge(ctx, userID, challenge, remaining); err != nil {
			return fmt.Errorf("failed to persist verification attempts: %w", err)
		}
		return ErrInvalidOTPCode
	}

	if _, err := sanitizeUniqueIranianMobile(ctx, s.userRepo, userID, challenge.Phone); err != nil {
		return err
	}

	if err := s.userRepo.UpdatePhone(ctx, user.ID, challenge.Phone); err != nil {
		return fmt.Errorf("failed to update phone: %w", err)
	}
	if err := s.userRepo.MarkPhoneAsVerified(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to mark phone as verified: %w", err)
	}
	if err := s.cacheRepo.DeleteMobileChangeChallenge(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete mobile change challenge: %w", err)
	}

	return nil
}

func (s *authService) enforceMobileChangeSendRateLimit(ctx context.Context, userID uint64) error {
	if s.cacheRepo == nil {
		return fmt.Errorf("mobile change cache is not configured")
	}
	allowed, err := s.cacheRepo.TryAcquireMobileChangeSendSlot(ctx, userID, mobileChangeSendPeriod)
	if err != nil {
		return fmt.Errorf("failed to check mobile change send rate limit: %w", err)
	}
	if !allowed {
		return ErrVerificationRequestRateLimited
	}
	return nil
}

func (s *authService) dispatchMobileChangeOTP(ctx context.Context, phone, code string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ErrPhoneRequired
	}
	if s.notificationsClient == nil {
		return fmt.Errorf("notification service client is not configured")
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.notificationsClient.SendOTP(sendCtx, &notificationspb.SendOTPRequest{
		Phone:  phone,
		Code:   code,
		Reason: "verify",
	})
	if err != nil {
		return fmt.Errorf("failed to dispatch mobile change otp: %w", err)
	}
	return nil
}

func sanitizeUniqueIranianMobile(ctx context.Context, userRepo repository.UserRepository, userID uint64, mobile string) (string, error) {
	sanitized := strings.TrimSpace(helpers.NormalizePersianNumbers(mobile))
	if sanitized == "" {
		return "", ErrPhoneRequired
	}
	if !iranMobileRegex.MatchString(sanitized) {
		return "", ErrInvalidPhoneFormat
	}

	taken, err := userRepo.IsPhoneTaken(ctx, sanitized, userID)
	if err != nil {
		return "", fmt.Errorf("failed to validate phone uniqueness: %w", err)
	}
	if taken {
		return "", ErrPhoneAlreadyTaken
	}
	return sanitized, nil
}
