package handler_test

import (
	"context"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
)

type mockAuthService struct {
	registerFunc               func(context.Context, string, string) (string, error)
	redirectFunc               func(context.Context, string, string) (string, string, error)
	callbackFunc               func(context.Context, string, string, string) (*service.CallbackResult, error)
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

func (m *mockAuthService) Callback(ctx context.Context, state, code, ip string) (*service.CallbackResult, error) {
	if m.callbackFunc != nil {
		return m.callbackFunc(ctx, state, code, ip)
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
