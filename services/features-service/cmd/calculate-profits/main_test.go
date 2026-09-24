package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	const key = "FEATURES_CALC_TEST_GETENV_KEY"
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
	t.Setenv("DB_USER", "cron")
	t.Setenv("DB_PASSWORD", "pw")
	t.Setenv("DB_HOST", "mysql")
	t.Setenv("DB_PORT", "3306")
	t.Setenv("DB_DATABASE", "metarang_db")

	got := buildMySQLDSN()
	want := "cron:pw@tcp(mysql:3306)/metarang_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
