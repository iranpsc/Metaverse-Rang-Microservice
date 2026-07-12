package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"

	pb "metargb/shared/pb/auth"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newAccountSecurityAuthHandler(t *testing.T, auth *testutil.MockAuthService) *handler.AuthHandler {
	t.Helper()
	conn, cleanup := testutil.DialAuthConn(&testutil.AuthMocks{Auth: auth})
	t.Cleanup(cleanup)
	return handler.NewAuthHandler(conn, nil, "en")
}

func TestRequestAccountSecurity_FlexibleInt32(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMin int32
	}{
		{name: "integer", body: `{"time":15}`, wantMin: 15},
		{name: "string", body: `{"time":"15"}`, wantMin: 15},
		{name: "persian string", body: `{"time":"۱۵"}`, wantMin: 15},
		{name: "float", body: `{"time":15.0}`, wantMin: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int32
			auth := &testutil.MockAuthService{}
			auth.RequestAccountSecurityFunc = func(_ context.Context, req *pb.RequestAccountSecurityRequest) (*emptypb.Empty, error) {
				got = req.TimeMinutes
				return &emptypb.Empty{}, nil
			}
			h := newAccountSecurityAuthHandler(t, auth)

			req := httptest.NewRequest(http.MethodPost, "/api/account/security", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			req = testutil.RequestWithUser(req, 1)
			w := httptest.NewRecorder()
			h.RequestAccountSecurity(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if got != tt.wantMin {
				t.Fatalf("TimeMinutes=%d, want %d", got, tt.wantMin)
			}
		})
	}
}

func TestRequestAccountSecurity_NullTimeValidation(t *testing.T) {
	auth := &testutil.MockAuthService{}
	h := newAccountSecurityAuthHandler(t, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/account/security", bytes.NewReader([]byte(`{"time":null}`)))
	req.Header.Set("Content-Type", "application/json")
	req = testutil.RequestWithUser(req, 1)
	w := httptest.NewRecorder()
	h.RequestAccountSecurity(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected validation error for null time")
	}
}
