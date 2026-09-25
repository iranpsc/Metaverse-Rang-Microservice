package scheduler

import (
	"context"
	"log"
	"time"
)

// Interval is how often sitemap files are regenerated.
const Interval = 3 * time.Hour

// GenerateFunc writes sitemap files once.
type GenerateFunc func(ctx context.Context) error

// Run calls generate immediately, then every interval, until ctx is cancelled.
// A failed run is logged and the next tick is still scheduled.
func Run(ctx context.Context, interval time.Duration, generate GenerateFunc) error {
	if interval <= 0 {
		interval = Interval
	}
	if generate == nil {
		return errNilGenerate
	}

	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, interval)
		err := generate(runCtx)
		cancel()
		if err != nil {
			log.Printf("sitemap generation failed: %v", err)
		}
	}

	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			run()
		}
	}
}
