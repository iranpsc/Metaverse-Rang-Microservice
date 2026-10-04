package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryStopsOnSuccessAndNonRetryable(t *testing.T) {
	var calls int
	err := Retry(context.Background(), 3, time.Millisecond, func(error) bool { return true }, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	calls = 0
	permanent := errors.New("nope")
	err = Retry(context.Background(), 3, time.Millisecond, func(err error) bool {
		return !errors.Is(err, permanent)
	}, func() error {
		calls++
		return permanent
	})
	if !errors.Is(err, permanent) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetryHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := Retry(ctx, 3, time.Second, func(error) bool { return true }, func() error {
		calls++
		return errors.New("temporary")
	})
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWithDefaultTimeout(t *testing.T) {
	ctx, cancel := WithDefaultTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected deadline")
	}

	parent, parentCancel := context.WithTimeout(context.Background(), time.Hour)
	defer parentCancel()
	child, childCancel := WithDefaultTimeout(parent, time.Millisecond)
	defer childCancel()
	parentDeadline, _ := parent.Deadline()
	childDeadline, ok := child.Deadline()
	if !ok || !childDeadline.Equal(parentDeadline) {
		t.Fatal("existing deadline should be preserved")
	}
}
