package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	notificationspb "metarang/shared/pb/notifications"
)

const accountSecurityVerificationRequestPeriod = time.Minute

// AccountSecurityService owns unlock requests, OTP verification, and the lock check.
type AccountSecurityService interface {
	RequestAccountSecurity(ctx context.Context, userID uint64, minutes int32, phone string) error
	VerifyAccountSecurity(ctx context.Context, userID uint64, code, ip, userAgent string) error
	// CheckAccountSecurity reports whether mutating requests may proceed.
	// Returns true when no security record exists or the unlock window is still valid.
	CheckAccountSecurity(ctx context.Context, userID uint64) (bool, error)
}

type accountSecurityService struct {
	userRepo                      repository.UserRepository
	accountSecurityRepo           repository.AccountSecurityRepository
	cacheRepo                     repository.CacheRepository
	notificationsClient           notificationspb.SMSServiceClient
	rateLimitVerificationRequests bool
}

func NewAccountSecurityService(
	userRepo repository.UserRepository,
	accountSecurityRepo repository.AccountSecurityRepository,
	cacheRepo repository.CacheRepository,
	notificationsClient notificationspb.SMSServiceClient,
	rateLimitVerificationRequests bool,
) AccountSecurityService {
	return &accountSecurityService{
		userRepo:                      userRepo,
		accountSecurityRepo:           accountSecurityRepo,
		cacheRepo:                     cacheRepo,
		notificationsClient:           notificationsClient,
		rateLimitVerificationRequests: rateLimitVerificationRequests,
	}
}

// externalCallError marks a failed call to another service.
// The verification rate-limit slot is released when this error is returned.
type externalCallError struct {
	err error
}

func (e *externalCallError) Error() string {
	if e == nil || e.err == nil {
		return "external call failed"
	}
	return e.err.Error()
}

func (e *externalCallError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (s *accountSecurityService) RequestAccountSecurity(ctx context.Context, userID uint64, minutes int32, phone string) (err error) {
	if minutes < 5 || minutes > 60 {
		return ErrInvalidUnlockDuration
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return ErrUserNotFound
	}

	// Validate phone before consuming a verification rate-limit slot so users
	// without a verified mobile get a validation error instead of rate limiting.
	phoneForOTP := strings.TrimSpace(user.Phone.String)
	if !user.Phone.Valid || phoneForOTP == "" || !user.PhoneVerifiedAt.Valid {
		phoneForOTP = strings.TrimSpace(phone)
		if phoneForOTP == "" {
			return ErrPhoneRequired
		}
		if !iranMobileRegex.MatchString(phoneForOTP) {
			return ErrInvalidPhoneFormat
		}
		taken, phoneErr := s.userRepo.IsPhoneTaken(ctx, phoneForOTP, user.ID)
		if phoneErr != nil {
			return fmt.Errorf("failed to validate phone uniqueness: %w", phoneErr)
		}
		if taken {
			return ErrPhoneAlreadyTaken
		}
	}

	// Apply rate limiting only after validation passes. The slot is released
	// again when a later validation failure or the notification API call fails.
	slotHeld, err := s.acquireAccountSecurityVerificationSlot(ctx, userID)
	if err != nil {
		return err
	}
	defer func() {
		if !slotHeld || err == nil || !releasesVerificationSlot(err) {
			return
		}
		if releaseErr := s.cacheRepo.ReleaseAccountSecurityVerificationSlot(ctx, userID); releaseErr != nil {
			err = fmt.Errorf("%w (also failed to release verification rate limit: %v)", err, releaseErr)
		}
	}()

	lengthSeconds := int64(minutes) * 60

	security, err := s.accountSecurityRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load account security: %w", err)
	}

	if security == nil {
		security = &models.AccountSecurity{
			UserID:   userID,
			Unlocked: false,
			Until:    sql.NullInt64{},
			Length:   lengthSeconds,
		}
		if err = s.accountSecurityRepo.Create(ctx, security); err != nil {
			return fmt.Errorf("failed to create account security: %w", err)
		}
	} else {
		security.Unlocked = false
		security.Until = sql.NullInt64{}
		security.Length = lengthSeconds
		if err = s.accountSecurityRepo.Update(ctx, security); err != nil {
			return fmt.Errorf("failed to update account security: %w", err)
		}
	}

	code, err := generateOtpCode()
	if err != nil {
		return fmt.Errorf("failed to generate otp: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash otp: %w", err)
	}

	otp := &models.Otp{
		UserID:       user.ID,
		VerifiableID: security.ID,
		Code:         string(hashed),
		Phone:        sql.NullString{String: phoneForOTP, Valid: true},
	}

	if err = s.accountSecurityRepo.UpsertOtp(ctx, otp); err != nil {
		return fmt.Errorf("failed to persist otp: %w", err)
	}

	if err = s.dispatchAccountSecurityOTP(ctx, phoneForOTP, code); err != nil {
		return err
	}

	return nil
}

func (s *accountSecurityService) VerifyAccountSecurity(ctx context.Context, userID uint64, code, ip, userAgent string) error {
	sanitizedCode := strings.TrimSpace(code)
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

	security, err := s.accountSecurityRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load account security: %w", err)
	}
	if security == nil {
		return ErrAccountSecurityNotFound
	}
	if security.Unlocked {
		return ErrAccountSecurityAlreadyUnlocked
	}

	otp, err := s.accountSecurityRepo.GetOtpByAccountSecurity(ctx, security.ID)
	if err != nil {
		return fmt.Errorf("failed to load otp: %w", err)
	}
	if otp == nil || otp.UserID != userID || !otp.Phone.Valid || otp.Phone.String == "" {
		return ErrAccountSecurityNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otp.Code), []byte(sanitizedCode)); err != nil {
		return ErrInvalidOTPCode
	}
	taken, err := s.userRepo.IsPhoneTaken(ctx, otp.Phone.String, userID)
	if err != nil {
		return fmt.Errorf("failed to validate phone uniqueness: %w", err)
	}
	if taken {
		return ErrPhoneAlreadyTaken
	}

	event := &models.UserEvent{
		UserID: user.ID,
		Event:  "غیر فعال سازی امنیت حساب کاربری",
		IP:     strings.TrimSpace(ip),
		Device: strings.TrimSpace(userAgent),
		Status: 1,
	}
	if err := s.accountSecurityRepo.CompleteVerification(ctx, otp, event); err != nil {
		switch {
		case errors.Is(err, repository.ErrUserPhoneTaken):
			return ErrPhoneAlreadyTaken
		case errors.Is(err, repository.ErrAccountSecurityChallengeChanged):
			return ErrInvalidOTPCode
		default:
			return fmt.Errorf("failed to complete account security verification: %w", err)
		}
	}
	return nil
}

func (s *accountSecurityService) CheckAccountSecurity(ctx context.Context, userID uint64) (bool, error) {
	security, err := s.accountSecurityRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to load account security: %w", err)
	}
	// No record means account security has never been enabled for this user.
	if security == nil {
		return true, nil
	}
	if !security.Unlocked {
		return false, nil
	}
	now := time.Now().Unix()
	if security.Until.Valid && security.Until.Int64 < now {
		security.Unlocked = false
		if err := s.accountSecurityRepo.Update(ctx, security); err != nil {
			return false, fmt.Errorf("failed to expire account security: %w", err)
		}
		return false, nil
	}

	security.LastActivity = sql.NullInt64{Int64: now, Valid: true}
	if err := s.accountSecurityRepo.Update(ctx, security); err != nil {
		return false, fmt.Errorf("failed to update account security activity: %w", err)
	}
	return true, nil
}

func (s *accountSecurityService) acquireAccountSecurityVerificationSlot(ctx context.Context, userID uint64) (bool, error) {
	if !s.rateLimitVerificationRequests {
		return false, nil
	}
	if s.cacheRepo == nil {
		return false, fmt.Errorf("verification request rate limit is enabled but cache is not configured")
	}

	allowed, err := s.cacheRepo.TryAcquireAccountSecurityVerificationSlot(
		ctx,
		userID,
		accountSecurityVerificationRequestPeriod,
	)
	if err != nil {
		return false, fmt.Errorf("failed to check verification request rate limit: %w", err)
	}
	if !allowed {
		return false, ErrVerificationRequestRateLimited
	}
	return true, nil
}

func (s *accountSecurityService) dispatchAccountSecurityOTP(ctx context.Context, phone, code string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ErrPhoneRequired
	}

	if s.notificationsClient == nil {
		return &externalCallError{err: fmt.Errorf("notification service client is not configured")}
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.notificationsClient.SendOTP(sendCtx, &notificationspb.SendOTPRequest{
		Phone:  phone,
		Code:   code,
		Reason: "verify",
	})
	if err != nil {
		return &externalCallError{err: fmt.Errorf("failed to dispatch account security otp: %w", err)}
	}

	return nil
}

// releasesVerificationSlot reports whether a failed request should give the
// rate-limit slot back. Validation failures and notification API failures do
// not count against the limit. Persistence failures keep the slot.
func releasesVerificationSlot(err error) bool {
	if err == nil || errors.Is(err, ErrVerificationRequestRateLimited) {
		return false
	}
	if isAccountSecurityValidationError(err) {
		return true
	}
	var external *externalCallError
	return errors.As(err, &external)
}

func isAccountSecurityValidationError(err error) bool {
	return errors.Is(err, ErrInvalidUnlockDuration) ||
		errors.Is(err, ErrPhoneRequired) ||
		errors.Is(err, ErrInvalidPhoneFormat) ||
		errors.Is(err, ErrPhoneAlreadyTaken) ||
		errors.Is(err, ErrUserNotFound) ||
		errors.Is(err, ErrInvalidOTPCode)
}
