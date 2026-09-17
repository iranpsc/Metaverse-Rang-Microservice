package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/auth-service/internal/middleware"
	"metarang/auth-service/internal/pubsub"
	sharedauth "metarang/shared/pkg/auth"
)

type lastSeenUserRepo struct {
	updatedIDs []uint64
	err        error
}

func (r *lastSeenUserRepo) UpdateLastSeen(_ context.Context, userID uint64) error {
	r.updatedIDs = append(r.updatedIDs, userID)
	return r.err
}

var _ middleware.LastSeenUpdater = (*lastSeenUserRepo)(nil)

type lastSeenPublisher struct {
	calls []struct {
		userID uint64
		online bool
	}
	err error
}

func (p *lastSeenPublisher) PublishUserStatusChanged(_ context.Context, userID uint64, online bool) error {
	p.calls = append(p.calls, struct {
		userID uint64
		online bool
	}{userID: userID, online: online})
	return p.err
}

func (p *lastSeenPublisher) Close() error { return nil }

var _ pubsub.RedisPublisher = (*lastSeenPublisher)(nil)

func TestLastSeenMiddleware(t *testing.T) {
	t.Run("updates last_seen and publishes online when authenticated", func(t *testing.T) {
		repo := &lastSeenUserRepo{}
		pub := &lastSeenPublisher{}
		nextCalled := false

		authMW := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{UserID: 42})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
		h := middleware.WithLastSeen(authMW, middleware.LastSeenMiddleware(repo, pub))(
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
		if len(repo.updatedIDs) != 1 || repo.updatedIDs[0] != 42 {
			t.Fatalf("expected UpdateLastSeen(42), got %v", repo.updatedIDs)
		}
		if len(pub.calls) != 1 || pub.calls[0].userID != 42 || !pub.calls[0].online {
			t.Fatalf("expected online publish for 42, got %+v", pub.calls)
		}
	})

	t.Run("skips when unauthenticated", func(t *testing.T) {
		repo := &lastSeenUserRepo{}
		pub := &lastSeenPublisher{}
		h := middleware.LastSeenMiddleware(repo, pub)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if len(repo.updatedIDs) != 0 || len(pub.calls) != 0 {
			t.Fatalf("expected no side effects, updates=%v publishes=%v", repo.updatedIDs, pub.calls)
		}
	})

	t.Run("still serves request when update/publish fail", func(t *testing.T) {
		repo := &lastSeenUserRepo{err: errors.New("db down")}
		pub := &lastSeenPublisher{err: errors.New("redis down")}
		h := middleware.LastSeenMiddleware(repo, pub)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{UserID: 7}))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("nil repo and publisher are safe", func(t *testing.T) {
		h := middleware.LastSeenMiddleware(nil, nil)(
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
