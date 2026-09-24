package redisbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"metarang/websocket-gateway/internal/redisbus"
)

type recordingBroadcaster struct {
	mu            sync.Mutex
	user          []map[string]any
	feature       []map[string]any
	notifications []map[string]any
}

func (r *recordingBroadcaster) BroadcastUserStatus(data map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.user = append(r.user, data)
}

func (r *recordingBroadcaster) BroadcastFeatureStatus(data map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.feature = append(r.feature, data)
}

func (r *recordingBroadcaster) BroadcastNotification(data map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications = append(r.notifications, data)
}

func TestSubscriberForwardsRedisChannels(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	recorder := &recordingBroadcaster{}
	sub, err := redisbus.NewSubscriber(ctx, "redis://"+mr.Addr(), recorder)
	if err != nil {
		t.Fatalf("NewSubscriber: %v", err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	// Wait until the client is subscribed before publishing.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mr.PubSubNumSub("feature-status")["feature-status"] > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mr.Publish("feature-status", `{"id":1001,"rgb":"G"}`)
	mr.Publish("user-status", `{"user_id":42,"online":true}`)
	mr.Publish("notifications", `{"user_id":42,"title":"hi"}`)

	waitFor(t, 2*time.Second, func() bool {
		recorder.mu.Lock()
		defer recorder.mu.Unlock()
		return len(recorder.feature) == 1 && len(recorder.user) == 1 && len(recorder.notifications) == 1
	})

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.feature[0]["rgb"] != "G" {
		t.Fatalf("feature payload = %#v", recorder.feature[0])
	}
	if recorder.user[0]["online"] != true {
		t.Fatalf("user payload = %#v", recorder.user[0])
	}
	if recorder.notifications[0]["title"] != "hi" {
		t.Fatalf("notification payload = %#v", recorder.notifications[0])
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for redis forwarding")
}
