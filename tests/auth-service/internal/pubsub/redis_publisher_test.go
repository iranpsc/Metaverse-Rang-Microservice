package pubsub_test

import (
	"context"
	"encoding/json"
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

	msg := waitPublish(t, ch)
	var env struct {
		Data struct {
			UserID string `json:"user_id"`
			Online bool   `json:"online"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != "42" || !env.Data.Online {
		t.Fatalf("payload = %s", msg.Payload)
	}

	if err := pub.PublishUserStatusChanged(ctx, 42, true); err != nil {
		t.Fatal(err)
	}
	expectNoPublish(t, ch)

	if err := pub.PublishUserStatusChanged(ctx, 42, false); err != nil {
		t.Fatal(err)
	}
	offline := waitPublish(t, ch)
	if err := json.Unmarshal([]byte(offline.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != "42" || env.Data.Online {
		t.Fatalf("offline payload = %s", offline.Payload)
	}
	if err := pub.PublishUserStatusChanged(ctx, 42, false); err != nil {
		t.Fatal(err)
	}
	expectNoPublish(t, ch)

	if err := pub.PublishUserStatusChanged(ctx, 42, true); err != nil {
		t.Fatal(err)
	}
	back := waitPublish(t, ch)
	if err := json.Unmarshal([]byte(back.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != "42" || !env.Data.Online {
		t.Fatalf("online again payload = %s", back.Payload)
	}

	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitPublish(t *testing.T, ch <-chan *redis.Message) *redis.Message {
	t.Helper()
	select {
	case msg := <-ch:
		if msg.Payload == "" {
			t.Fatal("empty payload")
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for publish")
	}
	return nil
}

func expectNoPublish(t *testing.T, ch <-chan *redis.Message) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Fatalf("unexpected publish: %s", msg.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}
