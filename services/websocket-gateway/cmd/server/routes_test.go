package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedisURLFromEnv(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://explicit:6379/1")
	if got := redisURLFromEnv(); got != "redis://explicit:6379/1" {
		t.Fatalf("explicit got %q", got)
	}

	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_HOST", "metarang-redis")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("REDIS_PASSWORD", "")
	if got := redisURLFromEnv(); got != "redis://metarang-redis:6379/0" {
		t.Fatalf("no password got %q", got)
	}

	t.Setenv("REDIS_PASSWORD", "p@ss")
	if got := redisURLFromEnv(); got != "redis://:p%40ss@metarang-redis:6379/0" {
		t.Fatalf("password got %q", got)
	}
}

func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/", "/tester"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/socket.io/?EIO=4&transport=polling", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("socket.io status = %d", rec.Code)
	}
}
