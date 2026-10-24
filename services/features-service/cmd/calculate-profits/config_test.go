package main

import (
	"strings"
	"testing"
)

func TestProfitJobDSN(t *testing.T) {
	t.Setenv("DB_USER", "profit_test")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "profits")
	if got := buildMySQLDSN(); !strings.HasPrefix(got, "profit_test:password@tcp(localhost:3307)/profits?") {
		t.Fatalf("dsn=%q", got)
	}
	if got := getEnv("DB_USER", "fallback"); got != "profit_test" {
		t.Fatal(got)
	}
	if got := getEnv("UNSET_PROFIT_CONFIG_TEST", "fallback"); got != "fallback" {
		t.Fatal(got)
	}
}
