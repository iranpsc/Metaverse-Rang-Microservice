// Package pubsub provides Redis pub/sub publishing for the auth service.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

const (
	userStatusChannel = "user-status"
	// presenceOnlineTTL matches the offline threshold. Activity refreshes it;
	// once it expires the sweeper may announce offline.
	presenceOnlineTTL = 2 * time.Minute
	// presenceOfflineTTL covers the sweeper lookback so the same user is announced once.
	presenceOfflineTTL = 10 * time.Minute
)

// RedisPublisher handles publishing events to Redis for WebSocket broadcasting
type RedisPublisher interface {
	PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error
	Close() error
}

type redisPublisher struct {
	client *redis.Client
}

// NewRedisPublisher creates a new Redis publisher
func NewRedisPublisher(redisURL string) (RedisPublisher, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Disable maint notifications to avoid warning about maint_notifications command
	// This feature is not available in Redis 7 and causes a harmless warning
	opts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &redisPublisher{
		client: client,
	}, nil
}

// UserStatusChangedEvent is the body of a user-status-changed event.
type UserStatusChangedEvent struct {
	UserID string `json:"user_id"`
	Online bool   `json:"online"`
}

type userStatusEnvelope struct {
	Data UserStatusChangedEvent `json:"data"`
}

func presenceOnlineKey(userID uint64) string {
	return "presence:online:" + strconv.FormatUint(userID, 10)
}

func presenceOfflineKey(userID uint64) string {
	return "presence:offline:" + strconv.FormatUint(userID, 10)
}

// PublishUserStatusChanged publishes a user status change on the user-status channel.
// Repeated online or offline announcements for the same user are suppressed until presence flips.
func (p *redisPublisher) PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error {
	if userID == 0 {
		return nil
	}

	release, err := p.claimPresence(ctx, userID, online)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}

	payload, err := json.Marshal(userStatusEnvelope{
		Data: UserStatusChangedEvent{
			UserID: strconv.FormatUint(userID, 10),
			Online: online,
		},
	})
	if err != nil {
		release()
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.client.Publish(ctx, userStatusChannel, payload).Err(); err != nil {
		release()
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	if online {
		_ = p.client.Del(ctx, presenceOfflineKey(userID)).Err()
	} else {
		_ = p.client.Del(ctx, presenceOnlineKey(userID)).Err()
	}
	return nil
}

// claimPresence returns a rollback func when this process should publish.
// A nil func means the status was already announced.
func (p *redisPublisher) claimPresence(ctx context.Context, userID uint64, online bool) (func(), error) {
	if online {
		ok, err := p.client.SetNX(ctx, presenceOnlineKey(userID), "1", presenceOnlineTTL).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to claim online presence: %w", err)
		}
		if !ok {
			if err := p.client.Expire(ctx, presenceOnlineKey(userID), presenceOnlineTTL).Err(); err != nil {
				return nil, fmt.Errorf("failed to refresh online presence: %w", err)
			}
			return nil, nil
		}
		return func() {
			_ = p.client.Del(context.Background(), presenceOnlineKey(userID)).Err()
		}, nil
	}

	ok, err := p.client.SetNX(ctx, presenceOfflineKey(userID), "1", presenceOfflineTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to claim offline presence: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return func() {
		_ = p.client.Del(context.Background(), presenceOfflineKey(userID)).Err()
	}, nil
}

// Close closes the Redis connection
func (p *redisPublisher) Close() error {
	return p.client.Close()
}
