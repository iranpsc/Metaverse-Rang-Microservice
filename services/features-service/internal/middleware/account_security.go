package middleware

import (
	"context"
	"fmt"
	"net/http"

	authpb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

type authAccountSecurityChecker struct {
	client authpb.AuthServiceClient
}

func (c *authAccountSecurityChecker) CheckAccountSecurity(ctx context.Context, userID uint64) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("auth client is not configured")
	}
	ctx = authpkg.AttachOutgoingAuth(ctx)
	resp, err := c.client.CheckAccountSecurity(ctx, &authpb.CheckAccountSecurityRequest{UserId: userID})
	if err != nil {
		return false, err
	}
	return resp.GetUnlocked(), nil
}

// AccountSecurityMiddleware blocks POST/PUT/DELETE when account security is locked (HTTP 410).
// Crypto-wallet login sessions skip the unlock requirement.
// It must run after AuthMiddleware so the user context is available.
func AccountSecurityMiddleware(client authpb.AuthServiceClient) func(http.Handler) http.Handler {
	if client == nil {
		return authpkg.AccountSecurityMiddleware(nil)
	}
	return authpkg.AccountSecurityMiddleware(&authAccountSecurityChecker{client: client})
}
