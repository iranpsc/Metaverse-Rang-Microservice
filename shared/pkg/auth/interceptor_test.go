package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type stubValidator struct {
	called bool
}

func (s *stubValidator) ValidateToken(ctx context.Context, token string) (*UserContext, error) {
	s.called = true
	return &UserContext{UserID: 42, Token: token}, nil
}

func TestContextWithOptionalAuth_NoHeader(t *testing.T) {
	validator := &stubValidator{}
	ctx := contextWithOptionalAuth(context.Background(), validator)

	if validator.called {
		t.Fatal("expected validator not to be called without authorization header")
	}
	if _, err := GetUserFromContext(ctx); err == nil {
		t.Fatal("expected no user in context without token")
	}
}

func TestContextWithOptionalAuth_ValidHeader(t *testing.T) {
	validator := &stubValidator{}
	md := metadata.Pairs("authorization", "Bearer test-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = contextWithOptionalAuth(ctx, validator)

	if !validator.called {
		t.Fatal("expected validator to be called with authorization header")
	}
	user, err := GetUserFromContext(ctx)
	if err != nil {
		t.Fatalf("expected user in context: %v", err)
	}
	if user.UserID != 42 {
		t.Fatalf("expected user id 42, got %d", user.UserID)
	}
}

func TestUnaryServerInterceptor_OptionalAuthWithoutToken(t *testing.T) {
	validator := &stubValidator{}
	interceptor := UnaryServerInterceptor(validator)
	called := false

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/features.FeatureService/ListFeatures"}
	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if validator.called {
		t.Fatal("expected validator not to be called without token on optional route")
	}
}

func TestUnaryServerInterceptor_SkipAuthFeatureTradeHistory(t *testing.T) {
	validator := &stubValidator{}
	interceptor := UnaryServerInterceptor(validator)
	called := false

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/features.FeatureService/GetFeatureTradeHistory"}
	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called without auth")
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if validator.called {
		t.Fatal("expected validator not to be called on public trade history route")
	}
}

func TestUnaryServerInterceptor_SkipAuthCompletedBuildings(t *testing.T) {
	validator := &stubValidator{}
	interceptor := UnaryServerInterceptor(validator)
	called := false

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/features.BuildingService/ListCompletedBuildings"}
	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called without auth")
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if validator.called {
		t.Fatal("expected validator not to be called on public completed buildings route")
	}
}

func TestUnaryServerInterceptor_SkipAuthCitizenFeatures(t *testing.T) {
	methods := []string{
		"/features.CitizenFeaturesService/GetCitizenFeatureSummary",
		"/features.CitizenFeaturesService/GetCitizenFeatureChart",
		"/features.CitizenFeaturesService/ListCitizenFeatures",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			validator := &stubValidator{}
			interceptor := UnaryServerInterceptor(validator)
			called := false

			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return "ok", nil
			}

			info := &grpc.UnaryServerInfo{FullMethod: method}
			resp, err := interceptor(context.Background(), nil, info, handler)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Fatal("expected handler to be called without auth")
			}
			if resp != "ok" {
				t.Fatalf("unexpected response: %v", resp)
			}
			if validator.called {
				t.Fatal("expected validator not to be called on public citizen features route")
			}
		})
	}
}

func TestUnaryServerInterceptor_SkipAuthWalletHistory(t *testing.T) {
	methods := []string{
		"/commercial.WalletHistoryService/GetWalletHistorySummary",
		"/commercial.WalletHistoryService/GetWalletHistoryChart",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			validator := &stubValidator{}
			interceptor := UnaryServerInterceptor(validator)
			called := false

			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return "ok", nil
			}

			info := &grpc.UnaryServerInfo{FullMethod: method}
			resp, err := interceptor(context.Background(), nil, info, handler)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Fatal("expected handler to be called without auth")
			}
			if resp != "ok" {
				t.Fatalf("unexpected response: %v", resp)
			}
			if validator.called {
				t.Fatal("expected validator not to be called on public wallet history route")
			}
		})
	}
}
