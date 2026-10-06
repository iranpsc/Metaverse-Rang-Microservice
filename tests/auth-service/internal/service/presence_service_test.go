package service_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"metarang/auth-service/internal/pubsub"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
)

type fakePresenceStore struct {
	mu       sync.Mutex
	ids      []uint64
	updated  []uint64
	onUpdate func(uint64)
}

func (f *fakePresenceStore) UpdateLastSeen(_ context.Context, userID uint64) error {
	f.mu.Lock()
	f.updated = append(f.updated, userID)
	f.mu.Unlock()
	if f.onUpdate != nil {
		f.onUpdate(userID)
	}
	return nil
}

func (f *fakePresenceStore) ListUserIDsByLastSeen(_ context.Context, _, _, cursorAt time.Time, cursorID uint64, limit int) ([]repository.LastSeenUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repository.LastSeenUser
	for _, id := range f.ids {
		at := cursorAt
		if cursorID != 0 && id <= cursorID {
			continue
		}
		out = append(out, repository.LastSeenUser{ID: id, LastSeen: at.Add(time.Second)})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestPresenceTouchThrottlesAndPublishesOnlineOnTransition(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := pubsub.NewRedisPublisher("redis://" + mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sub := client.Subscribe(ctx, "user-status")
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	messages := sub.Channel()

	updated := make(chan uint64, 4)
	store := &fakePresenceStore{onUpdate: func(id uint64) { updated <- id }}

	now := time.Now()
	var mu sync.Mutex
	svc := service.NewPresenceService(store, pub, nil, service.WithPresenceNow(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}))
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.Start(runCtx)

	svc.Touch(42)
	waitUpdated(t, updated, 42)
	assertStatus(t, waitStatus(t, messages), "42", true)

	svc.Touch(42)
	expectNoUpdate(t, updated)
	expectNoStatus(t, messages)

	mu.Lock()
	now = now.Add(31 * time.Second)
	mu.Unlock()
	svc.Touch(42)
	waitUpdated(t, updated, 42)
	expectNoStatus(t, messages)

	mu.Lock()
	now = now.Add(31 * time.Second)
	mu.Unlock()
	mr.FastForward(3 * time.Minute)
	svc.Touch(42)
	waitUpdated(t, updated, 42)
	assertStatus(t, waitStatus(t, messages), "42", true)
}

func TestPresenceSweepAnnouncesOfflineOnce(t *testing.T) {
	mr := miniredis.RunT(t)
	pub, err := pubsub.NewRedisPublisher("redis://" + mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sub := client.Subscribe(ctx, "user-status")
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	messages := sub.Channel()

	store := &fakePresenceStore{ids: []uint64{9, 10}}
	svc := service.NewPresenceService(store, pub, client)

	svc.Sweep(ctx)
	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		msg := waitStatus(t, messages)
		var env struct {
			Data struct {
				UserID string `json:"user_id"`
				Online bool   `json:"online"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Online {
			t.Fatalf("expected offline, payload=%s", msg.Payload)
		}
		got[env.Data.UserID] = true
	}
	if !got["9"] || !got["10"] {
		t.Fatalf("announced %v", got)
	}

	mr.FastForward(30 * time.Second)
	svc.Sweep(ctx)
	expectNoStatus(t, messages)
}

func waitUpdated(t *testing.T, updated <-chan uint64, want uint64) {
	t.Helper()
	select {
	case id := <-updated:
		if id != want {
			t.Fatalf("updated %d, want %d", id, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for last_seen update")
	}
}

func expectNoUpdate(t *testing.T, updated <-chan uint64) {
	t.Helper()
	select {
	case id := <-updated:
		t.Fatalf("unexpected last_seen update for %d", id)
	case <-time.After(150 * time.Millisecond):
	}
}

func waitStatus(t *testing.T, messages <-chan *redis.Message) *redis.Message {
	t.Helper()
	select {
	case msg := <-messages:
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for status event")
	}
	return nil
}

func expectNoStatus(t *testing.T, messages <-chan *redis.Message) {
	t.Helper()
	select {
	case msg := <-messages:
		t.Fatalf("unexpected status event: %s", msg.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}

func assertStatus(t *testing.T, msg *redis.Message, userID string, online bool) {
	t.Helper()
	var env struct {
		Data struct {
			UserID string `json:"user_id"`
			Online bool   `json:"online"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UserID != userID || env.Data.Online != online {
		t.Fatalf("payload=%s", msg.Payload)
	}
}
