package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
)

type fakeWalletCacheRepo struct {
	linkNonces     map[string]string
	securityNonces map[string]string
}

func (f *fakeWalletCacheRepo) SetState(context.Context, string, time.Duration) error { return nil }
func (f *fakeWalletCacheRepo) GetState(context.Context, string) (bool, error)        { return false, nil }
func (f *fakeWalletCacheRepo) SetRedirectTo(context.Context, string, string, time.Duration) error {
	return nil
}
func (f *fakeWalletCacheRepo) GetRedirectTo(context.Context, string) (string, error) { return "", nil }
func (f *fakeWalletCacheRepo) SetBackURL(context.Context, string, string, time.Duration) error {
	return nil
}
func (f *fakeWalletCacheRepo) GetBackURL(context.Context, string) (string, error) { return "", nil }
func (f *fakeWalletCacheRepo) TryAcquireAccountSecurityVerificationSlot(context.Context, uint64, time.Duration) (bool, error) {
	return true, nil
}

func (f *fakeWalletCacheRepo) SetWeb3LinkNonce(_ context.Context, userID uint64, address, nonce string, _ time.Duration) error {
	if f.linkNonces == nil {
		f.linkNonces = map[string]string{}
	}
	f.linkNonces[walletNonceKey(userID, address)] = nonce
	return nil
}

func (f *fakeWalletCacheRepo) PullWeb3LinkNonce(_ context.Context, userID uint64, address string) (string, error) {
	key := walletNonceKey(userID, address)
	nonce := f.linkNonces[key]
	delete(f.linkNonces, key)
	return nonce, nil
}

func (f *fakeWalletCacheRepo) SetWeb3SecurityNonce(_ context.Context, userID uint64, address, nonce string, _ time.Duration) error {
	if f.securityNonces == nil {
		f.securityNonces = map[string]string{}
	}
	f.securityNonces[walletNonceKey(userID, address)] = nonce
	return nil
}

func (f *fakeWalletCacheRepo) PullWeb3SecurityNonce(_ context.Context, userID uint64, address string) (string, error) {
	key := walletNonceKey(userID, address)
	nonce := f.securityNonces[key]
	delete(f.securityNonces, key)
	return nonce, nil
}

func walletNonceKey(userID uint64, address string) string {
	return fmt.Sprintf("%d:%s", userID, address)
}

type fakeWalletUserRepo struct {
	users map[uint64]*models.User
}

func (f *fakeWalletUserRepo) Create(context.Context, *models.User) error { return nil }
func (f *fakeWalletUserRepo) FindByEmail(context.Context, string) (*models.User, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) FindByID(_ context.Context, id uint64) (*models.User, error) {
	return f.users[id], nil
}
func (f *fakeWalletUserRepo) Update(context.Context, *models.User) error { return nil }
func (f *fakeWalletUserRepo) UpdateLastSeen(context.Context, uint64) error {
	return nil
}
func (f *fakeWalletUserRepo) FindByCode(context.Context, string) (*models.User, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) GetSettings(context.Context, uint64) (*models.Settings, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) CreateSettings(context.Context, *models.Settings) error { return nil }
func (f *fakeWalletUserRepo) GetKYC(context.Context, uint64) (*models.KYC, error)    { return nil, nil }
func (f *fakeWalletUserRepo) GetUnreadNotificationsCount(context.Context, uint64) (int32, error) {
	return 0, nil
}
func (f *fakeWalletUserRepo) MarkEmailAsVerified(context.Context, uint64) error { return nil }
func (f *fakeWalletUserRepo) UpdatePhone(context.Context, uint64, string) error { return nil }
func (f *fakeWalletUserRepo) MarkPhoneAsVerified(context.Context, uint64) error { return nil }
func (f *fakeWalletUserRepo) IsPhoneTaken(context.Context, string, uint64) (bool, error) {
	return false, nil
}
func (f *fakeWalletUserRepo) ExistsByWalletAddress(_ context.Context, address string, excludeUserID uint64) (bool, error) {
	for id, user := range f.users {
		if excludeUserID > 0 && id == excludeUserID {
			continue
		}
		if user.WalletAddress.Valid && user.WalletAddress.String == address {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeWalletUserRepo) LinkWalletAddress(_ context.Context, userID uint64, address string) (repository.LinkWalletResult, error) {
	user := f.users[userID]
	if user == nil {
		return "", ErrUserNotFound
	}
	if user.WalletAddress.Valid && user.WalletAddress.String != "" {
		return repository.LinkWalletAlreadyConnected, nil
	}
	for id, other := range f.users {
		if id != userID && other.WalletAddress.Valid && other.WalletAddress.String == address {
			return repository.LinkWalletAlreadyLinked, nil
		}
	}
	user.WalletAddress = sql.NullString{String: address, Valid: true}
	return repository.LinkWalletSuccess, nil
}
func (f *fakeWalletUserRepo) ListUsers(context.Context, string, string, int32, int32) ([]*repository.UserWithRelations, int32, error) {
	return nil, 0, nil
}
func (f *fakeWalletUserRepo) GetUsersLevelsForList(context.Context, []uint64) (map[uint64]*repository.UserListLevels, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) GetFollowersCount(context.Context, uint64) (int32, error) { return 0, nil }
func (f *fakeWalletUserRepo) GetFollowingCount(context.Context, uint64) (int32, error) { return 0, nil }
func (f *fakeWalletUserRepo) GetLatestProfilePhotoURL(context.Context, uint64) (string, error) {
	return "", nil
}
func (f *fakeWalletUserRepo) GetAllProfilePhotoURLs(context.Context, uint64) ([]string, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) GetUserLatestLevel(context.Context, uint64) (*repository.UserLevel, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) GetLevelsBelowScore(context.Context, int32) ([]*repository.UserLevel, error) {
	return nil, nil
}
func (f *fakeWalletUserRepo) GetNextLevelScore(context.Context, int32) (int32, error) { return 0, nil }
func (f *fakeWalletUserRepo) GetFeatureCounts(context.Context, uint64) (int32, int32, int32, error) {
	return 0, 0, 0, nil
}

func TestGetLinkNonceRejectsAlreadyConnectedUser(t *testing.T) {
	address := "0x1111111111111111111111111111111111111111"
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}

	svc := NewWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil, "Metarang", "http://localhost:8000")
	_, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != ErrWalletAlreadyConnected {
		t.Fatalf("expected ErrWalletAlreadyConnected, got %v", err)
	}
}

func TestGetSecurityNonceRequiresConnectedWallet(t *testing.T) {
	address := "0x2222222222222222222222222222222222222222"
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := NewWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil, "Metarang", "http://localhost:8000")
	_, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != ErrWalletNotConnectedToAccount {
		t.Fatalf("expected ErrWalletNotConnectedToAccount, got %v", err)
	}
}
