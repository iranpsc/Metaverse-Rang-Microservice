package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// AccountSecurityChecker reports whether a user's account security unlock window is active.
type AccountSecurityChecker interface {
	CheckAccountSecurity(ctx context.Context, userID uint64) (unlocked bool, err error)
}

// AccountSecurityMiddleware blocks POST, PUT, and DELETE when account security is locked.
// Sessions created via crypto-wallet login skip the unlock check.
// Locked requests receive HTTP 410 Gone with a JSON message for the unlock UI.
func AccountSecurityMiddleware(checker AccountSecurityChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}
			userCtx, err := GetUserFromContext(r.Context())
			if err != nil {
				writeAccountSecurityJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "Unauthenticated",
				})
				return
			}
			if userCtx.WalletLogin {
				next.ServeHTTP(w, r)
				return
			}
			if checker == nil {
				writeAccountSecurityJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "account security checker is not configured",
				})
				return
			}
			unlocked, err := checker.CheckAccountSecurity(r.Context(), userCtx.UserID)
			if err != nil {
				writeAccountSecurityJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to check account security",
				})
				return
			}
			if !unlocked {
				writeAccountSecurityJSON(w, http.StatusGone, map[string]string{
					"message": "Account security is locked. Unlock your account security to continue.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAccountSecurityJSON(w http.ResponseWriter, statusCode int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
