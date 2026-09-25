package main

import (
	"testing"
	"time"
)

func TestParseDurationHours(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"3", 3 * time.Hour},
		{"1", time.Hour},
		{"", 3 * time.Hour},
		{"bad", 3 * time.Hour},
		{"0", 3 * time.Hour},
		{"-1", 3 * time.Hour},
	}
	for _, tc := range cases {
		got := parseDurationHours(tc.raw, 3*time.Hour)
		if got != tc.want {
			t.Fatalf("raw %q: got %v want %v", tc.raw, got, tc.want)
		}
	}
}
