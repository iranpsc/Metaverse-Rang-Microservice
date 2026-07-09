package authlocal

import (
	"context"

	"metargb/auth-service/internal/repository"
	sharedauth "metargb/shared/pkg/auth"
)

// LocalTokenValidator validates bearer tokens using the auth-service token repository.
type LocalTokenValidator struct {
	tokenRepo repository.TokenRepository
}

func NewLocalTokenValidator(tokenRepo repository.TokenRepository) *LocalTokenValidator {
	return &LocalTokenValidator{tokenRepo: tokenRepo}
}

func (v *LocalTokenValidator) ValidateToken(ctx context.Context, token string) (*sharedauth.UserContext, error) {
	user, err := v.tokenRepo.ValidateToken(ctx, token)
	if err != nil {
		return nil, sharedauth.ErrInvalidToken
	}

	return &sharedauth.UserContext{
		UserID: user.ID,
		Email:  user.Email,
		Token:  token,
	}, nil
}
