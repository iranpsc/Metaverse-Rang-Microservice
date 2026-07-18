// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	pb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

// AuthMiddleware creates an HTTP middleware that validates authentication tokens
// and adds user context to the request context.
func AuthMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token := extractTokenFromHeader(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}

			// Validate token with auth service
			validateReq := &pb.ValidateTokenRequest{Token: token}
			validateResp, err := authClient.ValidateToken(r.Context(), validateReq)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}

			if !validateResp.Valid {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}

			// Create user context
			userCtx := &authpkg.UserContext{
				UserID: validateResp.UserId,
				Email:  validateResp.Email,
				Token:  token,
			}

			// Add user context to request context
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)

			// Add authorization header to gRPC metadata so gRPC services can access it
			ctx = ContextWithAuth(ctx, token)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware creates an HTTP middleware that validates authentication tokens
// if present, but doesn't require them. Useful for routes that work with or without auth.
func OptionalAuthMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token := extractTokenFromHeader(r)
			if token != "" {
				// Validate token with auth service
				validateReq := &pb.ValidateTokenRequest{Token: token}
				validateResp, err := authClient.ValidateToken(r.Context(), validateReq)
				if err == nil && validateResp.Valid {
					// Create user context
					userCtx := &authpkg.UserContext{
						UserID: validateResp.UserId,
						Email:  validateResp.Email,
						Token:  token,
					}

					// Add user context to request context
					ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)

					// Add authorization header to gRPC metadata so gRPC services can access it
					ctx = ContextWithAuth(ctx, token)

					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractTokenFromHeader extracts Bearer token from Authorization header
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Try cookie as fallback
		cookie, err := r.Cookie("token")
		if err == nil && cookie != nil {
			return cookie.Value
		}
		return ""
	}

	// Check for Bearer token format
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		// If no Bearer prefix, assume the whole header is the token
		return authHeader
	}

	return strings.TrimPrefix(authHeader, bearerPrefix)
}

// writeError writes an error response in JSON format
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}

// GuestMiddleware creates an HTTP middleware that only allows unauthenticated users.
// If a valid authentication token is present, the request is rejected.
// This is useful for routes like registration and login that should only be accessible to guests.
func GuestMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token := extractTokenFromHeader(r)
			if token != "" {
				// Validate token with auth service
				validateReq := &pb.ValidateTokenRequest{Token: token}
				validateResp, err := authClient.ValidateToken(r.Context(), validateReq)
				// If token is valid, reject the request (user is already authenticated)
				if err == nil && validateResp.Valid {
					writeError(w, http.StatusForbidden, "Forbidden")
					return
				}
			}

			// No valid token found, allow request to proceed
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromRequest retrieves user context from the HTTP request context
func GetUserFromRequest(r *http.Request) (*authpkg.UserContext, error) {
	return authpkg.GetUserFromContext(r.Context())
}

// ContextWithAuth adds the authorization token to the context as gRPC metadata.
// This allows gRPC services to receive the token in their interceptors.
// Use this when calling gRPC services from HTTP handlers.
func ContextWithAuth(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	// Add authorization header to gRPC metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// ContextWithAuthFromRequest extracts the token from the request and adds it to context as gRPC metadata.
// This is a convenience function that combines token extraction and context creation.
func ContextWithAuthFromRequest(r *http.Request) context.Context {
	token := extractTokenFromHeader(r)
	return ContextWithAuth(r.Context(), token)
}
