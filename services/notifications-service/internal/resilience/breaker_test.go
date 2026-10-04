package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreakerOpensAndAllowsTrial(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	b := NewBreaker(2, time.Second)
	b.now = func() time.Time { return current }

	boom := errors.New("down")
	isFailure := func(err error) bool { return errors.Is(err, boom) }

	if err := b.Execute(func() error { return boom }, isFailure); err != boom {
		t.Fatalf("first err=%v", err)
	}
	if err := b.Execute(func() error { return boom }, isFailure); !errors.Is(err, boom) {
		t.Fatalf("second err=%v", err)
	}

	calls := 0
	err := b.Execute(func() error {
		calls++
		return nil
	}, isFailure)
	if !errors.Is(err, ErrCircuitOpen) || calls != 0 {
		t.Fatalf("open breaker err=%v calls=%d", err, calls)
	}

	current = current.Add(time.Second)
	if err := b.Execute(func() error { return nil }, isFailure); err != nil {
		t.Fatalf("half-open trial err=%v", err)
	}
	if err := b.Execute(func() error { return nil }, isFailure); err != nil {
		t.Fatalf("closed breaker err=%v", err)
	}
}

func TestBreakerIgnoresCancellationAndBusinessErrors(t *testing.T) {
	b := NewBreaker(1, time.Minute)
	calls := 0
	business := errors.New("invalid")
	isFailure := func(err error) bool { return !errors.Is(err, business) }

	if err := b.Execute(func() error { return business }, isFailure); err != business {
		t.Fatalf("business err=%v", err)
	}
	if err := b.Execute(func() error {
		calls++
		return nil
	}, isFailure); err != nil || calls != 1 {
		t.Fatalf("business error opened breaker err=%v calls=%d", err, calls)
	}

	b = NewBreaker(1, time.Minute)
	_ = b.Execute(func() error { return context.Canceled }, func(error) bool { return true })
	if err := b.Execute(func() error {
		calls++
		return nil
	}, func(error) bool { return true }); err != nil {
		t.Fatalf("cancel opened breaker: %v", err)
	}
}

func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	b := NewBreaker(1, time.Second)
	b.now = func() time.Time { return current }

	_ = b.Execute(func() error { return errors.New("down") }, func(error) bool { return true })
	current = current.Add(time.Second)
	if err := b.Execute(func() error { return errors.New("still down") }, func(error) bool { return true }); err == nil {
		t.Fatal("expected trial failure")
	}
	if err := b.Execute(func() error { return nil }, func(error) bool { return true }); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected reopen, got %v", err)
	}
}
