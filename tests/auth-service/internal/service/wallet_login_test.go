package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
)

func TestCallbackAndGetMe_WalletLoginPersistsOnToken(t *testing.T) {
	ctx := context.Background()
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "mock_access_token",
				"refresh_token": "mock_refresh_token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		case r.URL.Path == "/api/user" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name":     "Test User",
				"email":    "wallet@example.com",
				"mobile":   "09123456789",
				"code":     "USER123",
				"referral": "",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer oauthServer.Close()

	cases := []struct {
		name        string
		walletLogin bool
	}{
		{name: "wallet login true", walletLogin: true},
		{name: "wallet login false", walletLogin: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := make(map[uint64]*models.User)
			userRepo := &extendedFakeUserRepository{
				fakeUserRepository: newFakeUserRepository(users),
			}
			userRepo.findByEmailFunc = func(context.Context, string) (*models.User, error) {
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

			tokenRepo := newFakeTokenRepository()
			cacheRepo := newFakeCacheRepository()
			state := "wallet_login_state"
			require.NoError(t, cacheRepo.SetState(ctx, state, 5*time.Minute))
			require.NoError(t, cacheRepo.SetRedirectTo(ctx, state, "https://example.com/app", 5*time.Minute))

			svc := service.NewAuthService(
				userRepo, tokenRepo, cacheRepo,
				nil, nil, nil,
				oauthServer.URL,
				"test-client-id",
				"test-client-secret",
				"http://localhost:8000",
				"http://localhost:3000",
			)

			result, err := svc.Callback(ctx, state, "test_code", "127.0.0.1", tc.walletLogin)
			require.NoError(t, err)
			require.NotEmpty(t, result.Token)
			require.Equal(t, tc.walletLogin, tokenRepo.walletLogin[result.Token])

			first, err := svc.GetMe(ctx, result.Token)
			require.NoError(t, err)
			require.Equal(t, tc.walletLogin, first.WalletLogin)

			second, err := svc.GetMe(ctx, result.Token)
			require.NoError(t, err)
			require.Equal(t, tc.walletLogin, second.WalletLogin)
			require.Equal(t, first.WalletLogin, second.WalletLogin)
		})
	}
}

func TestGetMe_WalletLoginIndependentOfHasWallet(t *testing.T) {
	ctx := context.Background()
	users := map[uint64]*models.User{
		1: {
			ID: 1, Name: "n", Email: "a@x.com", Code: "hm-1",
		},
	}
	userRepo := newFakeUserRepository(users)
	userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
		return &models.Settings{UserID: userID, AutomaticLogout: 45}, nil
	}
	userRepo.getKYCFunc = func(context.Context, uint64) (*models.KYC, error) { return nil, nil }
	userRepo.getUnreadNotificationsCountFunc = func(context.Context, uint64) (int32, error) { return 0, nil }

	tokenRepo := newFakeTokenRepository()
	tokenRepo.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return users[1], nil
	}
	tokenRepo.walletLogin["tok"] = true

	svc := service.NewAuthService(
		userRepo, tokenRepo, newFakeCacheRepository(),
		nil, nil, nil,
		"", "", "", "", "",
	)

	details, err := svc.GetMe(ctx, "tok")
	require.NoError(t, err)
	require.False(t, details.HasWallet)
	require.True(t, details.WalletLogin)
}
