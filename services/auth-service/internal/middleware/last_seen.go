package middleware

import (
	"context"
	"log"
	"net/http"

	"metarang/auth-service/internal/pubsub"
	authpkg "metarang/shared/pkg/auth"
)

// LastSeenUpdater updates the users.last_seen column.
type LastSeenUpdater interface {
	UpdateLastSeen(ctx context.Context, userID uint64) error
}

// LastSeenMiddleware updates users.last_seen and broadcasts an online user-status
// event for authenticated requests. Mirrors Laravel's Activity middleware.
// Must run after AuthMiddleware so the user context is already set.
func LastSeenMiddleware(updater LastSeenUpdater, publisher pubsub.RedisPublisher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, err := authpkg.GetUserFromContext(r.Context()); err == nil && user != nil && user.UserID > 0 {
				if updater != nil {
					if err := updater.UpdateLastSeen(r.Context(), user.UserID); err != nil {
						log.Printf("last_seen middleware: update failed user=%d: %v", user.UserID, err)
					}
				}
				if publisher != nil {
					if err := publisher.PublishUserStatusChanged(r.Context(), user.UserID, true); err != nil {
						log.Printf("last_seen middleware: publish failed user=%d: %v", user.UserID, err)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithLastSeen composes auth then last-seen so protected routes track activity.
// Request order: AuthMiddleware → LastSeenMiddleware → handler.
func WithLastSeen(authMW, lastSeenMW func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return authMW(lastSeenMW(next))
	}
}
