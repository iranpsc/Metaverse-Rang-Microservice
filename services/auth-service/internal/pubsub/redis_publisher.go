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
	// presenceOfflineTTL covers the sweeper lookback so the same quiet period is announced once.
	// An online publish clears this key so the next 2 minutes of inactivity can announce again.
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

func presenceOfflineKey(userID uint64) string {
	return "presence:offline:" + strconv.FormatUint(userID, 10)
}

// PublishUserStatusChanged publishes a user status change on the user-status channel.
// Online is published on every call. Offline is published once per quiet period, until a later online event.
func (p *redisPublisher) PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error {
	if userID == 0 {
		return nil
	}

	if !online {
		ok, err := p.client.SetNX(ctx, presenceOfflineKey(userID), "1", presenceOfflineTTL).Result()
		if err != nil {
			return fmt.Errorf("failed to claim offline presence: %w", err)
		}
		if !ok {
			return nil
		}
	}

	if err := p.publishStatus(ctx, userID, online); err != nil {
		if !online {
			_ = p.client.Del(context.Background(), presenceOfflineKey(userID)).Err()
		}
		return err
	}
	if online {
		_ = p.client.Del(ctx, presenceOfflineKey(userID)).Err()
	}
	return nil
}

func (p *redisPublisher) publishStatus(ctx context.Context, userID uint64, online bool) error {
	payload, err := json.Marshal(userStatusEnvelope{
		Data: UserStatusChangedEvent{
			UserID: strconv.FormatUint(userID, 10),
			Online: online,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if err := p.client.Publish(ctx, userStatusChannel, payload).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (p *redisPublisher) Close() error {
	return p.client.Close()
}
