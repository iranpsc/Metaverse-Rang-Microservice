package service

import (
	"context"
	"testing"
	"time"
)

func TestNotifyContext_IgnoresParentCancel(t *testing.T) {
	s := &JoinRequestService{}
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, stop := s.notifyContext(parent)
	defer stop()

	if err := parent.Err(); err == nil {
		t.Fatal("expected parent context to be canceled")
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("notify context should ignore parent cancel: %v", err)
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected notify context deadline")
	}
	if remaining := time.Until(deadline); remaining < 15*time.Second || remaining > joinNotificationTimeout {
		t.Fatalf("unexpected remaining deadline %v", remaining)
	}
}
