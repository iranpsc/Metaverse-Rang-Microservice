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
	again := waitPublish(t, ch)
	if err := json.Unmarshal([]byte(again.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != "42" || !env.Data.Online {
		t.Fatalf("repeat online payload = %s", again.Payload)
	}

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

	if err := pub.PublishUserStatusChanged(ctx, 42, false); err != nil {
		t.Fatal(err)
	}
	quiet := waitPublish(t, ch)
	if err := json.Unmarshal([]byte(quiet.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != "42" || env.Data.Online {
		t.Fatalf("offline after new activity payload = %s", quiet.Payload)
	}

	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisPublisher_LogoutBroadcastsOfflineAndBlocksOnline(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	pub, err := pubsub.NewRedisPublisher("redis://" + mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	sub := client.Subscribe(ctx, "user-status")
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	ch := sub.Channel()

	// Quiet period already claimed. Explicit logout must still broadcast.
	if err := pub.PublishUserStatusChanged(ctx, 42, false); err != nil {
		t.Fatal(err)
	}
	first := waitPublish(t, ch)
	if payloadOnline(t, first.Payload) {
		t.Fatalf("expected offline, payload=%s", first.Payload)
	}
	if err := pub.PublishUserLoggedOut(ctx, 42); err != nil {
		t.Fatal(err)
	}
	logout := waitPublish(t, ch)
	if payloadOnline(t, logout.Payload) {
		t.Fatalf("logout payload = %s", logout.Payload)
	}

	err = pub.PublishUserStatusChanged(ctx, 42, true)
	if err != pubsub.ErrOnlineSuppressed {
		t.Fatalf("expected ErrOnlineSuppressed, got %v", err)
	}
	expectNoPublish(t, ch)

	if err := pub.PublishUserLoggedIn(ctx, 42); err != nil {
		t.Fatal(err)
	}
	login := waitPublish(t, ch)
	if !payloadOnline(t, login.Payload) {
		t.Fatalf("login payload = %s", login.Payload)
	}

	if err := pub.PublishUserStatusChanged(ctx, 42, true); err != nil {
		t.Fatal(err)
	}
	again := waitPublish(t, ch)
	if !payloadOnline(t, again.Payload) {
		t.Fatalf("online after login payload = %s", again.Payload)
	}
}

func payloadOnline(t *testing.T, payload string) bool {
	t.Helper()
	var env struct {
		Data struct {
			Online bool `json:"online"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data.Online
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
