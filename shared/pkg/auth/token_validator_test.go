package auth_test

import (
	"testing"

	pb "metarang/shared/pb/auth"
	authpkg "metarang/shared/pkg/auth"
)

func TestUserContextFromValidateToken(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		if got := authpkg.UserContextFromValidateToken(nil, "tok"); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("copies wallet login", func(t *testing.T) {
		got := authpkg.UserContextFromValidateToken(&pb.ValidateTokenResponse{
			Valid:       true,
			UserId:      11,
			Email:       "a@b.com",
			WalletLogin: true,
		}, "tok")
		if got == nil {
			t.Fatal("expected user context")
		}
		if got.UserID != 11 || got.Email != "a@b.com" || got.Token != "tok" || !got.WalletLogin {
			t.Fatalf("unexpected: %+v", got)
		}
	})
}
