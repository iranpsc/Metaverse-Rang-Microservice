// Package pubsub provides Redis pub/sub publishing for the auth service.
package pubsub

import (
	"context"
	"encoding/json"
	"errors"
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
	// presenceSuppressTTL blocks presence touches already queued when the user logs out.
	// A later login clears the key immediately.
	presenceSuppressTTL = 2 * time.Minute
)

// ErrOnlineSuppressed is returned when a presence touch tries to announce online
// after the user has logged out and before they log in again.
var ErrOnlineSuppressed = errors.New("online presence suppressed after logout")

// RedisPublisher handles publishing events to Redis for WebSocket broadcasting
type RedisPublisher interface {
	PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error
	// PublishUserLoggedOut always broadcasts offline and blocks in-flight online touches.
	PublishUserLoggedOut(ctx context.Context, userID uint64) error
	// PublishUserLoggedIn broadcasts online and clears a logout suppression.
	PublishUserLoggedIn(ctx context.Context, userID uint64) error
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

func presenceSuppressKey(userID uint64) string {
	return "presence:suppress-online:" + strconv.FormatUint(userID, 10)
}

// publishOnlineScript announces online unless logout suppression is set.
// Redis runs the script atomically, so it cannot land between a logout's
// suppress write and its offline publish.
var publishOnlineScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 0
end
redis.call('PUBLISH', ARGV[1], ARGV[2])
redis.call('DEL', KEYS[2])
return 1
`)

// publishLogoutScript always announces offline and holds online suppression.
var publishLogoutScript = redis.NewScript(`
redis.call('SET', KEYS[1], '1', 'EX', tonumber(ARGV[1]))
redis.call('SET', KEYS[2], '1', 'EX', tonumber(ARGV[2]))
redis.call('PUBLISH', ARGV[3], ARGV[4])
return 1
`)

// publishLoginScript clears logout suppression and announces online.
var publishLoginScript = redis.NewScript(`
redis.call('DEL', KEYS[1])
redis.call('PUBLISH', ARGV[1], ARGV[2])
redis.call('DEL', KEYS[2])
return 1
`)

// PublishUserStatusChanged publishes a user status change on the user-status channel.
// Online is published on every call unless the user just logged out.
// Offline is published once per quiet period, until a later online event.
// Explicit logout uses PublishUserLoggedOut so it is not dropped by that dedupe.
func (p *redisPublisher) PublishUserStatusChanged(ctx context.Context, userID uint64, online bool) error {
	if userID == 0 {
		return nil
	}
	if online {
		return p.publishOnline(ctx, userID)
	}
	return p.publishOfflineOnce(ctx, userID)
}

// PublishUserLoggedOut broadcasts online=false even when this quiet period was already announced.
func (p *redisPublisher) PublishUserLoggedOut(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return nil
	}
	payload, err := statusPayload(userID, false)
	if err != nil {
		return err
	}
	_, err = publishLogoutScript.Run(
		ctx,
		p.client,
		[]string{presenceSuppressKey(userID), presenceOfflineKey(userID)},
		int(presenceSuppressTTL.Seconds()),
		int(presenceOfflineTTL.Seconds()),
		userStatusChannel,
		payload,
	).Result()
	if err != nil {
		return fmt.Errorf("failed to publish logout status: %w", err)
	}
	return nil
}

// PublishUserLoggedIn broadcasts online=true and allows later presence touches.
func (p *redisPublisher) PublishUserLoggedIn(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return nil
	}
	payload, err := statusPayload(userID, true)
	if err != nil {
		return err
	}
	_, err = publishLoginScript.Run(
		ctx,
		p.client,
		[]string{presenceSuppressKey(userID), presenceOfflineKey(userID)},
		userStatusChannel,
		payload,
	).Result()
	if err != nil {
		return fmt.Errorf("failed to publish login status: %w", err)
	}
	return nil
}

func (p *redisPublisher) publishOnline(ctx context.Context, userID uint64) error {
	payload, err := statusPayload(userID, true)
	if err != nil {
		return err
	}
	n, err := publishOnlineScript.Run(
		ctx,
		p.client,
		[]string{presenceSuppressKey(userID), presenceOfflineKey(userID)},
		userStatusChannel,
		payload,
	).Int()
	if err != nil {
		return fmt.Errorf("failed to publish online status: %w", err)
	}
	if n == 0 {
		return ErrOnlineSuppressed
	}
	return nil
}

func (p *redisPublisher) publishOfflineOnce(ctx context.Context, userID uint64) error {
	ok, err := p.client.SetNX(ctx, presenceOfflineKey(userID), "1", presenceOfflineTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to claim offline presence: %w", err)
	}
	if !ok {
		return nil
	}
	if err := p.publishStatus(ctx, userID, false); err != nil {
		_ = p.client.Del(context.Background(), presenceOfflineKey(userID)).Err()
		return err
	}
	return nil
}

func statusPayload(userID uint64, online bool) (string, error) {
	payload, err := json.Marshal(userStatusEnvelope{
		Data: UserStatusChangedEvent{
			UserID: strconv.FormatUint(userID, 10),
			Online: online,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}
	return string(payload), nil
}

func (p *redisPublisher) publishStatus(ctx context.Context, userID uint64, online bool) error {
	payload, err := statusPayload(userID, online)
	if err != nil {
		return err
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
