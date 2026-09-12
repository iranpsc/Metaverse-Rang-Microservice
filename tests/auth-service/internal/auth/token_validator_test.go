package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/auth-service/internal/auth"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

type mockTokenRepo struct {
	user        *models.User
	walletLogin bool
	err         error
}

func (m *mockTokenRepo) Create(context.Context, uint64, string, time.Time, bool) (string, error) {
	return "", nil
}
func (m *mockTokenRepo) ValidateToken(_ context.Context, _ string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}
func (m *mockTokenRepo) ValidateTokenSession(_ context.Context, _ string) (*repository.ValidatedToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &repository.ValidatedToken{User: m.user, WalletLogin: m.walletLogin}, nil
}
func (m *mockTokenRepo) DeleteUserTokens(context.Context, uint64) error { return nil }
func (m *mockTokenRepo) FindTokenByHash(context.Context, string) (*models.PersonalAccessToken, error) {
	return nil, nil
}

var _ repository.TokenRepository = (*mockTokenRepo)(nil)

func TestLocalTokenValidator(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		v := auth.NewLocalTokenValidator(&mockTokenRepo{
			user:        &models.User{ID: 9, Email: "a@b.com"},
			walletLogin: true,
		})
		uc, err := v.ValidateToken(context.Background(), "tok")
		if err != nil {
			t.Fatal(err)
		}
		if uc.UserID != 9 || uc.Email != "a@b.com" || uc.Token != "tok" || !uc.WalletLogin {
			t.Fatalf("unexpected: %+v", uc)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		v := auth.NewLocalTokenValidator(&mockTokenRepo{err: errors.New("invalid")})
		_, err := v.ValidateToken(context.Background(), "tok")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
