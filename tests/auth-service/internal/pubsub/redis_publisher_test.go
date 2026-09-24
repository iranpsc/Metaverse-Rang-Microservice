package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"metarang/auth-service/internal/pubsub"
)

func TestNewRedisPublisher_InvalidURL(t *testing.T) {
	_, err := pubsub.NewRedisPublisher("://bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRedisPublisher_PingFail(t *testing.T) {
	_, err := pubsub.NewRedisPublisher("redis://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestRedisPublisher_PublishAndClose(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	pub, err := pubsub.NewRedisPublisher("redis://" + mr.Addr())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	sub := client.Subscribe(ctx, "user-status")
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ch := sub.Channel()
	if err := pub.PublishUserStatusChanged(ctx, 42, true); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		if msg.Payload == "" {
			t.Fatal("empty payload")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for publish")
	}

	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
}
