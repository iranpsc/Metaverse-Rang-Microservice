package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/auth-service/internal/middleware"
	sharedauth "metarang/shared/pkg/auth"
)

type recordingPresence struct {
	ids []uint64
}

func (r *recordingPresence) Touch(userID uint64) {
	r.ids = append(r.ids, userID)
}

func TestLastSeenMiddleware(t *testing.T) {
	t.Run("touches presence when authenticated", func(t *testing.T) {
		tracker := &recordingPresence{}
		nextCalled := false

		authMW := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{UserID: 42})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
		h := middleware.WithLastSeen(authMW, middleware.LastSeenMiddleware(tracker))(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}),
		)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

		if !nextCalled || rr.Code != http.StatusOK {
			t.Fatalf("nextCalled=%v status=%d", nextCalled, rr.Code)
		}
		if len(tracker.ids) != 1 || tracker.ids[0] != 42 {
			t.Fatalf("expected Touch(42), got %v", tracker.ids)
		}
	})

	t.Run("skips when unauthenticated", func(t *testing.T) {
		tracker := &recordingPresence{}
		h := middleware.LastSeenMiddleware(tracker)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if len(tracker.ids) != 0 {
			t.Fatalf("expected no touch, got %v", tracker.ids)
		}
	})

	t.Run("nil tracker is safe", func(t *testing.T) {
		h := middleware.LastSeenMiddleware(nil)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{UserID: 1}))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
	})
}
