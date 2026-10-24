package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"

	pb "metarang/shared/pb/auth"
)

type stubAuthClient struct {
	pb.AuthServiceClient
	response *pb.ValidateTokenResponse
	err      error
}

func (s stubAuthClient) ValidateToken(_ context.Context, _ *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	return s.response, s.err
}

func TestValidateTokenGRPC(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *pb.ValidateTokenResponse
		want     uint64
		fails    bool
	}{
		{"valid", &pb.ValidateTokenResponse{Valid: true, UserId: 42}, 42, false},
		{"invalid", &pb.ValidateTokenResponse{Valid: false}, 0, true},
		{"nil response", nil, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &Validator{client: stubAuthClient{response: tc.response}}
			id, err := v.ValidateToken(context.Background(), "token")
			if id != tc.want || (err != nil) != tc.fails {
				t.Fatalf("id=%d err=%v", id, err)
			}
		})
	}
}

func TestValidateTokenHTTPFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   uint64
		fails  bool
	}{
		{"nested", 200, `{"data":{"valid":true,"user_id":7}}`, 7, false},
		{"flat", 200, `{"valid":true,"user_id":9}`, 9, false},
		{"denied", 403, `denied`, 0, true},
		{"invalid json", 200, `{`, 0, true},
		{"invalid token", 200, `{"valid":false,"user_id":9}`, 0, true},
		{"missing user", 200, `{"valid":true}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/auth/validate" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			v := &Validator{client: stubAuthClient{err: errors.New("grpc unavailable")}, httpClient: srv.Client(), httpBase: srv.URL}
			id, err := v.ValidateToken(context.Background(), "token")
			if id != tc.want || (err != nil) != tc.fails {
				t.Fatalf("id=%d err=%v", id, err)
			}
			if err != nil && !strings.Contains(err.Error(), "grpc unavailable") {
				t.Fatalf("missing grpc cause: %v", err)
			}
		})
	}
	v := &Validator{client: stubAuthClient{err: errors.New("grpc unavailable")}}
	if _, err := v.ValidateToken(context.Background(), "token"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("fallback error=%v", err)
	}
}

func TestValidatorURLDefaults(t *testing.T) {
	for _, tc := range []struct{ address, want string }{
		{"auth-service:50051", "http://auth-service:8066"},
		{"", "http://auth-service:8066"},
		{"localhost", "http://localhost:8066"},
	} {
		if got := defaultHTTPBase(tc.address); got != tc.want {
			t.Fatalf("address=%q got=%q", tc.address, got)
		}
	}
	t.Setenv("AUTH_SERVICE_HTTP_URL", "http://custom")
	if got := envOr("AUTH_SERVICE_HTTP_URL", "fallback"); got != "http://custom" {
		t.Fatal(got)
	}
	if got := envOr("UNSET_VALIDATOR_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatal(got)
	}
}
