package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
)

func TestAuthHandler_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("successful registration", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.registerFunc = func(ctx context.Context, backURL, referral string) (string, error) {
			return "https://oauth.example.com/register?client_id=test&redirect_uri=...", nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RegisterRequest{
			BackUrl:  "https://example.com/back",
			Referral: "REF123",
		}

		resp, err := handler.Register(ctx, req)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if resp.Url == "" {
			t.Error("Expected URL to be returned")
		}
	})

	t.Run("registration service error", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.registerFunc = func(ctx context.Context, backURL, referral string) (string, error) {
			return "", errors.New("service error")
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RegisterRequest{
			BackUrl:  "https://example.com/back",
			Referral: "REF123",
		}

		_, err := handler.Register(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Internal {
			t.Errorf("Expected Internal error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_Redirect(t *testing.T) {
	ctx := context.Background()

	t.Run("successful redirect", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.redirectFunc = func(ctx context.Context, redirectTo, backURL string) (string, string, error) {
			return "https://oauth.example.com/oauth/authorize?state=abc123", "abc123", nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RedirectRequest{
			RedirectTo: "https://example.com/dashboard",
			BackUrl:    "https://example.com/home",
		}

		resp, err := handler.Redirect(ctx, req)
		if err != nil {
			t.Fatalf("Redirect failed: %v", err)
		}

		if resp.Url == "" {
			t.Error("Expected URL to be returned")
		}
	})

	t.Run("redirect with only redirect_to", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.redirectFunc = func(ctx context.Context, redirectTo, backURL string) (string, string, error) {
			return "https://oauth.example.com/oauth/authorize?state=xyz789", "xyz789", nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RedirectRequest{
			RedirectTo: "https://example.com/dashboard",
		}

		resp, err := handler.Redirect(ctx, req)
		if err != nil {
			t.Fatalf("Redirect failed: %v", err)
		}

		if resp.Url == "" {
			t.Error("Expected URL to be returned")
		}
	})
}

func TestAuthHandler_Callback(t *testing.T) {
	ctx := context.Background()

	t.Run("successful callback", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.callbackFunc = func(ctx context.Context, state, code string) (*service.CallbackResult, error) {
			return &service.CallbackResult{
				Token:       "test_token_123",
				ExpiresAt:   60,
				RedirectURL: "https://example.com/dashboard?token=test_token_123&expires_at=60",
			}, nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.CallbackRequest{
			State: "test_state",
			Code:  "test_code",
		}

		resp, err := handler.Callback(ctx, req)
		if err != nil {
			t.Fatalf("Callback failed: %v", err)
		}

		if resp.Token == "" {
			t.Error("Expected token to be returned")
		}
		if resp.ExpiresAt != 60 {
			t.Errorf("Expected expires_at 60, got %d", resp.ExpiresAt)
		}
		if resp.RedirectUrl == "" {
			t.Error("Expected redirect URL to be returned")
		}
	})

	t.Run("callback with invalid state", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.callbackFunc = func(ctx context.Context, state, code string) (*service.CallbackResult, error) {
			return nil, errors.New("invalid state value: state not found or already consumed")
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.CallbackRequest{
			State: "invalid_state",
			Code:  "test_code",
		}

		_, err := handler.Callback(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_GetMe(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get me", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.getMeFunc = func(ctx context.Context, token string) (*service.UserDetails, error) {
			return &service.UserDetails{
				ID:              1,
				Name:            "Test User",
				Token:           token,
				AutomaticLogout: 60,
				Code:            "USER123",
				UnreadNotificationsCount: 5,
			}, nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.GetMeRequest{
			Token: "valid_token",
		}

		resp, err := handler.GetMe(ctx, req)
		if err != nil {
			t.Fatalf("GetMe failed: %v", err)
		}

		if resp.Id != 1 {
			t.Errorf("Expected user ID 1, got %d", resp.Id)
		}
		if resp.Name != "Test User" {
			t.Errorf("Expected name 'Test User', got %q", resp.Name)
		}
	})

	t.Run("get me with invalid token", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.getMeFunc = func(ctx context.Context, token string) (*service.UserDetails, error) {
			return nil, errors.New("invalid token")
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.GetMeRequest{
			Token: "invalid_token",
		}

		_, err := handler.GetMe(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected Unauthenticated error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("successful logout", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.logoutFunc = func(ctx context.Context, userID uint64, ip, userAgent string) error {
			return nil
		}

		tokenRepo := &mockTokenRepository{}
		tokenRepo.validateTokenFunc = func(ctx context.Context, token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		}

		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.LogoutRequest{
			Token: "valid_token",
		}

		_, err := handler.Logout(ctx, req)
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
	})

	t.Run("logout with invalid token", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		tokenRepo := &mockTokenRepository{}
		tokenRepo.validateTokenFunc = func(ctx context.Context, token string) (*models.User, error) {
			return nil, errors.New("invalid token")
		}

		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.LogoutRequest{
			Token: "invalid_token",
		}

		_, err := handler.Logout(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected Unauthenticated error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_ValidateToken(t *testing.T) {
	ctx := context.Background()

	t.Run("successful token validation", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.validateTokenFunc = func(ctx context.Context, token string) (*models.User, error) {
			return &models.User{ID: 1, Email: "test@example.com"}, nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.ValidateTokenRequest{
			Token: "valid_token",
		}

		resp, err := handler.ValidateToken(ctx, req)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}

		if !resp.Valid {
			t.Error("Expected token to be valid")
		}
		if resp.UserId != 1 {
			t.Errorf("Expected user ID 1, got %d", resp.UserId)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.validateTokenFunc = func(ctx context.Context, token string) (*models.User, error) {
			return nil, errors.New("invalid token")
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.ValidateTokenRequest{
			Token: "invalid_token",
		}

		resp, err := handler.ValidateToken(ctx, req)
		if err != nil {
			t.Fatalf("ValidateToken should not return error for invalid token: %v", err)
		}

		if resp.Valid {
			t.Error("Expected token to be invalid")
		}
	})
}

func TestAuthHandler_RequestAccountSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("successful request", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err != nil {
			t.Fatalf("RequestAccountSecurity failed: %v", err)
		}
	})

	t.Run("invalid unlock duration", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrInvalidUnlockDuration
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 3, // Below minimum
			Phone:       "09123456789",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("phone required when not verified", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrPhoneRequired
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("invalid phone format", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrInvalidPhoneFormat
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "123456",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("phone already taken", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrPhoneAlreadyTaken
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrUserNotFound
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.RequestAccountSecurityRequest{
			UserId:      999,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := handler.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_VerifyAccountSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("successful verification", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return nil
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "192.168.1.1",
			UserAgent: "Mozilla/5.0",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err != nil {
			t.Fatalf("VerifyAccountSecurity failed: %v", err)
		}
	})

	t.Run("invalid OTP code format", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrInvalidOTPCode
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "abc123",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("invalid OTP code - wrong value", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrInvalidOTPCode
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "000000",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("account security not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrAccountSecurityNotFound
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("account security already unlocked", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrAccountSecurityAlreadyUnlocked
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.FailedPrecondition {
			t.Errorf("Expected FailedPrecondition error code, got %v", st.Code())
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrUserNotFound
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    999,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("internal service error", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return errors.New("database connection failed")
		}

		tokenRepo := &mockTokenRepository{}
		handler := &authHandler{
			authService: mockAuthService,
			tokenRepo:   tokenRepo,
		}

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := handler.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Internal {
			t.Errorf("Expected Internal error code, got %v", st.Code())
		}
	})
}

// --- Mock implementations ---

type mockAuthService struct {
	registerFunc               func(context.Context, string, string) (string, error)
	redirectFunc               func(context.Context, string, string) (string, string, error)
	callbackFunc               func(context.Context, string, string) (*service.CallbackResult, error)
	getMeFunc                  func(context.Context, string) (*service.UserDetails, error)
	logoutFunc                 func(context.Context, uint64, string, string) error
	validateTokenFunc          func(context.Context, string) (*models.User, error)
	requestAccountSecurityFunc func(context.Context, uint64, int32, string) error
	verifyAccountSecurityFunc  func(context.Context, uint64, string, string, string) error
}

func (m *mockAuthService) Register(ctx context.Context, backURL, referral string) (string, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, backURL, referral)
	}
	return "", nil
}

func (m *mockAuthService) Redirect(ctx context.Context, redirectTo, backURL string) (string, string, error) {
	if m.redirectFunc != nil {
		return m.redirectFunc(ctx, redirectTo, backURL)
	}
	return "", "", nil
}

func (m *mockAuthService) Callback(ctx context.Context, state, code string) (*service.CallbackResult, error) {
	if m.callbackFunc != nil {
		return m.callbackFunc(ctx, state, code)
	}
	return nil, nil
}

func (m *mockAuthService) GetMe(ctx context.Context, token string) (*service.UserDetails, error) {
	if m.getMeFunc != nil {
		return m.getMeFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockAuthService) Logout(ctx context.Context, userID uint64, ip, userAgent string) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, userID, ip, userAgent)
	}
	return nil
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	if m.validateTokenFunc != nil {
		return m.validateTokenFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockAuthService) RequestAccountSecurity(ctx context.Context, userID uint64, minutes int32, phone string) error {
	if m.requestAccountSecurityFunc != nil {
		return m.requestAccountSecurityFunc(ctx, userID, minutes, phone)
	}
	return nil
}

func (m *mockAuthService) VerifyAccountSecurity(ctx context.Context, userID uint64, code, ip, userAgent string) error {
	if m.verifyAccountSecurityFunc != nil {
		return m.verifyAccountSecurityFunc(ctx, userID, code, ip, userAgent)
	}
	return nil
}

var _ service.AuthService = (*mockAuthService)(nil)

type mockTokenRepository struct {
	validateTokenFunc func(context.Context, string) (*models.User, error)
}

func (m *mockTokenRepository) Create(ctx context.Context, userID uint64, name string, expiresAt time.Time) (string, error) {
	return "", nil
}

func (m *mockTokenRepository) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	if m.validateTokenFunc != nil {
		return m.validateTokenFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockTokenRepository) DeleteUserTokens(ctx context.Context, userID uint64) error {
	return nil
}

func (m *mockTokenRepository) FindTokenByHash(ctx context.Context, tokenHash string) (*models.PersonalAccessToken, error) {
	return nil, nil
}

var _ repository.TokenRepository = (*mockTokenRepository)(nil)
