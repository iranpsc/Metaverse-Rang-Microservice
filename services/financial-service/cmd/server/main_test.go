package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	const key = "FINANCIAL_TEST_GETENV_KEY"
	t.Setenv(key, "custom")
	if got := getEnv(key, "default"); got != "custom" {
		t.Fatalf("got %q", got)
	}

	_ = os.Unsetenv(key)
	if got := getEnv(key, "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}
