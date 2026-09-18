package service_test

import (
	"context"
	"errors"
	"metarang/auth-service/internal/service"
	"testing"

	"metarang/auth-service/internal/models"
)

type mockSettingsRepository struct {
	findByUserIDFunc func(context.Context, uint64) (*models.Settings, error)
	findByIDFunc     func(context.Context, uint64) (*models.Settings, error)
	updateFunc       func(context.Context, *models.Settings) error
	createFunc       func(context.Context, *models.Settings) error
}

func (m *mockSettingsRepository) FindByUserID(ctx context.Context, userID uint64) (*models.Settings, error) {
	if m.findByUserIDFunc != nil {
		return m.findByUserIDFunc(ctx, userID)
	}
	return &models.Settings{
		ID:                1,
		UserID:            userID,
		Status:            true,
		Level:             true,
		Details:           true,
		CheckoutDaysCount: 3,
		AutomaticLogout:   55,
		Privacy:           models.DefaultPrivacySettings(),
		Notifications:     models.DefaultNotificationSettings(),
	}, nil
}

func (m *mockSettingsRepository) FindByID(ctx context.Context, id uint64) (*models.Settings, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return &models.Settings{
		ID:                id,
		UserID:            1,
		Status:            true,
		Level:             true,
		Details:           true,
		CheckoutDaysCount: 3,
		AutomaticLogout:   55,
		Privacy:           models.DefaultPrivacySettings(),
		Notifications:     models.DefaultNotificationSettings(),
	}, nil
}

func (m *mockSettingsRepository) Update(ctx context.Context, settings *models.Settings) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, settings)
	}
	return nil
}

func (m *mockSettingsRepository) Create(ctx context.Context, settings *models.Settings) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, settings)
	}
	settings.ID = 1
	return nil
}

func TestSettingsService_GetSettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("returns settings successfully", func(t *testing.T) {
		settings, err := svc.GetSettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings == nil {
			t.Fatal("expected settings, got nil")
		}
		if settings.UserID != 1 {
			t.Errorf("expected userID 1, got %d", settings.UserID)
		}
		if settings.AvailableResetMobileResets != 3 {
			t.Errorf("expected 3 available mobile resets when reset repo is unset, got %d", settings.AvailableResetMobileResets)
		}
	})

	t.Run("handles repository error", func(t *testing.T) {
		mockRepo.findByUserIDFunc = func(context.Context, uint64) (*models.Settings, error) {
			return nil, errors.New("database error")
		}
		_, err := svc.GetSettings(ctx, 1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSettingsService_GetSettingsAvailableMobileResets(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockSettingsRepository{}
	resetRepo := newFakeResetRepository()
	svc := service.NewSettingsServiceWithResets(mockRepo, resetRepo)

	t.Run("no verified resets yields 3", func(t *testing.T) {
		settings, err := svc.GetSettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.AvailableResetMobileResets != 3 {
			t.Errorf("expected 3, got %d", settings.AvailableResetMobileResets)
		}
	})

	t.Run("subtracts verified mobile resets from 3", func(t *testing.T) {
		resetRepo.seed(&models.Reset{UserID: 1, Type: models.ResetTypeMobile, Value: "09120000001", Verified: true})
		resetRepo.seed(&models.Reset{UserID: 1, Type: models.ResetTypeMobile, Value: "09120000002", Verified: true})
		resetRepo.seed(&models.Reset{UserID: 1, Type: models.ResetTypeMobile, Value: "09120000003", Verified: false})
		resetRepo.seed(&models.Reset{UserID: 1, Type: "email", Value: "a@b.c", Verified: true})
		resetRepo.seed(&models.Reset{UserID: 2, Type: models.ResetTypeMobile, Value: "09120000009", Verified: true})

		settings, err := svc.GetSettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.AvailableResetMobileResets != 1 {
			t.Errorf("expected 1 available reset, got %d", settings.AvailableResetMobileResets)
		}
	})

	t.Run("clamps at zero when over limit", func(t *testing.T) {
		resetRepo := newFakeResetRepository()
		for i := 0; i < 5; i++ {
			resetRepo.seed(&models.Reset{UserID: 1, Type: models.ResetTypeMobile, Verified: true})
		}
		svc := service.NewSettingsServiceWithResets(mockRepo, resetRepo)
		settings, err := svc.GetSettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.AvailableResetMobileResets != 0 {
			t.Errorf("expected 0, got %d", settings.AvailableResetMobileResets)
		}
	})

	t.Run("count error is returned", func(t *testing.T) {
		resetRepo := newFakeResetRepository()
		resetRepo.countErr = errors.New("db down")
		svc := service.NewSettingsServiceWithResets(mockRepo, resetRepo)
		_, err := svc.GetSettings(ctx, 1)
		if err == nil {
			t.Fatal("expected count error")
		}
	})
}

func TestSettingsService_UpdateSettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("updates checkout cadence successfully", func(t *testing.T) {
		checkoutDays := uint32(10)
		automaticLogout := int32(30)

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validates checkout days range", func(t *testing.T) {
		checkoutDays := uint32(2) // Invalid: must be >= 3
		automaticLogout := int32(30)

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != service.ErrInvalidCheckoutDays {
			t.Errorf("expected service.ErrInvalidCheckoutDays, got %v", err)
		}
	})

	t.Run("validates automatic logout range - too low", func(t *testing.T) {
		checkoutDays := uint32(10)
		automaticLogout := int32(0) // Invalid: must be >= 1

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != service.ErrInvalidAutomaticLogout {
			t.Errorf("expected service.ErrInvalidAutomaticLogout, got %v", err)
		}
	})

	t.Run("validates automatic logout range - too high", func(t *testing.T) {
		checkoutDays := uint32(10)
		automaticLogout := int32(60) // Invalid: must be <= 55

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != service.ErrInvalidAutomaticLogout {
			t.Errorf("expected service.ErrInvalidAutomaticLogout, got %v", err)
		}
	})

	t.Run("validates checkout days range - too high", func(t *testing.T) {
		checkoutDays := uint32(1001) // Invalid: must be <= 1000
		automaticLogout := int32(30)

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != service.ErrInvalidCheckoutDays {
			t.Errorf("expected service.ErrInvalidCheckoutDays, got %v", err)
		}
	})

	t.Run("updates profile exposure successfully", func(t *testing.T) {
		setting := "status"
		status := false

		err := svc.UpdateSettings(ctx, 1, nil, nil, &setting, &status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validates profile setting name", func(t *testing.T) {
		setting := "invalid"
		status := false

		err := svc.UpdateSettings(ctx, 1, nil, nil, &setting, &status)
		if err != service.ErrInvalidProfileSetting {
			t.Errorf("expected service.ErrInvalidProfileSetting, got %v", err)
		}
	})

	t.Run("requires both checkout fields", func(t *testing.T) {
		checkoutDays := uint32(10)

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, nil, nil, nil)
		if err != service.ErrMissingRequiredFields {
			t.Errorf("expected service.ErrMissingRequiredFields, got %v", err)
		}
	})

	t.Run("requires both profile exposure fields", func(t *testing.T) {
		setting := "status"

		err := svc.UpdateSettings(ctx, 1, nil, nil, &setting, nil)
		if err != service.ErrMissingRequiredFields {
			t.Errorf("expected service.ErrMissingRequiredFields, got %v", err)
		}
	})

	t.Run("creates settings if not exists", func(t *testing.T) {
		mockRepo.findByUserIDFunc = func(context.Context, uint64) (*models.Settings, error) {
			return &models.Settings{
				ID:                0, // No ID means not in DB
				UserID:            1,
				Status:            true,
				Level:             true,
				Details:           true,
				CheckoutDaysCount: 3,
				AutomaticLogout:   55,
				Privacy:           models.DefaultPrivacySettings(),
				Notifications:     models.DefaultNotificationSettings(),
			}, nil
		}
		created := false
		mockRepo.createFunc = func(context.Context, *models.Settings) error {
			created = true
			return nil
		}

		checkoutDays := uint32(10)
		automaticLogout := int32(30)

		err := svc.UpdateSettings(ctx, 1, &checkoutDays, &automaticLogout, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Error("expected settings to be created")
		}
	})
}

func TestSettingsService_GetGeneralSettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("returns notification settings with id", func(t *testing.T) {
		generalSettings, err := svc.GetGeneralSettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if generalSettings == nil {
			t.Fatal("expected general settings")
		}
		if generalSettings.ID != 1 {
			t.Errorf("expected settings id 1, got %d", generalSettings.ID)
		}
		if len(generalSettings.Notifications) == 0 {
			t.Error("expected non-empty notifications map")
		}
	})
}

func TestSettingsService_UpdateGeneralSettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("updates general settings successfully", func(t *testing.T) {
		notifications := map[string]bool{
			"announcements_sms":        false,
			"announcements_email":      true,
			"reports_sms":              true,
			"reports_email":            true,
			"login_verification_sms":   true,
			"login_verification_email": true,
			"transactions_sms":         false,
			"transactions_email":       true,
			"trades_sms":               true,
			"trades_email":             true,
		}

		updated, err := svc.UpdateGeneralSettings(ctx, 1, 1, notifications)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated["announcements_sms"] != false {
			t.Error("expected announcements_sms to be false")
		}
	})

	t.Run("validates ownership", func(t *testing.T) {
		mockRepo.findByIDFunc = func(context.Context, uint64) (*models.Settings, error) {
			return &models.Settings{
				ID:     1,
				UserID: 2, // Different user
			}, nil
		}

		notifications := models.DefaultNotificationSettings()
		_, err := svc.UpdateGeneralSettings(ctx, 1, 1, notifications)
		if err == nil {
			t.Fatal("expected error for ownership check")
		}
		if err.Error() != "settings do not belong to user" {
			t.Errorf("expected ownership error, got %v", err)
		}
	})

	t.Run("handles missing settings", func(t *testing.T) {
		mockRepo.findByIDFunc = func(context.Context, uint64) (*models.Settings, error) {
			return nil, nil
		}

		notifications := models.DefaultNotificationSettings()
		_, err := svc.UpdateGeneralSettings(ctx, 1, 1, notifications)
		if err != service.ErrSettingsNotFound {
			t.Errorf("expected service.ErrSettingsNotFound, got %v", err)
		}
	})

	t.Run("validates all required channels", func(t *testing.T) {
		notifications := map[string]bool{
			"announcements_sms": true,
			// Missing other channels
		}

		_, err := svc.UpdateGeneralSettings(ctx, 1, 1, notifications)
		if err == nil {
			t.Fatal("expected error for missing channels")
		}
	})
}

func TestSettingsService_GetPrivacySettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("returns privacy settings", func(t *testing.T) {
		privacy, err := svc.GetPrivacySettings(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if privacy == nil {
			t.Fatal("expected privacy map")
		}
		if len(privacy) == 0 {
			t.Error("expected non-empty privacy map")
		}
	})
}

func TestSettingsService_UpdatePrivacySettings(t *testing.T) {
	mockRepo := &mockSettingsRepository{}
	svc := service.NewSettingsService(mockRepo)
	ctx := context.Background()

	t.Run("updates privacy setting successfully", func(t *testing.T) {
		err := svc.UpdatePrivacySettings(ctx, 1, "phone", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validates privacy key", func(t *testing.T) {
		err := svc.UpdatePrivacySettings(ctx, 1, "invalid_key", 1)
		if err != service.ErrInvalidPrivacyKey {
			t.Errorf("expected service.ErrInvalidPrivacyKey, got %v", err)
		}
	})

	t.Run("validates privacy value", func(t *testing.T) {
		err := svc.UpdatePrivacySettings(ctx, 1, "phone", 2)
		if err != service.ErrInvalidPrivacyValue {
			t.Errorf("expected service.ErrInvalidPrivacyValue, got %v", err)
		}
	})

	t.Run("creates settings if not exists", func(t *testing.T) {
		mockRepo.findByUserIDFunc = func(context.Context, uint64) (*models.Settings, error) {
			return &models.Settings{
				ID:      0,
				UserID:  1,
				Privacy: models.DefaultPrivacySettings(),
			}, nil
		}
		created := false
		mockRepo.createFunc = func(context.Context, *models.Settings) error {
			created = true
			return nil
		}

		err := svc.UpdatePrivacySettings(ctx, 1, "phone", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Error("expected settings to be created")
		}
	})
}
