package sentry

import (
	"os"
	"testing"
	"time"
)

func TestInitFromEnvDisabledWithoutDSN(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")

	if err := InitFromEnv("test-service"); err != nil {
		t.Fatalf("InitFromEnv() error = %v", err)
	}
	if Enabled() {
		t.Fatal("expected Sentry to stay disabled without DSN")
	}

	Flush(100 * time.Millisecond)
}

func TestInitFromEnvInvalidSampleRate(t *testing.T) {
	t.Setenv("SENTRY_DSN", "https://examplePublicKey@o0.ingest.sentry.io/0")
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "not-a-number")

	if err := InitFromEnv("test-service"); err == nil {
		t.Fatal("expected error for invalid SENTRY_TRACES_SAMPLE_RATE")
	}
}

func TestFlushNoOpWhenDisabled(t *testing.T) {
	enabled = false
	Flush(100 * time.Millisecond)
}

func TestCaptureExceptionNoOpWhenDisabled(t *testing.T) {
	enabled = false
	CaptureException(os.ErrInvalid)
}
