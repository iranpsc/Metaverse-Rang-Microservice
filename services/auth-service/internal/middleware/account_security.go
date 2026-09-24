// Package middleware provides HTTP authentication for auth-service.
package middleware

import (
	"net/http"

	authpkg "metarang/shared/pkg/auth"
)

// AccountSecurityMiddleware blocks POST/PUT/DELETE when account security is locked (HTTP 410).
// Crypto-wallet login sessions skip the unlock requirement.
func AccountSecurityMiddleware(checker authpkg.AccountSecurityChecker) func(http.Handler) http.Handler {
	return authpkg.AccountSecurityMiddleware(checker)
}
