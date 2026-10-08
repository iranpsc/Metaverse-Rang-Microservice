// Package middleware provides HTTP authentication for buildings-service.
package middleware

import (
	"context"
	"net/http"
	"strings"

	authpb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

func AuthMiddleware(client authpb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client == nil {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			user, ok := userFromValidatedToken(r, client)
			if !ok {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(client authpb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client != nil {
				if user, ok := userFromValidatedToken(r, client); ok {
					r = r.WithContext(context.WithValue(r.Context(), authpkg.UserContextKey{}, user))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func userFromValidatedToken(r *http.Request, client authpb.AuthServiceClient) (*authpkg.UserContext, bool) {
	token := ExtractToken(r)
	if token == "" {
		return nil, false
	}
	response, err := client.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil || !response.Valid {
		return nil, false
	}
	return authpkg.UserContextFromValidateToken(response, token), true
}

func GetUserFromRequest(r *http.Request) (*authpkg.UserContext, error) {
	return authpkg.GetUserFromContext(r.Context())
}

// RejectCookieCSRF reports whether a state-changing request authenticated only
// by the token cookie is missing a header that cross-site form posts cannot set.
func RejectCookieCSRF(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	if _, err := r.Cookie("token"); err != nil {
		return false
	}
	if r.Header.Get("X-CSRF-TOKEN") != "" || r.Header.Get("X-XSRF-TOKEN") != "" || r.Header.Get("X-Requested-With") != "" {
		return false
	}
	return true
}

func ExtractToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if value == "" {
		if cookie, err := r.Cookie("token"); err == nil {
			return cookie.Value
		}
		return ""
	}
	return strings.TrimPrefix(value, "Bearer ")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
