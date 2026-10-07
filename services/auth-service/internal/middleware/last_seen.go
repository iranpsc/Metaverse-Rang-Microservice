package middleware

import (
	"net/http"

	authpkg "metarang/shared/pkg/auth"
)

// PresenceToucher records that an authenticated user is active.
type PresenceToucher interface {
	Touch(userID uint64)
}

// LastSeenMiddleware records activity for every authenticated request and publishes
// an online user-status event. Must run after AuthMiddleware so the user context is already set.
func LastSeenMiddleware(tracker PresenceToucher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tracker != nil {
				if user, err := authpkg.GetUserFromContext(r.Context()); err == nil && user != nil && user.UserID > 0 {
					tracker.Touch(user.UserID)
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
