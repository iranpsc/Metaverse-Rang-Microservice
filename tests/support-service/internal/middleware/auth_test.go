package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metarang/shared/pb/auth"
	"metarang/support-service/internal/middleware"
)

type mockAuthClient struct {
	ValidateTokenFunc func(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)
}

func (m *mockAuthClient) Register(context.Context, *pb.RegisterRequest, ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) Redirect(context.Context, *pb.RedirectRequest, ...grpc.CallOption) (*pb.RedirectResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) Callback(context.Context, *pb.CallbackRequest, ...grpc.CallOption) (*pb.CallbackResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) GetMe(context.Context, *pb.GetMeRequest, ...grpc.CallOption) (*pb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) Logout(context.Context, *pb.LogoutRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthClient) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	if m.ValidateTokenFunc != nil {
		return m.ValidateTokenFunc(ctx, req)
	}
	return &pb.ValidateTokenResponse{Valid: true, UserId: 42, Email: "user@example.com"}, nil
}
func (m *mockAuthClient) RequestAccountSecurity(context.Context, *pb.RequestAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthClient) VerifyAccountSecurity(context.Context, *pb.VerifyAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockAuthClient) CheckAccountSecurity(context.Context, *pb.CheckAccountSecurityRequest, ...grpc.CallOption) (*pb.CheckAccountSecurityResponse, error) {
	return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
}

func TestAuthMiddleware_RawAuthorizationHeaderWithoutBearer(t *testing.T) {
	auth := &mockAuthClient{
		ValidateTokenFunc: func(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			if req.Token != "raw-token" {
				t.Fatalf("token=%q", req.Token)
			}
			return &pb.ValidateTokenResponse{Valid: true, UserId: 7, Email: "a@b.com"}, nil
		},
	}
	handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := middleware.GetUserFromRequest(r)
		if err != nil || user.UserID != 7 || user.Token != "raw-token" {
			t.Fatalf("user=%+v err=%v", user, err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "raw-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestGetUserFromRequest_MissingContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := middleware.GetUserFromRequest(req)
	if err == nil {
		t.Fatal("expected missing user error")
	}
}

func TestAuthMiddleware_Success(t *testing.T) {
	auth := &mockAuthClient{
		ValidateTokenFunc: func(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
			if req.Token != "good-token" {
				t.Fatalf("token=%q", req.Token)
			}
			return &pb.ValidateTokenResponse{Valid: true, UserId: 99, Email: "a@b.com"}, nil
		},
	}
	mw := middleware.AuthMiddleware(auth)
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		user, err := middleware.GetUserFromRequest(r)
		if err != nil {
			t.Fatal(err)
		}
		if user.UserID != 99 || user.Token != "good-token" {
			t.Fatalf("user=%+v", user)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !called {
		t.Fatalf("code=%d called=%v", rr.Code, called)
	}
}

func TestAuthMiddleware_CookieToken(t *testing.T) {
	auth := &mockAuthClient{}
	mw := middleware.AuthMiddleware(auth)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := middleware.GetUserFromRequest(r)
		if err != nil || user.UserID != 42 {
			t.Fatal(err, user)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestAuthMiddleware_Unauthenticated(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		handler := middleware.AuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		handler := middleware.AuthMiddleware(&mockAuthClient{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		auth := &mockAuthClient{
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
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("validate error", func(t *testing.T) {
		auth := &mockAuthClient{
			ValidateTokenFunc: func(context.Context, *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
				return nil, errors.New("auth down")
			},
		}
		handler := middleware.AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer x")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})
}
