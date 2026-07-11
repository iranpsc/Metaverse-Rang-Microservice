package sentry

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
)

var enabled bool

// InitFromEnv initializes Sentry when SENTRY_DSN is set.
// When the DSN is empty, Sentry stays disabled and this returns nil.
func InitFromEnv(serviceName string) error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return nil
	}

	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = os.Getenv("APP_ENV")
	}
	if environment == "" {
		environment = "development"
	}

	tracesSampleRate := 0.0
	if value := os.Getenv("SENTRY_TRACES_SAMPLE_RATE"); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid SENTRY_TRACES_SAMPLE_RATE: %w", err)
		}
		tracesSampleRate = parsed
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          os.Getenv("SENTRY_RELEASE"),
		TracesSampleRate: tracesSampleRate,
		EnableTracing:    tracesSampleRate > 0,
		ServerName:       serviceName,
		AttachStacktrace: true,
	})
	if err != nil {
		return fmt.Errorf("sentry init: %w", err)
	}

	enabled = true
	return nil
}

// Enabled reports whether Sentry was initialized with a DSN.
func Enabled() bool {
	return enabled
}

// Flush waits for buffered Sentry events to be delivered.
func Flush(timeout time.Duration) {
	if enabled {
		sentry.Flush(timeout)
	}
}

// CaptureException sends an error to Sentry when enabled.
func CaptureException(err error) {
	if enabled && err != nil {
		sentry.CaptureException(err)
	}
}
