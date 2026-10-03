// Package middleware provides HTTP authentication middleware for financial-service.
package middleware

import (
	"context"
	"net/http"
	"strings"

	pb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

// AuthMiddleware validates a Bearer/cookie token via auth-service and injects user context.
func AuthMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx, ok := authenticatedUser(r, authClient)
			if !ok {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware validates a token when present; invalid/missing tokens are ignored.
func OptionalAuthMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userCtx, ok := authenticatedUser(r, authClient); ok {
				ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authenticatedUser(r *http.Request, authClient pb.AuthServiceClient) (*authpkg.UserContext, bool) {
	if authClient == nil {
		return nil, false
	}
	token := extractTokenFromHeader(r)
	if token == "" {
		return nil, false
	}
	validateResp, err := authClient.ValidateToken(r.Context(), &pb.ValidateTokenRequest{Token: token})
	if err != nil || !validateResp.Valid {
		return nil, false
	}
	return authpkg.UserContextFromValidateToken(validateResp, token), true
}

// GetUserFromRequest retrieves user context set by auth middleware.
func GetUserFromRequest(r *http.Request) (*authpkg.UserContext, error) {
	return authpkg.GetUserFromContext(r.Context())
}

func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		if cookie, err := r.Cookie("token"); err == nil && cookie != nil {
			return cookie.Value
		}
		return ""
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return authHeader
	}
	return strings.TrimPrefix(authHeader, bearerPrefix)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
