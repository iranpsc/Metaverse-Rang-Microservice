package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestIntervalIsThreeHours(t *testing.T) {
	if Interval != 3*time.Hour {
		t.Fatalf("interval = %s", Interval)
	}
}

func TestRunGeneratesImmediatelyThenOnTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var runs atomic.Int32
	done := make(chan struct{})
	go func() {
		_ = Run(ctx, 20*time.Millisecond, func(context.Context) error {
			if runs.Add(1) >= 2 {
				cancel()
			}
			return nil
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	if runs.Load() < 2 {
		t.Fatalf("runs = %d", runs.Load())
	}
}

func TestRunRejectsNilGenerate(t *testing.T) {
	err := Run(context.Background(), time.Second, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
