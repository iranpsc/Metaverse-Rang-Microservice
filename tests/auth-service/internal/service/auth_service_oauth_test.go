package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
)

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("successful registration URL generation", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := NewAuthService(
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
		if !strings.Contains(url, "redirect_uri=http://localhost:8000/api/auth/redirect") {
			t.Errorf("Expected URL to contain correct redirect_uri, got %q", url)
		}
		if !strings.Contains(url, "referral=REF123") {
			t.Errorf("Expected URL to contain referral code, got %q", url)
		}
		if !strings.Contains(url, "back_url=https://example.com/back") {
			t.Errorf("Expected URL to contain back_url, got %q", url)
		}
	})

	t.Run("registration without referral", func(t *testing.T) {
		userRepo := newFakeUserRepository(nil)
		tokenRepo := newFakeTokenRepository()
		cacheRepo := newFakeCacheRepository()
		accountRepo := newFakeAccountSecurityRepository()
		activityRepo := newFakeActivityRepository()

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

		svc := NewAuthService(
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

// --- Fake implementations for testing ---

type fakeCacheRepository struct {
	state                         map[string]bool
	redirectTo                    map[string]string
	backURL                       map[string]string
	ttl                           map[string]time.Duration
	setTime                       map[string]time.Time
	verificationRequestSlots      map[uint64]time.Time
}

func newFakeCacheRepository() *fakeCacheRepository {
	return &fakeCacheRepository{
		state:                    make(map[string]bool),
		redirectTo:               make(map[string]string),
		backURL:                  make(map[string]string),
		ttl:                      make(map[string]time.Duration),
		setTime:                  make(map[string]time.Time),
		verificationRequestSlots: make(map[uint64]time.Time),
	}
}

func (f *fakeCacheRepository) SetState(ctx context.Context, state string, ttl time.Duration) error {
	f.state["oauth:state:"+state] = true
	f.ttl["oauth:state:"+state] = ttl
	f.setTime["oauth:state:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetState(ctx context.Context, state string) (bool, error) {
	key := "oauth:state:" + state
	exists := f.state[key]
	if exists {
		delete(f.state, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return exists, nil
}

func (f *fakeCacheRepository) SetRedirectTo(ctx context.Context, state, redirectTo string, ttl time.Duration) error {
	f.redirectTo["oauth:redirect_to:"+state] = redirectTo
	f.ttl["oauth:redirect_to:"+state] = ttl
	f.setTime["oauth:redirect_to:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetRedirectTo(ctx context.Context, state string) (string, error) {
	key := "oauth:redirect_to:" + state
	val := f.redirectTo[key]
	if val != "" {
		delete(f.redirectTo, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return val, nil
}

func (f *fakeCacheRepository) SetBackURL(ctx context.Context, state, backURL string, ttl time.Duration) error {
	f.backURL["oauth:back_url:"+state] = backURL
	f.ttl["oauth:back_url:"+state] = ttl
	f.setTime["oauth:back_url:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetBackURL(ctx context.Context, state string) (string, error) {
	key := "oauth:back_url:" + state
	val := f.backURL[key]
	if val != "" {
		delete(f.backURL, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return val, nil
}

func (f *fakeCacheRepository) TryAcquireAccountSecurityVerificationSlot(_ context.Context, userID uint64, period time.Duration) (bool, error) {
	if until, exists := f.verificationRequestSlots[userID]; exists && time.Now().Before(until) {
		return false, nil
	}
	f.verificationRequestSlots[userID] = time.Now().Add(period)
	return true, nil
}

var _ repository.CacheRepository = (*fakeCacheRepository)(nil)

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

var _ ObserverService = (*fakeObserverService)(nil)

// Extended fake user repository for OAuth tests
type extendedFakeUserRepository struct {
	*fakeUserRepository
	findByEmailFunc                 func(context.Context, string) (*models.User, error)
	getSettingsFunc                 func(context.Context, uint64) (*models.Settings, error)
	getKycFunc                      func(context.Context, uint64) (*models.KYC, error)
	getUnreadNotificationsCountFunc func(context.Context, uint64) (int32, error)
	createFunc                      func(context.Context, *models.User) error
	updateFunc                      func(context.Context, *models.User) error
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
