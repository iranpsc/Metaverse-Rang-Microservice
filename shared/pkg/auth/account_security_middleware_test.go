package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "metarang/shared/pkg/auth"
)

type stubAccountSecurityChecker struct {
	unlocked bool
	err      error
	calls    int
}

func (s *stubAccountSecurityChecker) CheckAccountSecurity(context.Context, uint64) (bool, error) {
	s.calls++
	return s.unlocked, s.err
}

func TestAccountSecurityMiddleware_Returns410WhenLocked(t *testing.T) {
	checker := &stubAccountSecurityChecker{unlocked: false}
	handler := authpkg.AccountSecurityMiddleware(checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: 9}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusGone {
		t.Fatalf("status=%d", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["message"] == "" {
		t.Fatalf("expected message, body=%v", body)
	}
	if checker.calls != 1 {
		t.Fatalf("calls=%d", checker.calls)
	}
}

func TestAccountSecurityMiddleware_AllowsWalletLoginWithoutUnlock(t *testing.T) {
	checker := &stubAccountSecurityChecker{unlocked: false}
	called := false
	handler := authpkg.AccountSecurityMiddleware(checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{
		UserID:      9,
		WalletLogin: true,
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated || !called {
		t.Fatalf("status=%d called=%v", rr.Code, called)
	}
	if checker.calls != 0 {
		t.Fatalf("expected wallet login to skip account security check, calls=%d", checker.calls)
	}
}

func TestAccountSecurityMiddleware_AllowsUnlockedMutations(t *testing.T) {
	checker := &stubAccountSecurityChecker{unlocked: true}
	called := false
	handler := authpkg.AccountSecurityMiddleware(checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: 9}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated || !called {
		t.Fatalf("status=%d called=%v", rr.Code, called)
	}
}
