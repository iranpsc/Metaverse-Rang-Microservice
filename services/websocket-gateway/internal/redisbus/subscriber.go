// Package redisbus subscribes to Redis channels and forwards events to the hub.
package redisbus

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

	"metarang/websocket-gateway/internal/hub"
)

// Channels the gateway listens on. Includes legacy publisher names for compatibility.
var subscribedChannels = []string{
	"user-status",
	"user-status-changed",
	"feature-status",
	"feature-events",
	"notifications",
}

// Subscriber listens to Redis pub/sub channels and forwards events to the hub.
type Subscriber struct {
	client *redis.Client
	pubsub *redis.PubSub
}

// NewSubscriber connects to Redis and starts forwarding events.
func NewSubscriber(ctx context.Context, redisURL string, h *hub.Hub) (*Subscriber, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	opts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}

	client := redis.NewClient(opts)
	pubsub := client.Subscribe(ctx, subscribedChannels...)

	s := &Subscriber{client: client, pubsub: pubsub}
	go s.forward(ctx, h)
	return s, nil
}

func (s *Subscriber) forward(ctx context.Context, h *hub.Hub) {
	ch := s.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
				log.Printf("invalid redis payload on %s: %v", msg.Channel, err)
				continue
			}

			switch msg.Channel {
			case "user-status", "user-status-changed":
				h.BroadcastUserStatus(data)
			case "feature-status", "feature-events":
				h.BroadcastFeatureStatus(data)
			case "notifications":
				h.BroadcastNotification(data)
			default:
				log.Printf("unknown redis channel: %s", msg.Channel)
			}
		}
	}
}

// Close stops the subscriber and closes Redis connections.
func (s *Subscriber) Close() error {
	if err := s.pubsub.Close(); err != nil {
		return err
	}
	return s.client.Close()
}
