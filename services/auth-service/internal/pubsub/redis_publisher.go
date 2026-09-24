// Package pubsub provides Redis pub/sub publishing for the auth service.
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
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

// UserStatusChangedEvent represents the user status change event
type UserStatusChangedEvent struct {
	UserID uint64 `json:"user_id"`
	ID     uint64 `json:"id"` // legacy alias for older consumers
	Online bool   `json:"online"`
}

// PublishUserStatusChanged publishes a user status change event to Redis
// This will be picked up by the WebSocket gateway and broadcast to connected clients
func (p *redisPublisher) PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error {
	event := UserStatusChangedEvent{
		UserID: userID,
		ID:     userID,
		Online: online,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to the channel expected by websocket-gateway
	err = p.client.Publish(ctx, "user-status", payload).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (p *redisPublisher) Close() error {
	return p.client.Close()
}
