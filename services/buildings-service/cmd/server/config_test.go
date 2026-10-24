package main

import (
	"testing"
	"time"
)

func TestServerConfigHelpers(t *testing.T) {
	t.Setenv("BUILDINGS_CONFIG_TEST", "configured")
	if got := getEnv("BUILDINGS_CONFIG_TEST", "fallback"); got != "configured" {
		t.Fatal(got)
	}
	t.Setenv("BUILDINGS_CONFIG_TEST", "")
	if got := getEnv("BUILDINGS_CONFIG_TEST", "fallback"); got != "fallback" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		raw  string
		want time.Duration
		ok   bool
	}{
		{"", 0, false}, {"invalid", 0, false}, {"3s", 3 * time.Second, true},
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
		{"invalid", 0, false}, {"0", 0, false}, {"3", 3, true},
	} {
		got, ok := parsePositiveInt(tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("int %q=%d,%v", tc.raw, got, ok)
		}
	}
}
