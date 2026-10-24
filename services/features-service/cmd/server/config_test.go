package main

import (
	"strings"
	"testing"
	"time"
)

func TestServerConfigHelpers(t *testing.T) {
	t.Setenv("DB_USER", "tester")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "test_db")
	if dsn := buildMySQLDSN(); !strings.HasPrefix(dsn, "tester:password@tcp(localhost:3307)/test_db?") {
		t.Fatalf("dsn=%q", dsn)
	}
	if got := getEnv("DB_USER", "fallback"); got != "tester" {
		t.Fatal(got)
	}
	if got := getEnv("UNSET_FEATURE_CONFIG_TEST", "fallback"); got != "fallback" {
		t.Fatal(got)
	}
	if got := grpcListenAddr(""); got != ":50053" {
		t.Fatal(got)
	}
	if got := grpcListenAddr("1234"); got != ":1234" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		raw  string
		want time.Duration
		ok   bool
	}{
		{"", 0, false}, {"bad", 0, false}, {"2s", 2 * time.Second, true},
	} {
		got, ok := parsePositiveDuration(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("duration %q=%v,%v", tc.raw, got, ok)
		}
	}
	for _, tc := range []struct {
		raw  string
		want int
		ok   bool
	}{
		{"bad", 0, false}, {"0", 0, false}, {"3", 3, true},
	} {
		got, ok := parsePositiveInt(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("integer %q=%d,%v", tc.raw, got, ok)
		}
	}
}
