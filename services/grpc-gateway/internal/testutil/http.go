package testutil

import (
	"context"
	"net/http"

	authpkg "metargb/shared/pkg/auth"
)

// RequestWithUser attaches an authenticated user to the request context.
func RequestWithUser(r *http.Request, userID uint64) *http.Request {
	userCtx := &authpkg.UserContext{UserID: userID}
	ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
	return r.WithContext(ctx)
}
