// Package middleware provides HTTP authentication for features-service.
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
			token := ExtractToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			response, err := client.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token})
			if err != nil || !response.Valid {
				writeError(w, http.StatusUnauthorized, "Unauthenticated")
				return
			}
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{
				UserID: response.UserId,
				Email:  response.Email,
				Token:  token,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(client authpb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client != nil {
				token := ExtractToken(r)
				if token != "" {
					if response, err := client.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token}); err == nil && response.Valid {
						r = r.WithContext(context.WithValue(r.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{
							UserID: response.UserId,
							Email:  response.Email,
							Token:  token,
						}))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetUserFromRequest(r *http.Request) (*authpkg.UserContext, error) {
	return authpkg.GetUserFromContext(r.Context())
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
