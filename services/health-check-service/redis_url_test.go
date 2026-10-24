package main

import "testing"

func TestRedisURLFromEnv(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://explicit:6379/1")
	if got := redisURLFromEnv(); got != "redis://explicit:6379/1" {
		t.Fatalf("explicit got %q", got)
	}

	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("REDIS_PASSWORD", "")
	if got := redisURLFromEnv(); got != "redis://redis:6379/0" {
		t.Fatalf("no password got %q", got)
	}

	t.Setenv("REDIS_PASSWORD", "p@ss")
	if got := redisURLFromEnv(); got != "redis://:p%40ss@redis:6379/0" {
		t.Fatalf("password got %q", got)
	}
}
