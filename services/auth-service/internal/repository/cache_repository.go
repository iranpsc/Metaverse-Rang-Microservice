package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheRepository handles caching operations for OAuth state and redirect URLs
type CacheRepository interface {
	// SetState stores the OAuth state with 5 minute TTL
	SetState(ctx context.Context, state string, ttl time.Duration) error

	// GetState retrieves and removes the OAuth state (pull semantics)
	GetState(ctx context.Context, state string) (bool, error)

	// SetRedirectTo stores the redirect_to URL with 5 minute TTL
	SetRedirectTo(ctx context.Context, state, redirectTo string, ttl time.Duration) error

	// GetRedirectTo retrieves and removes the redirect_to URL (pull semantics)
	GetRedirectTo(ctx context.Context, state string) (string, error)

	// SetBackURL stores the back_url with 5 minute TTL
	SetBackURL(ctx context.Context, state, backURL string, ttl time.Duration) error

	// GetBackURL retrieves and removes the back_url (pull semantics)
	GetBackURL(ctx context.Context, state string) (string, error)

	// TryAcquireAccountSecurityVerificationSlot returns true when the user may request a new verification code.
	TryAcquireAccountSecurityVerificationSlot(ctx context.Context, userID uint64, period time.Duration) (bool, error)

	TryAcquireMobileChangeSendSlot(ctx context.Context, userID uint64, period time.Duration) (bool, error)
	ReleaseMobileChangeSendSlot(ctx context.Context, userID uint64) error
	SaveMobileChangeChallenge(ctx context.Context, userID uint64, challenge *MobileChangeChallenge, ttl time.Duration) error
	GetMobileChangeChallenge(ctx context.Context, userID uint64) (*MobileChangeChallenge, error)
	DeleteMobileChangeChallenge(ctx context.Context, userID uint64) error

	SetWeb3LinkNonce(ctx context.Context, userID uint64, address, nonce string, ttl time.Duration) error
	PullWeb3LinkNonce(ctx context.Context, userID uint64, address string) (string, error)
	SetWeb3SecurityNonce(ctx context.Context, userID uint64, address, nonce string, ttl time.Duration) error
	PullWeb3SecurityNonce(ctx context.Context, userID uint64, address string) (string, error)
}

type cacheRepository struct {
	client *redis.Client
}

// NewCacheRepository creates a new cache repository
func NewCacheRepository(client *redis.Client) CacheRepository {
	return &cacheRepository{
		client: client,
	}
}

func (r *cacheRepository) SetState(ctx context.Context, state string, ttl time.Duration) error {
	key := fmt.Sprintf("oauth:state:%s", state)
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *cacheRepository) GetState(ctx context.Context, state string) (bool, error) {
	key := fmt.Sprintf("oauth:state:%s", state)

	// Use GETDEL to atomically get and delete (pull semantics)
	val, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get state: %w", err)
	}

	return val == "1", nil
}

func (r *cacheRepository) SetRedirectTo(ctx context.Context, state, redirectTo string, ttl time.Duration) error {
	key := fmt.Sprintf("oauth:redirect_to:%s", state)
	return r.client.Set(ctx, key, redirectTo, ttl).Err()
}

func (r *cacheRepository) GetRedirectTo(ctx context.Context, state string) (string, error) {
	key := fmt.Sprintf("oauth:redirect_to:%s", state)

	// Use GETDEL to atomically get and delete (pull semantics)
	val, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get redirect_to: %w", err)
	}

	return val, nil
}

func (r *cacheRepository) SetBackURL(ctx context.Context, state, backURL string, ttl time.Duration) error {
	key := fmt.Sprintf("oauth:back_url:%s", state)
	return r.client.Set(ctx, key, backURL, ttl).Err()
}

func (r *cacheRepository) GetBackURL(ctx context.Context, state string) (string, error) {
	key := fmt.Sprintf("oauth:back_url:%s", state)

	// Use GETDEL to atomically get and delete (pull semantics)
	val, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get back_url: %w", err)
	}

	return val, nil
}

func (r *cacheRepository) TryAcquireAccountSecurityVerificationSlot(ctx context.Context, userID uint64, period time.Duration) (bool, error) {
	key := fmt.Sprintf("account_security:verification_request:%d", userID)
	ok, err := r.client.SetNX(ctx, key, "1", period).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check verification request rate limit: %w", err)
	}
	return ok, nil
}

// MobileChangeChallenge is the pending OTP + target mobile used to change a user's phone number.
type MobileChangeChallenge struct {
	Phone     string    `json:"phone"`
	CodeHash  string    `json:"code_hash"`
	CreatedAt time.Time `json:"created_at"`
	Attempts  int       `json:"attempts"`
	ResetID   uint64    `json:"reset_id,omitempty"`
}

func mobileChangeSendSlotKey(userID uint64) string {
	return fmt.Sprintf("mobile_change:send_slot:%d", userID)
}

func mobileChangeChallengeKey(userID uint64) string {
	return fmt.Sprintf("mobile_change:challenge:%d", userID)
}

func (r *cacheRepository) TryAcquireMobileChangeSendSlot(ctx context.Context, userID uint64, period time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, mobileChangeSendSlotKey(userID), "1", period).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check mobile change send rate limit: %w", err)
	}
	return ok, nil
}

func (r *cacheRepository) ReleaseMobileChangeSendSlot(ctx context.Context, userID uint64) error {
	if err := r.client.Del(ctx, mobileChangeSendSlotKey(userID)).Err(); err != nil {
		return fmt.Errorf("failed to release mobile change send rate limit: %w", err)
	}
	return nil
}

func (r *cacheRepository) SaveMobileChangeChallenge(ctx context.Context, userID uint64, challenge *MobileChangeChallenge, ttl time.Duration) error {
	if challenge == nil {
		return fmt.Errorf("mobile change challenge is required")
	}
	payload, err := json.Marshal(challenge)
	if err != nil {
		return fmt.Errorf("failed to encode mobile change challenge: %w", err)
	}
	if err := r.client.Set(ctx, mobileChangeChallengeKey(userID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store mobile change challenge: %w", err)
	}
	return nil
}

func (r *cacheRepository) GetMobileChangeChallenge(ctx context.Context, userID uint64) (*MobileChangeChallenge, error) {
	val, err := r.client.Get(ctx, mobileChangeChallengeKey(userID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load mobile change challenge: %w", err)
	}
	challenge := &MobileChangeChallenge{}
	if err := json.Unmarshal(val, challenge); err != nil {
		return nil, fmt.Errorf("failed to decode mobile change challenge: %w", err)
	}
	return challenge, nil
}

func (r *cacheRepository) DeleteMobileChangeChallenge(ctx context.Context, userID uint64) error {
	if err := r.client.Del(ctx, mobileChangeChallengeKey(userID)).Err(); err != nil {
		return fmt.Errorf("failed to delete mobile change challenge: %w", err)
	}
	return nil
}

func (r *cacheRepository) SetWeb3LinkNonce(ctx context.Context, userID uint64, address, nonce string, ttl time.Duration) error {
	key := fmt.Sprintf("web3_nonce_link_%d_%s", userID, address)
	return r.client.Set(ctx, key, nonce, ttl).Err()
}

func (r *cacheRepository) PullWeb3LinkNonce(ctx context.Context, userID uint64, address string) (string, error) {
	key := fmt.Sprintf("web3_nonce_link_%d_%s", userID, address)
	val, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to pull web3 link nonce: %w", err)
	}
	return val, nil
}

func (r *cacheRepository) SetWeb3SecurityNonce(ctx context.Context, userID uint64, address, nonce string, ttl time.Duration) error {
	key := fmt.Sprintf("web3_nonce_security_%d_%s", userID, address)
	return r.client.Set(ctx, key, nonce, ttl).Err()
}

func (r *cacheRepository) PullWeb3SecurityNonce(ctx context.Context, userID uint64, address string) (string, error) {
	key := fmt.Sprintf("web3_nonce_security_%d_%s", userID, address)
	val, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to pull web3 security nonce: %w", err)
	}
	return val, nil
}
