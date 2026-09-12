// Package auth provides local token validation for the auth service.
package auth

import (
	"context"

	"metarang/auth-service/internal/repository"
	sharedauth "metarang/shared/pkg/auth"
)

// LocalTokenValidator validates Sanctum-style tokens via the local token repository.
type LocalTokenValidator struct {
	tokenRepo repository.TokenRepository
}

// NewLocalTokenValidator creates a TokenValidator backed by auth-service's token store.
func NewLocalTokenValidator(tokenRepo repository.TokenRepository) *LocalTokenValidator {
	return &LocalTokenValidator{tokenRepo: tokenRepo}
}

// ValidateToken implements sharedauth.TokenValidator.
func (v *LocalTokenValidator) ValidateToken(ctx context.Context, token string) (*sharedauth.UserContext, error) {
	session, err := v.tokenRepo.ValidateTokenSession(ctx, token)
	if err != nil {
		return nil, err
	}
	return &sharedauth.UserContext{
		UserID:      session.User.ID,
		Email:       session.User.Email,
		Token:       token,
		WalletLogin: session.WalletLogin,
	}, nil
}
