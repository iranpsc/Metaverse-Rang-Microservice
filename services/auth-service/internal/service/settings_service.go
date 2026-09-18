package service

import (
	"context"
	"errors"
	"fmt"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

var (
	ErrSettingsNotFound       = errors.New("settings not found")
	ErrInvalidCheckoutDays    = errors.New("checkout_days_count must be between 3 and 1000")
	ErrInvalidAutomaticLogout = errors.New("automatic_logout must be between 1 and 55")
	ErrInvalidProfileSetting  = errors.New("setting must be one of: status, level, details")
	ErrInvalidPrivacyKey      = errors.New("invalid privacy key")
	ErrInvalidPrivacyValue    = errors.New("privacy value must be 0 or 1")
	ErrMissingRequiredFields  = errors.New("missing required fields")
)

type GeneralSettings struct {
	ID            uint64
	Notifications map[string]bool
}

type SettingsService interface {
	GetSettings(ctx context.Context, userID uint64) (*models.Settings, error)
	UpdateSettings(ctx context.Context, userID uint64, checkoutDaysCount *uint32, automaticLogout *int32, setting *string, status *bool) error
	GetGeneralSettings(ctx context.Context, userID uint64) (*GeneralSettings, error)
	UpdateGeneralSettings(ctx context.Context, userID uint64, settingID uint64, notifications map[string]bool) (map[string]bool, error)
	GetPrivacySettings(ctx context.Context, userID uint64) (map[string]int, error)
	UpdatePrivacySettings(ctx context.Context, userID uint64, key string, value int32) error
}

type settingsService struct {
	settingsRepo repository.SettingsRepository
	resetRepo    repository.ResetRepository
}

func NewSettingsService(settingsRepo repository.SettingsRepository) SettingsService {
	return NewSettingsServiceWithResets(settingsRepo, nil)
}

func NewSettingsServiceWithResets(settingsRepo repository.SettingsRepository, resetRepo repository.ResetRepository) SettingsService {
	return &settingsService{
		settingsRepo: settingsRepo,
		resetRepo:    resetRepo,
	}
}

func availableResetMobileResets(verifiedCount int) int32 {
	remaining := maxVerifiedMobileResets - verifiedCount
	if remaining < 0 {
		return 0
	}
	return int32(remaining)
}

func (s *settingsService) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	settings, err := s.settingsRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	if settings == nil {
		return nil, fmt.Errorf("failed to get settings: empty result")
	}

	settings.AvailableResetMobileResets = int32(maxVerifiedMobileResets)
	if s.resetRepo != nil {
		count, err := s.resetRepo.CountVerifiedByUserAndType(ctx, userID, models.ResetTypeMobile)
		if err != nil {
			return nil, fmt.Errorf("failed to count mobile resets: %w", err)
		}
		settings.AvailableResetMobileResets = availableResetMobileResets(count)
	}
	return settings, nil
}

func (s *settingsService) UpdateSettings(ctx context.Context, userID uint64, checkoutDaysCount *uint32, automaticLogout *int32, setting *string, status *bool) error {
	// Get existing settings or create default
	settings, err := s.settingsRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get settings: %w", err)
	}

	// If settings don't exist in DB, create them
	if settings.ID == 0 {
		settings.UserID = userID
		if err := s.settingsRepo.Create(ctx, settings); err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	}

	// Update checkout cadence if provided
	if checkoutDaysCount != nil && automaticLogout != nil {
		if *checkoutDaysCount < 3 || *checkoutDaysCount > 1000 {
			return ErrInvalidCheckoutDays
		}
		if *automaticLogout < 1 || *automaticLogout > 55 {
			return ErrInvalidAutomaticLogout
		}
		settings.CheckoutDaysCount = *checkoutDaysCount
		settings.AutomaticLogout = *automaticLogout
	} else if checkoutDaysCount != nil || automaticLogout != nil {
		// If only one is provided, return error
		return ErrMissingRequiredFields
	}

	// Update profile exposure toggle if provided
	if setting != nil && status != nil {
		switch *setting {
		case "status":
			settings.Status = *status
		case "level":
			settings.Level = *status
		case "details":
			settings.Details = *status
		default:
			return ErrInvalidProfileSetting
		}
	} else if setting != nil || status != nil {
		// If only one is provided, return error
		return ErrMissingRequiredFields
	}

	// Save changes
	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

func (s *settingsService) GetGeneralSettings(ctx context.Context, userID uint64) (*GeneralSettings, error) {
	settings, err := s.settingsRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return &GeneralSettings{
		ID:            settings.ID,
		Notifications: settings.Notifications,
	}, nil
}

func (s *settingsService) UpdateGeneralSettings(ctx context.Context, userID uint64, settingID uint64, notifications map[string]bool) (map[string]bool, error) {
	// Verify ownership
	settings, err := s.settingsRepo.FindByID(ctx, settingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	if settings == nil {
		return nil, ErrSettingsNotFound
	}
	if settings.UserID != userID {
		return nil, errors.New("settings do not belong to user")
	}

	// Validate all required notification channels are present
	requiredChannels := []string{
		"announcements_sms",
		"announcements_email",
		"reports_sms",
		"reports_email",
		"login_verification_sms",
		"login_verification_email",
		"transactions_sms",
		"transactions_email",
		"trades_sms",
		"trades_email",
	}

	for _, channel := range requiredChannels {
		if _, exists := notifications[channel]; !exists {
			return nil, fmt.Errorf("missing required notification channel: %s", channel)
		}
	}

	// Update notifications
	settings.Notifications = notifications
	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to update general settings: %w", err)
	}

	return settings.Notifications, nil
}

func (s *settingsService) GetPrivacySettings(ctx context.Context, userID uint64) (map[string]int, error) {
	settings, err := s.settingsRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return settings.Privacy, nil
}

func (s *settingsService) UpdatePrivacySettings(ctx context.Context, userID uint64, key string, value int32) error {
	// Get existing settings or create default
	settings, err := s.settingsRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get settings: %w", err)
	}

	// If settings don't exist in DB, create them
	if settings.ID == 0 {
		settings.UserID = userID
		if err := s.settingsRepo.Create(ctx, settings); err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	}

	// Validate key exists in default privacy settings
	defaultPrivacy := models.DefaultPrivacySettings()
	if _, exists := defaultPrivacy[key]; !exists {
		return ErrInvalidPrivacyKey
	}

	// Validate value is 0 or 1
	if value != 0 && value != 1 {
		return ErrInvalidPrivacyValue
	}

	// Update privacy setting
	if settings.Privacy == nil {
		settings.Privacy = models.DefaultPrivacySettings()
	}
	settings.Privacy[key] = int(value)

	// Save changes
	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("failed to update privacy settings: %w", err)
	}

	return nil
}
