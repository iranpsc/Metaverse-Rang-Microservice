package wallet_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
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
		return "", service.ErrUserNotFound
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

type fakeWalletAccountSecurityRepo struct {
	nextID      uint64
	records     map[uint64]*models.AccountSecurity
	createCount int
	updateCount int
}

func newFakeWalletAccountSecurityRepo() *fakeWalletAccountSecurityRepo {
	return &fakeWalletAccountSecurityRepo{
		nextID:  100,
		records: map[uint64]*models.AccountSecurity{},
	}
}

func (f *fakeWalletAccountSecurityRepo) GetByUserID(_ context.Context, userID uint64) (*models.AccountSecurity, error) {
	return f.records[userID], nil
}

func (f *fakeWalletAccountSecurityRepo) Create(_ context.Context, security *models.AccountSecurity) error {
	f.createCount++
	if security.ID == 0 {
		security.ID = f.nextID
		f.nextID++
	}
	now := time.Now()
	security.CreatedAt = now
	security.UpdatedAt = now
	f.records[security.UserID] = security
	return nil
}

func (f *fakeWalletAccountSecurityRepo) Update(_ context.Context, security *models.AccountSecurity) error {
	f.updateCount++
	security.UpdatedAt = time.Now()
	f.records[security.UserID] = security
	return nil
}

func (f *fakeWalletAccountSecurityRepo) GetOtpByAccountSecurity(context.Context, uint64) (*models.Otp, error) {
	return nil, nil
}
func (f *fakeWalletAccountSecurityRepo) UpsertOtp(context.Context, *models.Otp) error { return nil }
func (f *fakeWalletAccountSecurityRepo) DeleteOtp(context.Context, uint64) error      { return nil }

type fakeWalletActivityRepo struct {
	events []*models.UserEvent
}

func (f *fakeWalletActivityRepo) CreateUserEvent(_ context.Context, event *models.UserEvent) error {
	f.events = append(f.events, event)
	return nil
}
func (f *fakeWalletActivityRepo) CreateActivity(context.Context, *models.UserActivity) error {
	return nil
}
func (f *fakeWalletActivityRepo) GetLatestActivity(context.Context, uint64) (*models.UserActivity, error) {
	return nil, nil
}
func (f *fakeWalletActivityRepo) UpdateActivity(context.Context, *models.UserActivity) error {
	return nil
}
func (f *fakeWalletActivityRepo) GetTotalActivityMinutes(context.Context, uint64) (int32, error) {
	return 0, nil
}
func (f *fakeWalletActivityRepo) GetUserLog(context.Context, uint64) (*models.UserLog, error) {
	return nil, nil
}
func (f *fakeWalletActivityRepo) CreateUserLog(context.Context, *models.UserLog) error { return nil }
func (f *fakeWalletActivityRepo) UpdateUserLog(context.Context, *models.UserLog) error { return nil }
func (f *fakeWalletActivityRepo) IncrementLogField(context.Context, uint64, string, float64) error {
	return nil
}
func (f *fakeWalletActivityRepo) CloseUserEventReport(context.Context, uint64) error { return nil }
func (f *fakeWalletActivityRepo) CreateUserEventReport(context.Context, *models.UserEventReport) error {
	return nil
}
func (f *fakeWalletActivityRepo) CreateUserEventReportResponse(context.Context, *models.UserEventReportResponse) error {
	return nil
}
func (f *fakeWalletActivityRepo) GetUserEventByID(context.Context, uint64, uint64) (*models.UserEvent, error) {
	return nil, nil
}
func (f *fakeWalletActivityRepo) GetUserEventsByUserID(context.Context, uint64, int32) ([]*models.UserEvent, error) {
	return nil, nil
}
func (f *fakeWalletActivityRepo) GetUserEventReportByEventID(context.Context, uint64) (*models.UserEventReport, error) {
	return nil, nil
}
func (f *fakeWalletActivityRepo) UpdateUserEventReportStatus(context.Context, uint64, int32) error {
	return nil
}
func (f *fakeWalletActivityRepo) GetUserEventReportResponses(context.Context, uint64) ([]*models.UserEventReportResponse, error) {
	return nil, nil
}

func newTestWalletConnectionService(
	userRepo *fakeWalletUserRepo,
	cacheRepo *fakeWalletCacheRepo,
	accountRepo *fakeWalletAccountSecurityRepo,
	activityRepo *fakeWalletActivityRepo,
) service.WalletConnectionService {
	return service.NewWalletConnectionService(userRepo, cacheRepo, accountRepo, activityRepo, "Metarang", "http://localhost:8000")
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

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != service.ErrWalletAlreadyConnected {
		t.Fatalf("expected ErrWalletAlreadyConnected, got %v", err)
	}
}

func TestGetLinkNonceSuccess(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}
	cacheRepo := &fakeWalletCacheRepo{}

	svc := newTestWalletConnectionService(userRepo, cacheRepo, nil, nil)
	message, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetLinkNonce returned error: %v", err)
	}
	if !strings.Contains(message, "Link wallet to your Metarang account at localhost:8000.") {
		t.Fatalf("expected link message prefix, got %q", message)
	}
	if !strings.Contains(message, "Account ID: 1") {
		t.Fatalf("expected account id in message, got %q", message)
	}
	if !strings.Contains(message, "Wallet: "+address) {
		t.Fatalf("expected wallet address in message, got %q", message)
	}
	if cacheRepo.linkNonces[walletNonceKey(1, address)] == "" {
		t.Fatalf("expected link nonce to be cached")
	}
}

func TestGetLinkNonceRejectsAlreadyLinkedWallet(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
			2: {
				ID:            2,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != service.ErrWalletAlreadyLinked {
		t.Fatalf("expected ErrWalletAlreadyLinked, got %v", err)
	}
}

func TestGetLinkNonceRejectsInvalidAddress(t *testing.T) {
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.GetLinkNonce(context.Background(), 1, "not-a-wallet")
	if err != service.ErrInvalidWalletAddress {
		t.Fatalf("expected ErrInvalidWalletAddress, got %v", err)
	}
}

func TestLinkWalletSuccess(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}
	cacheRepo := &fakeWalletCacheRepo{}

	svc := newTestWalletConnectionService(userRepo, cacheRepo, nil, nil)
	message, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetLinkNonce returned error: %v", err)
	}

	signature := signWalletMessage(t, message)
	linkedAddress, err := svc.LinkWallet(context.Background(), 1, address, signature, "127.0.0.1")
	if err != nil {
		t.Fatalf("LinkWallet returned error: %v", err)
	}
	if linkedAddress != address {
		t.Fatalf("expected linked address %q, got %q", address, linkedAddress)
	}

	user := userRepo.users[1]
	if !user.WalletAddress.Valid || user.WalletAddress.String != address {
		t.Fatalf("expected wallet to be linked on user record, got %+v", user.WalletAddress)
	}
}

func TestLinkWalletRejectsExpiredNonce(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.LinkWallet(context.Background(), 1, address, "0x"+strings.Repeat("a", 130), "127.0.0.1")
	if err != service.ErrWalletNonceExpired {
		t.Fatalf("expected ErrWalletNonceExpired, got %v", err)
	}
}

func TestLinkWalletRejectsInvalidSignature(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	message, err := svc.GetLinkNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetLinkNonce returned error: %v", err)
	}

	wrongMessage := message + "tampered"
	signature := signWalletMessage(t, wrongMessage)
	_, err = svc.LinkWallet(context.Background(), 1, address, signature, "127.0.0.1")
	if err != service.ErrWalletSignatureFailed {
		t.Fatalf("expected ErrWalletSignatureFailed, got %v", err)
	}
}

func TestLinkWalletRejectsInvalidSignatureFormat(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	if _, err := svc.GetLinkNonce(context.Background(), 1, address); err != nil {
		t.Fatalf("GetLinkNonce returned error: %v", err)
	}

	_, err := svc.LinkWallet(context.Background(), 1, address, "0x1234", "127.0.0.1")
	if err != service.ErrInvalidWalletSignature {
		t.Fatalf("expected ErrInvalidWalletSignature, got %v", err)
	}
}

func TestGetSecurityNonceRequiresConnectedWallet(t *testing.T) {
	address := "0x2222222222222222222222222222222222222222"
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {ID: 1},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != service.ErrWalletNotConnectedToAccount {
		t.Fatalf("expected ErrWalletNotConnectedToAccount, got %v", err)
	}
}

func TestGetSecurityNonceSuccess(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}
	cacheRepo := &fakeWalletCacheRepo{}

	svc := newTestWalletConnectionService(userRepo, cacheRepo, nil, nil)
	message, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetSecurityNonce returned error: %v", err)
	}
	if !strings.Contains(message, "Unlock account security on Metarang at localhost:8000.") {
		t.Fatalf("expected security message prefix, got %q", message)
	}
	if cacheRepo.securityNonces[walletNonceKey(1, address)] == "" {
		t.Fatalf("expected security nonce to be cached")
	}
}

func TestGetSecurityNonceRejectsMismatchedAddress(t *testing.T) {
	connected := testWalletAddress(t)
	other := "0x2222222222222222222222222222222222222222"
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: connected, Valid: true},
			},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, nil, nil)
	_, err := svc.GetSecurityNonce(context.Background(), 1, other)
	if err != service.ErrWalletNotConnectedToAccount {
		t.Fatalf("expected ErrWalletNotConnectedToAccount, got %v", err)
	}
}

func TestVerifySecuritySignatureCreatesAccountSecurity(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}
	accountRepo := newFakeWalletAccountSecurityRepo()
	activityRepo := &fakeWalletActivityRepo{}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, accountRepo, activityRepo)
	message, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetSecurityNonce returned error: %v", err)
	}

	signature := signWalletMessage(t, message)
	before := time.Now().Unix()
	until, err := svc.VerifySecuritySignature(context.Background(), 1, address, signature, 15, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("VerifySecuritySignature returned error: %v", err)
	}
	after := time.Now().Unix()

	expectedMin := before + 15*60
	expectedMax := after + 15*60
	if until < expectedMin || until > expectedMax {
		t.Fatalf("expected until between %d and %d, got %d", expectedMin, expectedMax, until)
	}

	security := accountRepo.records[1]
	if security == nil {
		t.Fatalf("expected account security record to be created")
	}
	if !security.Unlocked {
		t.Fatalf("expected security to be unlocked")
	}
	if security.Length != 15*60 {
		t.Fatalf("expected length 900, got %d", security.Length)
	}
	if accountRepo.createCount != 1 {
		t.Fatalf("expected one create, got %d", accountRepo.createCount)
	}
	if len(activityRepo.events) != 1 {
		t.Fatalf("expected one user event, got %d", len(activityRepo.events))
	}
}

func TestVerifySecuritySignatureUpdatesExistingRecord(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}
	accountRepo := newFakeWalletAccountSecurityRepo()
	accountRepo.records[1] = &models.AccountSecurity{
		ID:       42,
		UserID:   1,
		Unlocked: false,
		Length:   300,
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, accountRepo, &fakeWalletActivityRepo{})
	message, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetSecurityNonce returned error: %v", err)
	}

	signature := signWalletMessage(t, message)
	if _, err := svc.VerifySecuritySignature(context.Background(), 1, address, signature, 20, "127.0.0.1", "test-agent"); err != nil {
		t.Fatalf("VerifySecuritySignature returned error: %v", err)
	}

	security := accountRepo.records[1]
	if !security.Unlocked {
		t.Fatalf("expected security to be unlocked")
	}
	if security.Length != 20*60 {
		t.Fatalf("expected length 1200, got %d", security.Length)
	}
	if accountRepo.createCount != 0 {
		t.Fatalf("expected no create, got %d", accountRepo.createCount)
	}
	if accountRepo.updateCount != 1 {
		t.Fatalf("expected one update, got %d", accountRepo.updateCount)
	}
}

func TestVerifySecuritySignatureRejectsInvalidDuration(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, newFakeWalletAccountSecurityRepo(), &fakeWalletActivityRepo{})
	message, err := svc.GetSecurityNonce(context.Background(), 1, address)
	if err != nil {
		t.Fatalf("GetSecurityNonce returned error: %v", err)
	}
	signature := signWalletMessage(t, message)

	_, err = svc.VerifySecuritySignature(context.Background(), 1, address, signature, 4, "127.0.0.1", "test-agent")
	if err != service.ErrInvalidWalletSecurityDuration {
		t.Fatalf("expected ErrInvalidWalletSecurityDuration for low duration, got %v", err)
	}

	_, err = svc.VerifySecuritySignature(context.Background(), 1, address, signature, 121, "127.0.0.1", "test-agent")
	if err != service.ErrInvalidWalletSecurityDuration {
		t.Fatalf("expected ErrInvalidWalletSecurityDuration for high duration, got %v", err)
	}
}

func TestVerifySecuritySignatureRejectsExpiredNonce(t *testing.T) {
	address := testWalletAddress(t)
	userRepo := &fakeWalletUserRepo{
		users: map[uint64]*models.User{
			1: {
				ID:            1,
				WalletAddress: sql.NullString{String: address, Valid: true},
			},
		},
	}

	svc := newTestWalletConnectionService(userRepo, &fakeWalletCacheRepo{}, newFakeWalletAccountSecurityRepo(), &fakeWalletActivityRepo{})
	_, err := svc.VerifySecuritySignature(
		context.Background(),
		1,
		address,
		"0x"+strings.Repeat("a", 130),
		15,
		"127.0.0.1",
		"test-agent",
	)
	if err != service.ErrWalletNonceExpired {
		t.Fatalf("expected ErrWalletNonceExpired, got %v", err)
	}
}
