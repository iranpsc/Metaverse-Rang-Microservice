package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/features-service/internal/middleware"
	pb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

func TestAccountSecurityMiddleware_AllowsWhenUnlocked(t *testing.T) {
	auth := &mockAuthServiceClient{
		ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			return &pb.ValidateTokenResponse{Valid: true, UserId: 7, Email: "u@example.com"}, nil
		},
	}
	auth.CheckAccountSecurityFunc = func(_ context.Context, req *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		assert.Equal(t, uint64(7), req.UserId)
		return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
	}

	called := false
	handler := middleware.AuthMiddleware(auth)(middleware.AccountSecurityMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/buy-requests", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, called)
}

func TestAccountSecurityMiddleware_Returns410WhenLocked(t *testing.T) {
	auth := &mockAuthServiceClient{
		ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			return &pb.ValidateTokenResponse{Valid: true, UserId: 7}, nil
		},
	}
	auth.CheckAccountSecurityFunc = func(context.Context, *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		return &pb.CheckAccountSecurityResponse{Unlocked: false}, nil
	}

	called := false
	handler := middleware.AuthMiddleware(auth)(middleware.AccountSecurityMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})))

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(method, "/api/my-features/1", nil)
			req.Header.Set("Authorization", "Bearer token")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusGone, rr.Code)
			assert.False(t, called)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Contains(t, body["message"], "Account security is locked")
		})
	}
}

func TestAccountSecurityMiddleware_AllowsWalletLoginWhenLocked(t *testing.T) {
	auth := &mockAuthServiceClient{
		ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			return &pb.ValidateTokenResponse{Valid: true, UserId: 7, WalletLogin: true}, nil
		},
	}
	auth.CheckAccountSecurityFunc = func(context.Context, *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		t.Fatal("CheckAccountSecurity should not be called for wallet login")
		return nil, nil
	}

	called := false
	handler := middleware.AuthMiddleware(auth)(middleware.AccountSecurityMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/buy-requests", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, called)
}

func TestAccountSecurityMiddleware_SkipsSafeMethods(t *testing.T) {
	auth := &mockAuthServiceClient{}
	auth.CheckAccountSecurityFunc = func(context.Context, *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		t.Fatal("CheckAccountSecurity should not be called for GET")
		return nil, nil
	}

	handler := middleware.AccountSecurityMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: 1})
		_ = ctx
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/my-features", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}
