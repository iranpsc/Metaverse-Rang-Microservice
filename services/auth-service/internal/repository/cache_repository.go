package repository

import (
	"context"
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
