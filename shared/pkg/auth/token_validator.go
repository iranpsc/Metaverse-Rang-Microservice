package auth

import (
	"context"

	"google.golang.org/grpc"

	pb "metarang/shared/pb/auth"
)

// AuthServiceTokenValidator implements TokenValidator interface using the auth service gRPC client.
// This adapter bridges the gap between the auth service's ValidateToken response
// and the middleware's expected UserContext format.
type AuthServiceTokenValidator struct {
	authClient pb.AuthServiceClient
}

// NewAuthServiceTokenValidator creates a new token validator that uses the auth service.
func NewAuthServiceTokenValidator(conn *grpc.ClientConn) *AuthServiceTokenValidator {
	return &AuthServiceTokenValidator{
		authClient: pb.NewAuthServiceClient(conn),
	}
}

// NewAuthServiceTokenValidatorWithClient creates a validator with an existing client.
func NewAuthServiceTokenValidatorWithClient(client pb.AuthServiceClient) *AuthServiceTokenValidator {
	return &AuthServiceTokenValidator{
		authClient: client,
	}
}

// ValidateToken validates the token by calling the auth service and returns UserContext.
func (v *AuthServiceTokenValidator) ValidateToken(ctx context.Context, token string) (*UserContext, error) {
	resp, err := v.authClient.ValidateToken(ctx, &pb.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Valid {
		return nil, ErrInvalidToken
	}

	return UserContextFromValidateToken(resp, token), nil
}

// UserContextFromValidateToken maps an auth-service ValidateToken response into UserContext.
func UserContextFromValidateToken(resp *pb.ValidateTokenResponse, token string) *UserContext {
	if resp == nil {
		return nil
	}
	return &UserContext{
		UserID:      resp.GetUserId(),
		Email:       resp.GetEmail(),
		Token:       token,
		WalletLogin: resp.GetWalletLogin(),
	}
}
