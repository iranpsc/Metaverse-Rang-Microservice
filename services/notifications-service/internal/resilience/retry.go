package resilience

import (
	"context"
	"time"
)

// Retry calls fn up to attempts times. Backoff doubles after the first wait (initial, 2x, 4x).
// retryable decides whether another attempt is worthwhile. A canceled context stops the loop.
func Retry(ctx context.Context, attempts int, initialBackoff time.Duration, retryable func(error) bool, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		if attempt > 0 && initialBackoff > 0 {
			wait := initialBackoff << (attempt - 1)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				if last != nil {
					return last
				}
				return ctx.Err()
			case <-timer.C:
			}
		}

		last = fn()
		if last == nil || retryable == nil || !retryable(last) {
			return last
		}
	}
	return last
}

// WithDefaultTimeout returns ctx when it already has a deadline.
// Otherwise it returns a child context that expires after d.
func WithDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	if d <= 0 {
		d = 10 * time.Second
	}
	return context.WithTimeout(ctx, d)
}
