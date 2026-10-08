package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "metarang/shared/pb/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/notifications-service/internal/resilience"
)

type stubAuthClient struct {
	pb.AuthServiceClient
	fn func(ctx context.Context, in *pb.ValidateTokenRequest, opts ...grpc.CallOption) (*pb.ValidateTokenResponse, error)
}

func (s stubAuthClient) ValidateToken(ctx context.Context, in *pb.ValidateTokenRequest, opts ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	return s.fn(ctx, in, opts...)
}

func TestAuthMiddleware_RetriesUnavailableThenSucceeds(t *testing.T) {
	var calls int
	auth := stubAuthClient{fn: func(context.Context, *pb.ValidateTokenRequest, ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
		calls++
		if calls == 1 {
			return nil, status.Error(codes.Unavailable, "down")
		}
		return &pb.ValidateTokenResponse{Valid: true, UserId: 7}, nil
	}}

	handler := AuthMiddlewareWithPolicy(auth, AuthPolicy{
		Timeout:  time.Second,
		Attempts: 2,
		Backoff:  time.Millisecond,
		Breaker:  resilience.NewBreaker(5, time.Minute),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestAuthMiddleware_DoesNotRetryRejectedToken(t *testing.T) {
	var calls int
	auth := stubAuthClient{fn: func(context.Context, *pb.ValidateTokenRequest, ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
		calls++
		return nil, status.Error(codes.Unauthenticated, "bad")
	}}

	handler := AuthMiddlewareWithPolicy(auth, AuthPolicy{
		Timeout:  time.Second,
		Attempts: 3,
		Backoff:  time.Second,
		Breaker:  resilience.NewBreaker(5, time.Minute),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestAuthMiddleware_TimeoutReturnsServiceUnavailable(t *testing.T) {
	auth := stubAuthClient{fn: func(ctx context.Context, _ *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}

	handler := AuthMiddlewareWithPolicy(auth, AuthPolicy{
		Timeout:  30 * time.Millisecond,
		Attempts: 1,
		Backoff:  time.Millisecond,
		Breaker:  resilience.NewBreaker(5, time.Minute),
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rr, req)
	if time.Since(start) > time.Second {
		t.Fatal("validate token did not honor the deadline")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_OpenBreakerSkipsAuthCall(t *testing.T) {
	var calls int
	auth := stubAuthClient{fn: func(context.Context, *pb.ValidateTokenRequest, ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
		calls++
		return nil, status.Error(codes.Unavailable, "down")
	}}
	policy := AuthPolicy{
		Timeout:  time.Second,
		Attempts: 1,
		Backoff:  time.Millisecond,
		Breaker:  resilience.NewBreaker(1, time.Minute),
	}
	handler := AuthMiddlewareWithPolicy(auth, policy)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer tok")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("request %d status=%d", i, rr.Code)
		}
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
