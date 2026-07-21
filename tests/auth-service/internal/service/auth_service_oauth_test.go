package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"metarang/auth-service/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	pbCommon "metarang/shared/pb/common"
	notificationspb "metarang/shared/pb/notifications"

	"google.golang.org/grpc"
)

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("successful registration URL generation", func(t *testing.T) {
		userRepo := newFakeUserRepository(map[uint64]*models.User{
			1: {ID: 1, Code: "REF123"},
		})
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"https://oauth.example.com",
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		url, err := svc.Register(ctx, "https://example.com/back", "REF123")
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if !strings.Contains(url, "https://oauth.example.com/register") {
			t.Errorf("Expected URL to contain OAuth server URL, got %q", url)
		}
		if !strings.Contains(url, "client_id=test-client-id") {
			t.Errorf("Expected URL to contain client_id, got %q", url)
		}
		if !strings.Contains(url, "redirect_uri=http%3A%2F%2Flocalhost%3A8000%2Fapi%2Fauth%2Fredirect") {
			t.Errorf("Expected URL to contain correct redirect_uri, got %q", url)
		}
		if !strings.Contains(url, "referral=REF123") {
			t.Errorf("Expected URL to contain referral code, got %q", url)
		}
		if !strings.Contains(url, "back_url=https%3A%2F%2Fexample.com%2Fback") {
			t.Errorf("Expected URL to contain back_url, got %q", url)
		}
	})

	t.Run("registration without referral", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"https://oauth.example.com",
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		url, err := svc.Register(ctx, "https://example.com/back", "")
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if strings.Contains(url, "referral=") {
			t.Errorf("Expected URL to not contain referral when empty, got %q", url)
		}
	})
}

func TestRedirect(t *testing.T) {
	ctx := context.Background()

	t.Run("successful redirect with state caching", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"https://oauth.example.com",
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		url, state, err := svc.Redirect(ctx, "https://example.com/dashboard", "https://example.com/home")
		if err != nil {
			t.Fatalf("Redirect failed: %v", err)
		}

		if state == "" {
			t.Error("Expected state to be generated")
		}
		if len(state) != 40 {
			t.Errorf("Expected state to be 40 characters, got %d", len(state))
		}

		if !strings.Contains(url, "https://oauth.example.com/oauth/authorize") {
			t.Errorf("Expected URL to contain authorize endpoint, got %q", url)
		}
		if !strings.Contains(url, "client_id=test-client-id") {
			t.Errorf("Expected URL to contain client_id, got %q", url)
		}
		if !strings.Contains(url, "response_type=code") {
			t.Errorf("Expected URL to contain response_type, got %q", url)
		}
		if !strings.Contains(url, "state="+state) {
			t.Errorf("Expected URL to contain state, got %q", url)
		}

		// Verify state was cached
		exists, _ := cacheRepo.GetState(ctx, state)
		if !exists {
			t.Error("Expected state to be cached")
		}

		// Verify redirect_to was cached
		redirectTo, _ := cacheRepo.GetRedirectTo(ctx, state)
		if redirectTo != "https://example.com/dashboard" {
			t.Errorf("Expected redirect_to to be cached, got %q", redirectTo)
		}

		// Verify back_url was cached
		backURL, _ := cacheRepo.GetBackURL(ctx, state)
		if backURL != "https://example.com/home" {
			t.Errorf("Expected back_url to be cached, got %q", backURL)
		}
	})

	t.Run("redirect with only redirect_to", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"https://oauth.example.com",
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		_, state, err := svc.Redirect(ctx, "https://example.com/dashboard", "")
		if err != nil {
			t.Fatalf("Redirect failed: %v", err)
		}

		redirectTo, _ := cacheRepo.GetRedirectTo(ctx, state)
		if redirectTo != "https://example.com/dashboard" {
			t.Errorf("Expected redirect_to to be cached, got %q", redirectTo)
		}

		backURL, _ := cacheRepo.GetBackURL(ctx, state)
		if backURL != "" {
			t.Errorf("Expected back_url to be empty, got %q", backURL)
		}
	})
}

func TestCallback(t *testing.T) {
	ctx := context.Background()

	// Create a mock OAuth server
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" && r.Method == "POST" {
			// Mock token exchange
			response := map[string]interface{}{
				"access_token":  "mock_access_token",
				"refresh_token": "mock_refresh_token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/api/user" && r.Method == "GET" {
			// Mock user profile
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer mock_access_token") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			response := map[string]interface{}{
				"name":     "Test User",
				"email":    "test@example.com",
				"mobile":   "09123456789",
				"code":     "USER123",
				"referral": "",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer oauthServer.Close()

	t.Run("successful callback with new user", func(t *testing.T) {
		users := make(map[uint64]*models.User)
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			return nil, nil // User doesn't exist yet
		}
		userRepo.createFunc = func(_ context.Context, user *models.User) error {
			if user.ID == 0 {
				user.ID = uint64(len(users) + 1)
			}
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{
				UserID:          userID,
				AutomaticLogout: 55,
			}, nil
		}
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		observerService := newFakeObserverService()

		// Set up state and redirect URLs in cache
		state := "test_state_123"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetRedirectTo(ctx, state, "https://example.com/dashboard", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, nil, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		result, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		if result.Token == "" {
			t.Error("Expected token to be generated")
		}
		if result.ExpiresAt <= 0 {
			t.Errorf("Expected expires_at to be positive, got %d", result.ExpiresAt)
		}
		if !strings.Contains(result.RedirectURL, "https://example.com/dashboard") {
			t.Errorf("Expected redirect URL to contain cached redirect_to, got %q", result.RedirectURL)
		}
		if !strings.Contains(result.RedirectURL, "token=") {
			t.Errorf("Expected redirect URL to contain token, got %q", result.RedirectURL)
		}
		if !strings.Contains(result.RedirectURL, "expires_at=") {
			t.Errorf("Expected redirect URL to contain expires_at, got %q", result.RedirectURL)
		}

		// Verify user was created
		if len(users) == 0 {
			t.Error("Expected user to be created")
		}

		// Verify state was consumed (pull semantics)
		exists, _ := cacheRepo.GetState(ctx, state)
		if exists {
			t.Error("Expected state to be consumed after callback")
		}
	})

	t.Run("callback with invalid state", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		_, err := svc.Callback(ctx, "invalid_state", "test_code", "127.0.0.1")
		if err == nil {
			t.Fatal("Expected error for invalid state")
		}
		if !strings.Contains(err.Error(), "invalid state value") {
			t.Errorf("Expected error about invalid state, got %v", err)
		}
	})

	t.Run("callback with existing user", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:    1,
				Email: "test@example.com",
				Name:  "Old Name",
			},
		}
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			if email == "test@example.com" {
				return users[1], nil
			}
			return nil, nil
		}
		userRepo.updateFunc = func(_ context.Context, user *models.User) error {
			users[user.ID] = user
			return nil
		}

		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		observerService := newFakeObserverService()

		state := "test_state_existing"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetBackURL(ctx, state, "https://example.com/home", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, nil, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		result, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		// Verify user was updated
		user := users[1]
		if user.Name != "Test User" {
			t.Errorf("Expected user name to be updated, got %q", user.Name)
		}
		if !user.AccessToken.Valid {
			t.Error("Expected access token to be set")
		}

		// Verify redirect uses back_url when redirect_to is not present
		if !strings.Contains(result.RedirectURL, "https://example.com/home") {
			t.Errorf("Expected redirect URL to use back_url, got %q", result.RedirectURL)
		}
	})

	t.Run("callback prefers redirect_to over back_url", func(t *testing.T) {
		users := make(map[uint64]*models.User)
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			return nil, nil
		}
		userRepo.createFunc = func(_ context.Context, user *models.User) error {
			if user.ID == 0 {
				user.ID = uint64(len(users) + 1)
			}
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{
				UserID:          userID,
				AutomaticLogout: 55,
			}, nil
		}
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		observerService := newFakeObserverService()

		state := "test_state_preference"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetRedirectTo(ctx, state, "https://example.com/dashboard", 5*time.Minute)
		cacheRepo.SetBackURL(ctx, state, "https://example.com/home", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, nil, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		result, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		// Should prefer redirect_to over back_url
		if !strings.Contains(result.RedirectURL, "https://example.com/dashboard") {
			t.Errorf("Expected redirect URL to use redirect_to, got %q", result.RedirectURL)
		}
		if strings.Contains(result.RedirectURL, "https://example.com/home") {
			t.Error("Expected redirect URL to not use back_url when redirect_to is present")
		}
	})
}

func TestCallbackCreatesUserRelatedRecords(t *testing.T) {
	ctx := context.Background()

	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" && r.Method == "POST" {
			response := map[string]interface{}{
				"access_token":  "mock_access_token",
				"refresh_token": "mock_refresh_token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.URL.Path == "/api/user" && r.Method == "GET" {
			response := map[string]interface{}{
				"name":     "New User",
				"email":    "newuser@example.com",
				"mobile":   "09121112233",
				"code":     "NEWUSER1",
				"referral": "",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer oauthServer.Close()

	t.Run("new user creates settings, log, activity, wallet, and variables", func(t *testing.T) {
		users := make(map[uint64]*models.User)
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			return nil, nil
		}
		userRepo.createFunc = func(_ context.Context, user *models.User) error {
			if user.ID == 0 {
				user.ID = uint64(len(users) + 1)
			}
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{
				UserID:          userID,
				AutomaticLogout: 55,
			}, nil
		}

		settingsRepo := newTrackingSettingsRepository()
		activityRepo := newTrackingActivityRepository()
		publisher := &noopPublisher{}
		observerService := service.NewObserverServiceWithSettings(userRepo, settingsRepo, activityRepo, publisher, nil)
		helperService := newFakeHelperService()

		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()

		state := "test_state_related_records"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetRedirectTo(ctx, state, "https://example.com/dashboard", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, helperService, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		result, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}
		if result.Token == "" {
			t.Fatal("Expected token to be generated")
		}

		user := users[1]
		if user == nil {
			t.Fatal("Expected user to be created with ID 1")
		}

		// Settings (Laravel UserObserver::created)
		if settingsRepo.created == nil {
			t.Fatal("Expected settings to be created for new user")
		}
		if settingsRepo.created.UserID != user.ID {
			t.Errorf("Expected settings user_id %d, got %d", user.ID, settingsRepo.created.UserID)
		}
		if settingsRepo.created.AutomaticLogout != 55 {
			t.Errorf("Expected automatic_logout 55, got %d", settingsRepo.created.AutomaticLogout)
		}
		if settingsRepo.created.CheckoutDaysCount != 3 {
			t.Errorf("Expected checkout_days_count 3, got %d", settingsRepo.created.CheckoutDaysCount)
		}
		if !settingsRepo.created.Status || !settingsRepo.created.Level || !settingsRepo.created.Details {
			t.Error("Expected default settings status/level/details to be true")
		}

		// User log
		if activityRepo.createdLog == nil {
			t.Fatal("Expected user log to be created for new user")
		}
		if activityRepo.createdLog.UserID != user.ID {
			t.Errorf("Expected log user_id %d, got %d", user.ID, activityRepo.createdLog.UserID)
		}
		if activityRepo.createdLog.Score != 0 || activityRepo.createdLog.TransactionsCount != 0 {
			t.Errorf("Expected zeroed user log, got %+v", activityRepo.createdLog)
		}

		// Initial activity (created by OnUserCreated; login may create another)
		if len(activityRepo.activities) == 0 {
			t.Fatal("Expected at least one activity to be created for new user")
		}
		if activityRepo.activities[0].UserID != user.ID {
			t.Errorf("Expected activity user_id %d, got %d", user.ID, activityRepo.activities[0].UserID)
		}
		if activityRepo.activities[0].IP != "127.0.0.1" {
			t.Errorf("Expected activity IP 127.0.0.1, got %q", activityRepo.activities[0].IP)
		}

		// Email verified on create
		if !userRepo.emailVerified {
			t.Error("Expected email to be marked verified on user creation")
		}

		// Wallet + user_variables via commercial (HelperService)
		if len(helperService.createdWalletUserIDs) != 1 || helperService.createdWalletUserIDs[0] != user.ID {
			t.Errorf("Expected CreateWallet(%d) once, got %v", user.ID, helperService.createdWalletUserIDs)
		}
		if len(helperService.createdVariableUserIDs) != 1 || helperService.createdVariableUserIDs[0] != user.ID {
			t.Errorf("Expected CreateUserVariables(%d) once, got %v", user.ID, helperService.createdVariableUserIDs)
		}
	})

	t.Run("existing user does not recreate settings, wallet, or variables", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:    1,
				Email: "newuser@example.com",
				Name:  "Existing",
			},
		}
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			if email == "newuser@example.com" {
				return users[1], nil
			}
			return nil, nil
		}
		userRepo.updateFunc = func(_ context.Context, user *models.User) error {
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{UserID: userID, AutomaticLogout: 55}, nil
		}

		settingsRepo := newTrackingSettingsRepository()
		activityRepo := newTrackingActivityRepository()
		publisher := &noopPublisher{}
		observerService := service.NewObserverServiceWithSettings(userRepo, settingsRepo, activityRepo, publisher, nil)
		helperService := newFakeHelperService()

		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()

		state := "test_state_existing_related"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetBackURL(ctx, state, "https://example.com/home", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, helperService, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		_, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		if settingsRepo.created != nil {
			t.Error("Expected settings not to be created for existing user")
		}
		if activityRepo.createdLog != nil {
			t.Error("Expected user log not to be created for existing user")
		}
		if len(helperService.createdWalletUserIDs) != 0 {
			t.Errorf("Expected CreateWallet not to be called for existing user, got %v", helperService.createdWalletUserIDs)
		}
		if len(helperService.createdVariableUserIDs) != 0 {
			t.Errorf("Expected CreateUserVariables not to be called for existing user, got %v", helperService.createdVariableUserIDs)
		}
	})
}

func TestCallbackLoggedInEvent(t *testing.T) {
	ctx := context.Background()

	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" && r.Method == "POST" {
			response := map[string]interface{}{
				"access_token":  "mock_access_token",
				"refresh_token": "mock_refresh_token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.URL.Path == "/api/user" && r.Method == "GET" {
			response := map[string]interface{}{
				"name":     "Returning User",
				"email":    "returning@example.com",
				"mobile":   "09121112233",
				"code":     "RET123",
				"referral": "",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer oauthServer.Close()

	t.Run("existing user login creates event, activity, notification, and online broadcast", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:              1,
				Email:           "returning@example.com",
				Name:            "Old Name",
				Phone:           sql.NullString{String: "09121112233", Valid: true},
				PhoneVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
			},
		}
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			if email == "returning@example.com" {
				return users[1], nil
			}
			return nil, nil
		}
		userRepo.updateFunc = func(_ context.Context, user *models.User) error {
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{
				UserID:          userID,
				AutomaticLogout: 55,
				Notifications: map[string]bool{
					"login_verification_sms":   true,
					"login_verification_email": true,
				},
			}, nil
		}

		settingsRepo := newTrackingSettingsRepository()
		activityRepo := newTrackingActivityRepository()
		publisher := newTrackingPublisher()
		notificationClient := newFakeNotificationServiceClient()
		observerService := service.NewObserverServiceWithSettings(
			userRepo, settingsRepo, activityRepo, publisher, notificationClient,
		)
		helperService := newFakeHelperService()

		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()

		state := "test_state_logged_in"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetBackURL(ctx, state, "https://example.com/home", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, helperService, nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		_, err := svc.Callback(ctx, state, "test_code", "203.0.113.10")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		// Login event (Laravel UserObserver::logedIn)
		if len(activityRepo.events) != 1 {
			t.Fatalf("Expected 1 login event, got %d", len(activityRepo.events))
		}
		if activityRepo.events[0].UserID != 1 {
			t.Errorf("Expected event user_id 1, got %d", activityRepo.events[0].UserID)
		}
		if activityRepo.events[0].Event != "ورود به حساب کاربری" {
			t.Errorf("Expected login event text, got %q", activityRepo.events[0].Event)
		}
		if activityRepo.events[0].IP != "203.0.113.10" {
			t.Errorf("Expected event IP 203.0.113.10, got %q", activityRepo.events[0].IP)
		}
		if activityRepo.events[0].Status != 1 {
			t.Errorf("Expected event status 1, got %d", activityRepo.events[0].Status)
		}

		// last_seen updated
		if !userRepo.lastSeenUpdated {
			t.Error("Expected last_seen to be updated on login")
		}

		// Activity session started
		if len(activityRepo.activities) != 1 {
			t.Fatalf("Expected 1 activity on login, got %d", len(activityRepo.activities))
		}
		if activityRepo.activities[0].UserID != 1 || activityRepo.activities[0].IP != "203.0.113.10" {
			t.Errorf("Unexpected activity: %+v", activityRepo.activities[0])
		}

		// Notification service (Laravel LogedInNotification)
		if notificationClient.lastRequest == nil {
			t.Fatal("Expected notification service SendNotification to be called")
		}
		req := notificationClient.lastRequest
		if req.UserId != 1 {
			t.Errorf("Expected notification user_id 1, got %d", req.UserId)
		}
		if req.Type != "login" {
			t.Errorf("Expected notification type login, got %q", req.Type)
		}
		if req.Title != "ورود به حساب کاربری" {
			t.Errorf("Expected notification title, got %q", req.Title)
		}
		if req.Message != "شما با موفقیت وارد حساب کاربری خود شدید." {
			t.Errorf("Expected notification message, got %q", req.Message)
		}
		if req.Data["ip"] != "203.0.113.10" {
			t.Errorf("Expected notification data ip, got %v", req.Data)
		}
		if !req.SendSms {
			t.Error("Expected SendSms true when login_verification_sms enabled and phone verified")
		}
		if !req.SendEmail {
			t.Error("Expected SendEmail true when login_verification_email enabled")
		}

		// WebSocket online broadcast
		if len(publisher.statusChanges) != 1 {
			t.Fatalf("Expected 1 status broadcast, got %d", len(publisher.statusChanges))
		}
		if publisher.statusChanges[0].userID != 1 || !publisher.statusChanges[0].online {
			t.Errorf("Expected online broadcast for user 1, got %+v", publisher.statusChanges[0])
		}

		// Must not recreate created-path records
		if settingsRepo.created != nil {
			t.Error("Expected settings not to be created for existing user login")
		}
		if len(helperService.createdWalletUserIDs) != 0 {
			t.Error("Expected wallet not to be created for existing user login")
		}
	})

	t.Run("new user does not fire logged-in observer actions", func(t *testing.T) {
		users := make(map[uint64]*models.User)
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.findByEmailFunc = func(_ context.Context, email string) (*models.User, error) {
			return nil, nil
		}
		userRepo.createFunc = func(_ context.Context, user *models.User) error {
			if user.ID == 0 {
				user.ID = uint64(len(users) + 1)
			}
			users[user.ID] = user
			return nil
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{UserID: userID, AutomaticLogout: 55}, nil
		}

		settingsRepo := newTrackingSettingsRepository()
		activityRepo := newTrackingActivityRepository()
		publisher := newTrackingPublisher()
		notificationClient := newFakeNotificationServiceClient()
		observerService := service.NewObserverServiceWithSettings(
			userRepo, settingsRepo, activityRepo, publisher, notificationClient,
		)

		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()

		state := "test_state_new_no_login"
		cacheRepo.SetState(ctx, state, 5*time.Minute)
		cacheRepo.SetRedirectTo(ctx, state, "https://example.com/dashboard", 5*time.Minute)

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, newFakeHelperService(), nil,
			oauthServer.URL,
			"test-client-id",
			"test-client-secret",
			"http://localhost:8000",
			"http://localhost:3000",
			false,
		)

		_, err := svc.Callback(ctx, state, "test_code", "127.0.0.1")
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		if notificationClient.lastRequest != nil {
			t.Error("Expected login notification not to be sent for new user (created path only)")
		}
		if len(publisher.statusChanges) != 0 {
			t.Errorf("Expected no online broadcast for new user create, got %v", publisher.statusChanges)
		}
		if len(activityRepo.events) != 0 {
			t.Errorf("Expected no login event for new user, got %d", len(activityRepo.events))
		}
		// OnUserCreated still creates initial activity
		if len(activityRepo.activities) != 1 {
			t.Fatalf("Expected only created-path activity, got %d", len(activityRepo.activities))
		}
	})
}

func TestGetMe(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get me", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:    1,
				Name:  "Test User",
				Email: "test@example.com",
				Code:  "USER123",
			},
		}
		userRepo := &extendedFakeUserRepository{
			fakeUserRepository: newFakeUserRepository(users),
		}
		userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
			return &models.Settings{
				UserID:          userID,
				AutomaticLogout: 60,
			}, nil
		}
		userRepo.getKycFunc = func(_ context.Context, userID uint64) (*models.KYC, error) {
			return nil, nil
		}
		userRepo.getUnreadNotificationsCountFunc = func(_ context.Context, userID uint64) (int32, error) {
			return 5, nil
		}

		tokenRepo := newFakeTokenRepository()
		tokenRepo.validateTokenFunc = func(_ context.Context, token string) (*models.User, error) {
			return users[1], nil
		}

		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"", "", "", "", "",
			false,
		)

		details, err := svc.GetMe(ctx, "valid_token")
		if err != nil {
			t.Fatalf("GetMe failed: %v", err)
		}

		if details.ID != 1 {
			t.Errorf("Expected user ID 1, got %d", details.ID)
		}
		if details.Name != "Test User" {
			t.Errorf("Expected name 'Test User', got %q", details.Name)
		}
		if details.AutomaticLogout != 60 {
			t.Errorf("Expected automatic_logout 60, got %d", details.AutomaticLogout)
		}
		if details.UnreadNotificationsCount != 5 {
			t.Errorf("Expected unread_notifications_count 5, got %d", details.UnreadNotificationsCount)
		}
	})

	t.Run("get me with invalid token", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		tokenRepo.validateTokenFunc = func(_ context.Context, token string) (*models.User, error) {
			return nil, fmt.Errorf("invalid token")
		}
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"", "", "", "", "",
			false,
		)

		_, err := svc.GetMe(ctx, "invalid_token")
		if err == nil {
			t.Fatal("Expected error for invalid token")
		}
	})
}

func TestLogout(t *testing.T) {
	ctx := context.Background()

	t.Run("successful logout", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:    1,
				Name:  "Test User",
				Email: "test@example.com",
			},
		}
		userRepo := newFakeUserRepository(users)
		tokenRepo := newFakeTokenRepository()
		tokenRepo.deleteUserTokensFunc = func(_ context.Context, userID uint64) error {
			return nil
		}
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()
		observerService := newFakeObserverService()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			observerService, nil, nil,
			"", "", "", "", "",
			false,
		)

		err := svc.Logout(ctx, 1, "127.0.0.1", "Mozilla/5.0")
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}

		// Verify observer was called
		if observerService.logoutCount == 0 {
			t.Error("Expected logout observer to be called")
		}
	})

	t.Run("logout with non-existent user", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"", "", "", "", "",
			false,
		)

		err := svc.Logout(ctx, 999, "127.0.0.1", "Mozilla/5.0")
		if err == nil {
			t.Fatal("Expected error for non-existent user")
		}
	})
}

func TestValidateToken(t *testing.T) {
	ctx := context.Background()

	t.Run("successful token validation", func(t *testing.T) {
		users := map[uint64]*models.User{
			1: {
				ID:    1,
				Email: "test@example.com",
			},
		}
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		tokenRepo.validateTokenFunc = func(_ context.Context, token string) (*models.User, error) {
			return users[1], nil
		}
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"", "", "", "", "",
			false,
		)

		user, err := svc.ValidateToken(ctx, "valid_token")
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
		if user == nil {
			t.Fatal("Expected user to be returned")
		}
		if user.ID != 1 {
			t.Errorf("Expected user ID 1, got %d", user.ID)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		tokenRepo.validateTokenFunc = func(_ context.Context, token string) (*models.User, error) {
			return nil, fmt.Errorf("invalid token")
		}
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := service.NewAuthService(
			userRepo, tokenRepo, cacheRepo, accountRepo, activityRepo,
			nil, nil, nil,
			"", "", "", "", "",
			false,
		)

		_, err := svc.ValidateToken(ctx, "invalid_token")
		if err == nil {
			t.Fatal("Expected error for invalid token")
		}
	})
}

// --- OAuth-specific fake implementations ---

type fakeTokenRepository struct {
	tokens               map[string]*models.User
	createTokenFunc      func(context.Context, uint64, string, time.Time) (string, error)
	validateTokenFunc    func(context.Context, string) (*models.User, error)
	deleteUserTokensFunc func(context.Context, uint64) error
}

func newFakeTokenRepository() *fakeTokenRepository {
	return &fakeTokenRepository{
		tokens: make(map[string]*models.User),
	}
}

func (f *fakeTokenRepository) Create(ctx context.Context, userID uint64, name string, expiresAt time.Time) (string, error) {
	if f.createTokenFunc != nil {
		return f.createTokenFunc(ctx, userID, name, expiresAt)
	}
	token := fmt.Sprintf("token_%d_%d", userID, time.Now().Unix())
	return fmt.Sprintf("%d|%s", userID, token), nil
}

func (f *fakeTokenRepository) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	if f.validateTokenFunc != nil {
		return f.validateTokenFunc(ctx, token)
	}
	if user, ok := f.tokens[token]; ok {
		return user, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func (f *fakeTokenRepository) DeleteUserTokens(ctx context.Context, userID uint64) error {
	if f.deleteUserTokensFunc != nil {
		return f.deleteUserTokensFunc(ctx, userID)
	}
	return nil
}

func (f *fakeTokenRepository) FindTokenByHash(ctx context.Context, tokenHash string) (*models.PersonalAccessToken, error) {
	return nil, nil
}

var _ repository.TokenRepository = (*fakeTokenRepository)(nil)

type fakeObserverService struct {
	loginCount  int
	logoutCount int
	createCount int
}

func newFakeObserverService() *fakeObserverService {
	return &fakeObserverService{}
}

func (f *fakeObserverService) OnUserLogin(ctx context.Context, user *models.User, ip, userAgent string) error {
	f.loginCount++
	return nil
}

func (f *fakeObserverService) OnUserLogout(ctx context.Context, user *models.User, ip, userAgent string) error {
	f.logoutCount++
	return nil
}

func (f *fakeObserverService) OnUserCreated(ctx context.Context, user *models.User) error {
	f.createCount++
	return nil
}

func (f *fakeObserverService) OnHourReached(ctx context.Context, user *models.User) error {
	return nil
}

func (f *fakeObserverService) CalculateScore(ctx context.Context, user *models.User) error {
	return nil
}

var _ service.ObserverService = (*fakeObserverService)(nil)

type fakeHelperService struct {
	createdWalletUserIDs   []uint64
	createdVariableUserIDs []uint64
}

func newFakeHelperService() *fakeHelperService {
	return &fakeHelperService{}
}

func (f *fakeHelperService) GetHourlyProfitTimePercentage(context.Context, uint64) (float64, error) {
	return 0, nil
}

func (f *fakeHelperService) GetScorePercentageToNextLevel(context.Context, uint64, int32) (float64, error) {
	return 0, nil
}

func (f *fakeHelperService) GetUserLevel(context.Context, uint64) (*service.LevelInfo, error) {
	return nil, nil
}

func (f *fakeHelperService) GetUserWallet(context.Context, uint64) (*service.WalletInfo, error) {
	return nil, nil
}

func (f *fakeHelperService) CreateWallet(_ context.Context, userID uint64) error {
	f.createdWalletUserIDs = append(f.createdWalletUserIDs, userID)
	return nil
}

func (f *fakeHelperService) CreateUserVariables(_ context.Context, userID uint64) error {
	f.createdVariableUserIDs = append(f.createdVariableUserIDs, userID)
	return nil
}

func (f *fakeHelperService) Close() error {
	return nil
}

var _ service.HelperService = (*fakeHelperService)(nil)

type noopPublisher struct{}

func (n *noopPublisher) PublishUserStatusChanged(context.Context, uint64, bool) error {
	return nil
}

func (n *noopPublisher) Close() error {
	return nil
}

type statusChange struct {
	userID uint64
	online bool
}

type trackingPublisher struct {
	statusChanges []statusChange
}

func newTrackingPublisher() *trackingPublisher {
	return &trackingPublisher{}
}

func (p *trackingPublisher) PublishUserStatusChanged(_ context.Context, userID uint64, online bool) error {
	p.statusChanges = append(p.statusChanges, statusChange{userID: userID, online: online})
	return nil
}

func (p *trackingPublisher) Close() error {
	return nil
}

type fakeNotificationServiceClient struct {
	lastRequest *notificationspb.SendNotificationRequest
}

func newFakeNotificationServiceClient() *fakeNotificationServiceClient {
	return &fakeNotificationServiceClient{}
}

func (f *fakeNotificationServiceClient) SendNotification(_ context.Context, in *notificationspb.SendNotificationRequest, _ ...grpc.CallOption) (*notificationspb.NotificationResponse, error) {
	f.lastRequest = in
	return &notificationspb.NotificationResponse{Id: 1, Sent: true}, nil
}

func (f *fakeNotificationServiceClient) GetNotifications(context.Context, *notificationspb.GetNotificationsRequest, ...grpc.CallOption) (*notificationspb.NotificationsResponse, error) {
	return nil, nil
}

func (f *fakeNotificationServiceClient) GetNotification(context.Context, *notificationspb.GetNotificationRequest, ...grpc.CallOption) (*notificationspb.Notification, error) {
	return nil, nil
}

func (f *fakeNotificationServiceClient) MarkAsRead(context.Context, *notificationspb.MarkAsReadRequest, ...grpc.CallOption) (*pbCommon.Empty, error) {
	return nil, nil
}

func (f *fakeNotificationServiceClient) MarkAllAsRead(context.Context, *notificationspb.MarkAllAsReadRequest, ...grpc.CallOption) (*pbCommon.Empty, error) {
	return nil, nil
}

var _ notificationspb.NotificationServiceClient = (*fakeNotificationServiceClient)(nil)

type trackingSettingsRepository struct {
	created *models.Settings
}

func newTrackingSettingsRepository() *trackingSettingsRepository {
	return &trackingSettingsRepository{}
}

func (r *trackingSettingsRepository) FindByUserID(context.Context, uint64) (*models.Settings, error) {
	return r.created, nil
}

func (r *trackingSettingsRepository) FindByID(context.Context, uint64) (*models.Settings, error) {
	return r.created, nil
}

func (r *trackingSettingsRepository) Update(context.Context, *models.Settings) error {
	return nil
}

func (r *trackingSettingsRepository) Create(_ context.Context, settings *models.Settings) error {
	copySettings := *settings
	r.created = &copySettings
	return nil
}

var _ repository.SettingsRepository = (*trackingSettingsRepository)(nil)

type trackingActivityRepository struct {
	*fakeActivityRepository
	activities []*models.UserActivity
	createdLog *models.UserLog
}

func newTrackingActivityRepository() *trackingActivityRepository {
	return &trackingActivityRepository{
		fakeActivityRepository: newFakeActivityRepository(),
	}
}

func (r *trackingActivityRepository) CreateActivity(_ context.Context, activity *models.UserActivity) error {
	copyActivity := *activity
	r.activities = append(r.activities, &copyActivity)
	return nil
}

func (r *trackingActivityRepository) CreateUserLog(_ context.Context, log *models.UserLog) error {
	copyLog := *log
	r.createdLog = &copyLog
	return nil
}

func (r *trackingActivityRepository) GetLatestActivity(_ context.Context, userID uint64) (*models.UserActivity, error) {
	for i := len(r.activities) - 1; i >= 0; i-- {
		if r.activities[i].UserID == userID {
			return r.activities[i], nil
		}
	}
	return nil, nil
}

func (r *trackingActivityRepository) UpdateActivity(context.Context, *models.UserActivity) error {
	return nil
}

func (r *trackingActivityRepository) GetTotalActivityMinutes(context.Context, uint64) (int32, error) {
	return 0, nil
}

func (r *trackingActivityRepository) GetUserLog(_ context.Context, userID uint64) (*models.UserLog, error) {
	if r.createdLog != nil && r.createdLog.UserID == userID {
		return r.createdLog, nil
	}
	return nil, nil
}

func (r *trackingActivityRepository) UpdateUserLog(context.Context, *models.UserLog) error {
	return nil
}

func (r *trackingActivityRepository) IncrementLogField(context.Context, uint64, string, float64) error {
	return nil
}

// Extended fake user repository for OAuth tests
type extendedFakeUserRepository struct {
	*fakeUserRepository
	findByEmailFunc                 func(context.Context, string) (*models.User, error)
	getSettingsFunc                 func(context.Context, uint64) (*models.Settings, error)
	getKycFunc                      func(context.Context, uint64) (*models.KYC, error)
	getUnreadNotificationsCountFunc func(context.Context, uint64) (int32, error)
	createFunc                      func(context.Context, *models.User) error
	updateFunc                      func(context.Context, *models.User) error
	emailVerified                   bool
	lastSeenUpdated                 bool
}

func (f *extendedFakeUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if f.findByEmailFunc != nil {
		return f.findByEmailFunc(ctx, email)
	}
	for _, user := range f.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (f *extendedFakeUserRepository) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	if f.getSettingsFunc != nil {
		return f.getSettingsFunc(ctx, userID)
	}
	return &models.Settings{
		UserID:          userID,
		AutomaticLogout: 55,
	}, nil
}

func (f *extendedFakeUserRepository) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	if f.getKycFunc != nil {
		return f.getKycFunc(ctx, userID)
	}
	return nil, nil
}

func (f *extendedFakeUserRepository) GetUnreadNotificationsCount(ctx context.Context, userID uint64) (int32, error) {
	if f.getUnreadNotificationsCountFunc != nil {
		return f.getUnreadNotificationsCountFunc(ctx, userID)
	}
	return 0, nil
}

func (f *extendedFakeUserRepository) Create(ctx context.Context, user *models.User) error {
	if f.createFunc != nil {
		return f.createFunc(ctx, user)
	}
	if user.ID == 0 {
		user.ID = uint64(len(f.users) + 1)
	}
	f.users[user.ID] = user
	return nil
}

func (f *extendedFakeUserRepository) Update(ctx context.Context, user *models.User) error {
	if f.updateFunc != nil {
		return f.updateFunc(ctx, user)
	}
	f.users[user.ID] = user
	return nil
}

func (f *extendedFakeUserRepository) FindByCode(ctx context.Context, code string) (*models.User, error) {
	for _, user := range f.users {
		if user.Code == code {
			return user, nil
		}
	}
	return nil, nil
}

func (f *extendedFakeUserRepository) MarkEmailAsVerified(_ context.Context, userID uint64) error {
	f.emailVerified = true
	if user, ok := f.users[userID]; ok {
		user.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	return nil
}

func (f *extendedFakeUserRepository) UpdateLastSeen(_ context.Context, userID uint64) error {
	f.lastSeenUpdated = true
	if user, ok := f.users[userID]; ok {
		user.LastSeen = sql.NullTime{Time: time.Now(), Valid: true}
	}
	return nil
}

func (f *extendedFakeUserRepository) CreateSettings(context.Context, *models.Settings) error {
	return nil
}
