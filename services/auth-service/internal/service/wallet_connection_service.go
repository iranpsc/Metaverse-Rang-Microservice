package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
)

const walletNonceTTL = 5 * time.Minute

var (
	walletAddressRegex   = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
	walletSignatureRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{130}$`)
	walletNonceCharset   = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	ErrWalletAlreadyConnected        = errors.New("Wallet already connected to this account.")
	ErrWalletAlreadyLinked           = errors.New("This wallet is already linked to another account.")
	ErrWalletNonceExpired            = errors.New("Nonce expired or not found. Please try again.")
	ErrWalletSignatureFailed         = errors.New("Signature verification failed")
	ErrWalletNotConnectedToAccount   = errors.New("This wallet is not connected to your account.")
	ErrInvalidWalletAddress          = errors.New("invalid wallet address")
	ErrInvalidWalletSignature        = errors.New("invalid wallet signature")
	ErrInvalidWalletSecurityDuration = errors.New("invalid wallet security duration")
)

type WalletConnectionService interface {
	GetLinkNonce(ctx context.Context, userID uint64, address string) (string, error)
	LinkWallet(ctx context.Context, userID uint64, address, signature, ip string) (string, error)
	GetSecurityNonce(ctx context.Context, userID uint64, address string) (string, error)
	VerifySecuritySignature(ctx context.Context, userID uint64, address, signature string, durationMinutes int32, ip, userAgent string) (int64, error)
}

type walletConnectionService struct {
	userRepo            repository.UserRepository
	cacheRepo           repository.CacheRepository
	accountSecurityRepo repository.AccountSecurityRepository
	activityRepo        repository.ActivityRepository
	appName             string
	appURL              string
}

func NewWalletConnectionService(
	userRepo repository.UserRepository,
	cacheRepo repository.CacheRepository,
	accountSecurityRepo repository.AccountSecurityRepository,
	activityRepo repository.ActivityRepository,
	appName, appURL string,
) WalletConnectionService {
	if strings.TrimSpace(appName) == "" {
		appName = "Laravel"
	}
	return &walletConnectionService{
		userRepo:            userRepo,
		cacheRepo:           cacheRepo,
		accountSecurityRepo: accountSecurityRepo,
		activityRepo:        activityRepo,
		appName:             appName,
		appURL:              appURL,
	}
}

func (s *walletConnectionService) GetLinkNonce(ctx context.Context, userID uint64, address string) (string, error) {
	normalizedAddress, err := normalizeWalletAddress(address)
	if err != nil {
		return "", err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return "", ErrUserNotFound
	}
	if user.WalletAddress.Valid && user.WalletAddress.String != "" {
		return "", ErrWalletAlreadyConnected
	}

	taken, err := s.userRepo.ExistsByWalletAddress(ctx, normalizedAddress, 0)
	if err != nil {
		return "", fmt.Errorf("failed to check wallet address: %w", err)
	}
	if taken {
		return "", ErrWalletAlreadyLinked
	}

	nonce := s.buildLinkMessage(userID, normalizedAddress)
	if err := s.cacheRepo.SetWeb3LinkNonce(ctx, userID, normalizedAddress, nonce, walletNonceTTL); err != nil {
		return "", fmt.Errorf("failed to store link nonce: %w", err)
	}

	return nonce, nil
}

func (s *walletConnectionService) LinkWallet(ctx context.Context, userID uint64, address, signature, ip string) (string, error) {
	normalizedAddress, err := normalizeWalletAddress(address)
	if err != nil {
		return "", err
	}
	if err := validateWalletSignature(signature); err != nil {
		return "", err
	}

	nonce, err := s.cacheRepo.PullWeb3LinkNonce(ctx, userID, normalizedAddress)
	if err != nil {
		return "", fmt.Errorf("failed to load link nonce: %w", err)
	}
	if nonce == "" {
		log.Printf("Wallet link rejected: nonce missing or expired (user_id=%d address=%s ip=%s)", userID, normalizedAddress, ip)
		return "", ErrWalletNonceExpired
	}

	if !IsValidWalletSignature(normalizedAddress, signature, nonce) {
		log.Printf("Wallet link rejected: invalid signature (user_id=%d address=%s ip=%s)", userID, normalizedAddress, ip)
		return "", ErrWalletSignatureFailed
	}

	result, err := s.userRepo.LinkWalletAddress(ctx, userID, normalizedAddress)
	if err != nil {
		return "", fmt.Errorf("failed to link wallet: %w", err)
	}

	switch result {
	case repository.LinkWalletAlreadyConnected:
		return "", ErrWalletAlreadyConnected
	case repository.LinkWalletAlreadyLinked:
		return "", ErrWalletAlreadyLinked
	default:
		return normalizedAddress, nil
	}
}

func (s *walletConnectionService) GetSecurityNonce(ctx context.Context, userID uint64, address string) (string, error) {
	normalizedAddress, err := normalizeWalletAddress(address)
	if err != nil {
		return "", err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return "", ErrUserNotFound
	}
	if !user.WalletAddress.Valid || strings.ToLower(user.WalletAddress.String) != normalizedAddress {
		return "", ErrWalletNotConnectedToAccount
	}

	nonce := s.buildSecurityMessage(userID, normalizedAddress)
	if err := s.cacheRepo.SetWeb3SecurityNonce(ctx, userID, normalizedAddress, nonce, walletNonceTTL); err != nil {
		return "", fmt.Errorf("failed to store security nonce: %w", err)
	}

	return nonce, nil
}

func (s *walletConnectionService) VerifySecuritySignature(ctx context.Context, userID uint64, address, signature string, durationMinutes int32, ip, userAgent string) (int64, error) {
	normalizedAddress, err := normalizeWalletAddress(address)
	if err != nil {
		return 0, err
	}
	if err := validateWalletSignature(signature); err != nil {
		return 0, err
	}
	if durationMinutes < 5 || durationMinutes > 120 {
		return 0, ErrInvalidWalletSecurityDuration
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return 0, ErrUserNotFound
	}
	if !user.WalletAddress.Valid || strings.ToLower(user.WalletAddress.String) != normalizedAddress {
		return 0, ErrWalletNotConnectedToAccount
	}

	nonce, err := s.cacheRepo.PullWeb3SecurityNonce(ctx, userID, normalizedAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to load security nonce: %w", err)
	}
	if nonce == "" {
		return 0, ErrWalletNonceExpired
	}

	if !IsValidWalletSignature(normalizedAddress, signature, nonce) {
		log.Printf("Wallet security verification rejected: invalid signature (user_id=%d address=%s ip=%s)", userID, normalizedAddress, ip)
		return 0, ErrWalletSignatureFailed
	}

	lengthSeconds := int64(durationMinutes) * 60
	until := time.Now().Unix() + lengthSeconds

	security, err := s.accountSecurityRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to load account security: %w", err)
	}
	if security == nil {
		security = &models.AccountSecurity{
			UserID:   userID,
			Unlocked: true,
			Until:    sql.NullInt64{Int64: until, Valid: true},
			Length:   lengthSeconds,
		}
		if err := s.accountSecurityRepo.Create(ctx, security); err != nil {
			return 0, fmt.Errorf("failed to create account security: %w", err)
		}
	} else {
		security.Unlocked = true
		security.Until = sql.NullInt64{Int64: until, Valid: true}
		security.Length = lengthSeconds
		if err := s.accountSecurityRepo.Update(ctx, security); err != nil {
			return 0, fmt.Errorf("failed to update account security: %w", err)
		}
	}

	if err := s.activityRepo.CreateUserEvent(ctx, &models.UserEvent{
		UserID: userID,
		Event:  "غیر فعال سازی امنیت حساب کاربری (کیف پول)",
		IP:     ip,
		Device: userAgent,
		Status: 1,
	}); err != nil {
		log.Printf("Warning: failed to create wallet security user event: %v", err)
	}

	return until, nil
}

func (s *walletConnectionService) buildLinkMessage(userID uint64, address string) string {
	return strings.Join([]string{
		fmt.Sprintf("Link wallet to your %s account at %s.", s.appName, s.applicationDomain()),
		"",
		fmt.Sprintf("Account ID: %d", userID),
		fmt.Sprintf("Wallet: %s", address),
		fmt.Sprintf("Nonce: %s", randomWalletNonce(32)),
	}, "\n")
}

func (s *walletConnectionService) buildSecurityMessage(userID uint64, address string) string {
	return strings.Join([]string{
		fmt.Sprintf("Unlock account security on %s at %s.", s.appName, s.applicationDomain()),
		"",
		fmt.Sprintf("Account ID: %d", userID),
		fmt.Sprintf("Wallet: %s", address),
		fmt.Sprintf("Nonce: %s", randomWalletNonce(32)),
	}, "\n")
}

func (s *walletConnectionService) applicationDomain() string {
	parsed, err := url.Parse(s.appURL)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "localhost"
}

func normalizeWalletAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if !walletAddressRegex.MatchString(address) {
		return "", ErrInvalidWalletAddress
	}
	return strings.ToLower(address), nil
}

func validateWalletSignature(signature string) error {
	signature = strings.TrimSpace(signature)
	if !walletSignatureRegex.MatchString(signature) {
		return ErrInvalidWalletSignature
	}
	return nil
}

func randomWalletNonce(length int) string {
	out := make([]byte, length)
	max := big.NewInt(int64(len(walletNonceCharset)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			out[i] = walletNonceCharset[0]
			continue
		}
		out[i] = walletNonceCharset[n.Int64()]
	}
	return string(out)
}
