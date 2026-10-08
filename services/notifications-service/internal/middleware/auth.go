// Package middleware provides HTTP authentication middleware for notifications-service.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	pb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/notifications-service/internal/resilience"
)

const (
	defaultAuthTimeout = 2 * time.Second
	defaultAuthRetries = 2
	defaultAuthBackoff = 100 * time.Millisecond
)

// AuthPolicy bounds ValidateToken calls against auth-service.
type AuthPolicy struct {
	Timeout  time.Duration
	Attempts int
	Backoff  time.Duration
	Breaker  *resilience.Breaker
}

// AuthMiddleware validates a Bearer/cookie token via auth-service and injects user context.
func AuthMiddleware(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return AuthMiddlewareWithPolicy(authClient, AuthPolicy{})
}

// AuthMiddlewareWithPolicy is AuthMiddleware with an explicit timeout, retry, and breaker policy.
func AuthMiddlewareWithPolicy(authClient pb.AuthServiceClient, policy AuthPolicy) func(http.Handler) http.Handler {
	policy = policy.normalized()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authClient == nil {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}

			token := extractTokenFromHeader(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}

			var validateResp *pb.ValidateTokenResponse
			err := policy.Breaker.Execute(func() error {
				return resilience.Retry(r.Context(), policy.Attempts, policy.Backoff, authRetryable, func() error {
					ctx, cancel := context.WithTimeout(r.Context(), policy.Timeout)
					defer cancel()
					var callErr error
					validateResp, callErr = authClient.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: token})
					return callErr
				})
			}, authTransportFailure)
			if err != nil || validateResp == nil || !validateResp.Valid {
				writeAuthFailure(w, err)
				return
			}

			userCtx := authpkg.UserContextFromValidateToken(validateResp, token)
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (p AuthPolicy) normalized() AuthPolicy {
	if p.Timeout <= 0 {
		p.Timeout = defaultAuthTimeout
	}
	if p.Attempts < 1 {
		p.Attempts = defaultAuthRetries
	}
	if p.Backoff <= 0 {
		p.Backoff = defaultAuthBackoff
	}
	if p.Breaker == nil {
		p.Breaker = resilience.NewBreaker(resilience.DefaultFailureThreshold, resilience.DefaultOpenCooldown)
	}
	return p
}

func authRetryable(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

func authTransportFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, resilience.ErrCircuitOpen) {
		return false
	}
	return authRetryable(err)
}

func writeAuthFailure(w http.ResponseWriter, err error) {
	if isAuthOutage(err) {
		writeError(w, http.StatusServiceUnavailable, "Auth service unavailable")
		return
	}
	writeError(w, http.StatusUnauthorized, "Unauthenticated")
}

func isAuthOutage(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, resilience.ErrCircuitOpen) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return authRetryable(err)
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
