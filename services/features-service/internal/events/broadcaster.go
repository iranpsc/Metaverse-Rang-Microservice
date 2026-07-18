// Package events provides event broadcasting for the features service.
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// EventBroadcaster interface for broadcasting events
type EventBroadcaster interface {
	BroadcastFeatureStatusChanged(ctx context.Context, featureID uint64, rgb string) error
	Close() error
}

// RedisBroadcaster implements EventBroadcaster using Redis Pub/Sub
type RedisBroadcaster struct {
	redisClient *redis.Client
	channel     string
}

// NewRedisBroadcaster creates a new Redis-based event broadcaster
func NewRedisBroadcaster(redisAddr, redisPassword, channel string) (*RedisBroadcaster, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisBroadcaster{
		redisClient: rdb,
		channel:     channel,
	}, nil
}

// BroadcastFeatureStatusChanged broadcasts a feature status change event
// Matches Laravel's FeatureStatusChanged event structure
func (b *RedisBroadcaster) BroadcastFeatureStatusChanged(ctx context.Context, featureID uint64, rgb string) error {
	payload := map[string]interface{}{
		"id":  featureID,
		"rgb": rgb,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Publish to Redis channel
	err = b.redisClient.Publish(ctx, b.channel, payloadJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (b *RedisBroadcaster) Close() error {
	if b.redisClient != nil {
		return b.redisClient.Close()
	}
	return nil
}
