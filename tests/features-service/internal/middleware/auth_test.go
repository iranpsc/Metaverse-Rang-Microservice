package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/features-service/internal/middleware"
	pb "metarang/shared/pb/auth"
)

type mockAuthServiceClient struct {
	ValidateTokenFunc         func(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)
	CheckAccountSecurityFunc  func(ctx context.Context, req *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error)
}

func (m *mockAuthServiceClient) Register(context.Context, *pb.RegisterRequest, ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Redirect(context.Context, *pb.RedirectRequest, ...grpc.CallOption) (*pb.RedirectResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Callback(context.Context, *pb.CallbackRequest, ...grpc.CallOption) (*pb.CallbackResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) GetMe(context.Context, *pb.GetMeRequest, ...grpc.CallOption) (*pb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) Logout(context.Context, *pb.LogoutRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	if m.ValidateTokenFunc != nil {
		return m.ValidateTokenFunc(ctx, req)
	}
	return &pb.ValidateTokenResponse{Valid: true, UserId: 42, Email: "user@example.com"}, nil
}
func (m *mockAuthServiceClient) RequestAccountSecurity(context.Context, *pb.RequestAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) VerifyAccountSecurity(context.Context, *pb.VerifyAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthServiceClient) CheckAccountSecurity(ctx context.Context, req *pb.CheckAccountSecurityRequest, _ ...grpc.CallOption) (*pb.CheckAccountSecurityResponse, error) {
	if m.CheckAccountSecurityFunc != nil {
		return m.CheckAccountSecurityFunc(ctx, req)
	}
	return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
}

var _ pb.AuthServiceClient = (*mockAuthServiceClient)(nil)

func TestAuthMiddleware_Success_BearerToken(t *testing.T) {
	auth := &mockAuthServiceClient{
		ValidateTokenFunc: func(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			assert.Equal(t, "good-token", req.Token)
			return &pb.ValidateTokenResponse{Valid: true, UserId: 99, Email: "a@b.com"}, nil
		},
	}
	called := false
	handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		user, err := middleware.GetUserFromRequest(r)
		require.NoError(t, err)
		assert.Equal(t, uint64(99), user.UserID)
		assert.Equal(t, "a@b.com", user.Email)
		assert.Equal(t, "good-token", user.Token)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestAuthMiddleware_CookieToken(t *testing.T) {
	auth := &mockAuthServiceClient{}
	handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := middleware.GetUserFromRequest(r)
		require.NoError(t, err)
		assert.Equal(t, uint64(42), user.UserID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthMiddleware_Unauthenticated(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		handler := middleware.AuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		var body map[string]string
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "Unauthenticated", body["error"])
	})

	t.Run("missing token", func(t *testing.T) {
		handler := middleware.AuthMiddleware(&mockAuthServiceClient{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		auth := &mockAuthServiceClient{
			ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
				return &pb.ValidateTokenResponse{Valid: false}, nil
			},
		}
		handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("validate error", func(t *testing.T) {
		auth := &mockAuthServiceClient{
			ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
				return nil, context.DeadlineExceeded
			},
		}
		handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer x")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestOptionalAuthMiddleware(t *testing.T) {
	t.Run("nil client continues", func(t *testing.T) {
		called := false
		handler := middleware.OptionalAuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_, err := middleware.GetUserFromRequest(r)
			assert.Error(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, called)
	})

	t.Run("without token continues", func(t *testing.T) {
		handler := middleware.OptionalAuthMiddleware(&mockAuthServiceClient{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("with valid token", func(t *testing.T) {
		handler := middleware.OptionalAuthMiddleware(&mockAuthServiceClient{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := middleware.GetUserFromRequest(r)
			require.NoError(t, err)
			assert.Equal(t, uint64(42), user.UserID)
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer cookie-token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid token continues unauthenticated", func(t *testing.T) {
		auth := &mockAuthServiceClient{
			ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
				return &pb.ValidateTokenResponse{Valid: false}, nil
			},
		}
		handler := middleware.OptionalAuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := middleware.GetUserFromRequest(r)
			assert.Error(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestExtractToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, "", middleware.ExtractToken(req))

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	assert.Equal(t, "abc", middleware.ExtractToken(req))

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "from-cookie"})
	assert.Equal(t, "from-cookie", middleware.ExtractToken(req))
}

func TestGetUserFromRequest_MissingContext(t *testing.T) {
	_, err := middleware.GetUserFromRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Error(t, err)
}
