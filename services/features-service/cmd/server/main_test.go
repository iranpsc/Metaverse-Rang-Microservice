package main

import (
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	const key = "FEATURES_TEST_GETENV_KEY"
	t.Setenv(key, "custom")
	if got := getEnv(key, "default"); got != "custom" {
		t.Fatalf("got %q", got)
	}
	_ = os.Unsetenv(key)
	if got := getEnv(key, "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildMySQLDSN(t *testing.T) {
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "features_db")

	got := buildMySQLDSN()
	want := "app:secret@tcp(db.internal:3307)/features_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGrpcListenAddr(t *testing.T) {
	if got := grpcListenAddr("50053"); got != ":50053" {
		t.Fatalf("got %q", got)
	}
	if got := grpcListenAddr(""); got != ":50053" {
		t.Fatalf("got %q", got)
	}
}

func TestParsePositiveDuration(t *testing.T) {
	d, ok := parsePositiveDuration("3s")
	if !ok || d != 3*time.Second {
		t.Fatalf("got %v %v", d, ok)
	}
	if _, ok := parsePositiveDuration(""); ok {
		t.Fatal("expected empty to fail")
	}
	if _, ok := parsePositiveDuration("nope"); ok {
		t.Fatal("expected invalid to fail")
	}
}

func TestParsePositiveInt(t *testing.T) {
	n, ok := parsePositiveInt("3")
	if !ok || n != 3 {
		t.Fatalf("got %d %v", n, ok)
	}
	if _, ok := parsePositiveInt("0"); ok {
		t.Fatal("expected non-positive to fail")
	}
	if _, ok := parsePositiveInt("bad"); ok {
		t.Fatal("expected invalid to fail")
	}
}
