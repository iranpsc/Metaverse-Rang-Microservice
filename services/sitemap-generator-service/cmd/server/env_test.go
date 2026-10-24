package main

import "testing"

func TestGetEnv(t *testing.T) {
	t.Setenv("SITEMAP_TEST_SETTING", "configured")
	if got := getEnv("SITEMAP_TEST_SETTING", "fallback"); got != "configured" {
		t.Fatalf("configured=%q", got)
	}
	t.Setenv("SITEMAP_TEST_SETTING", "")
	if got := getEnv("SITEMAP_TEST_SETTING", "fallback"); got != "fallback" {
		t.Fatalf("fallback=%q", got)
	}
}
