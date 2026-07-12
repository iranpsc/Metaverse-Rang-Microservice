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

func newVerifySecurityAuthHandler(t *testing.T, auth *testutil.MockAuthService) *handler.AuthHandler {
	t.Helper()
	conn, cleanup := testutil.DialAuthConn(&testutil.AuthMocks{Auth: auth})
	t.Cleanup(cleanup)
	return handler.NewAuthHandler(conn, nil, "en")
}

func TestVerifyAccountSecurity_FlexibleStringCode(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		ctype    string
		wantCode string
	}{
		{name: "json string", body: []byte(`{"code":"123456"}`), ctype: "application/json", wantCode: "123456"},
		{name: "json numeric", body: []byte(`{"code":123456}`), ctype: "application/json", wantCode: "123456"},
		{name: "persian string", body: []byte(`{"code":"۱۲۳۴۵۶"}`), ctype: "application/json", wantCode: "123456"},
		{name: "form-urlencoded", body: []byte("code=654321"), ctype: "application/x-www-form-urlencoded", wantCode: "654321"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			auth := &testutil.MockAuthService{}
			auth.VerifyAccountSecurityFunc = func(_ context.Context, req *pb.VerifyAccountSecurityRequest) (*emptypb.Empty, error) {
				got = req.Code
				return &emptypb.Empty{}, nil
			}
			h := newVerifySecurityAuthHandler(t, auth)

			req := httptest.NewRequest(http.MethodPost, "/api/account/security/verify", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.ctype)
			req = testutil.RequestWithUser(req, 1)
			w := httptest.NewRecorder()
			h.VerifyAccountSecurity(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if got != tt.wantCode {
				t.Fatalf("code=%q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestVerifyAccountSecurity_InvalidJSON(t *testing.T) {
	auth := &testutil.MockAuthService{}
	h := newVerifySecurityAuthHandler(t, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/account/security/verify", bytes.NewReader([]byte(`{"code":}`)))
	req.Header.Set("Content-Type", "application/json")
	req = testutil.RequestWithUser(req, 1)
	w := httptest.NewRecorder()
	h.VerifyAccountSecurity(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected decode error for malformed JSON")
	}
}

func TestVerifyAccountSecurity_NullCode(t *testing.T) {
	var got string
	auth := &testutil.MockAuthService{}
	auth.VerifyAccountSecurityFunc = func(_ context.Context, req *pb.VerifyAccountSecurityRequest) (*emptypb.Empty, error) {
		got = req.Code
		return &emptypb.Empty{}, nil
	}
	h := newVerifySecurityAuthHandler(t, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/account/security/verify", bytes.NewReader([]byte(`{"code":null}`)))
	req.Header.Set("Content-Type", "application/json")
	req = testutil.RequestWithUser(req, 1)
	w := httptest.NewRecorder()
	h.VerifyAccountSecurity(w, req)

	if got != "" {
		t.Fatalf("code=%q, want empty for null", got)
	}
}
