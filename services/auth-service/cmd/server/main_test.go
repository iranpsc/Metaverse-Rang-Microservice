package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	const key = "AUTH_TEST_GETENV_KEY"
	t.Setenv(key, "custom")
	if got := getEnv(key, "default"); got != "custom" {
		t.Fatalf("got %q", got)
	}
	_ = os.Unsetenv(key)
	if got := getEnv(key, "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	const key = "AUTH_TEST_INT_KEY"
	t.Setenv(key, "42")
	if got := getEnvAsInt(key, 10); got != 42 {
		t.Fatalf("got %d", got)
	}
	t.Setenv(key, "invalid")
	if got := getEnvAsInt(key, 10); got != 10 {
		t.Fatalf("got %d", got)
	}
	_ = os.Unsetenv(key)
	if got := getEnvAsInt(key, 10); got != 10 {
		t.Fatalf("got %d", got)
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	const key = "AUTH_TEST_DURATION_KEY"
	t.Setenv(key, "2h")
	if got := getEnvAsDuration(key, time.Minute); got != 2*time.Hour {
		t.Fatalf("got %v", got)
	}
	t.Setenv(key, "bad")
	if got := getEnvAsDuration(key, time.Minute); got != time.Minute {
		t.Fatalf("got %v", got)
	}
	_ = os.Unsetenv(key)
	if got := getEnvAsDuration(key, time.Minute); got != time.Minute {
		t.Fatalf("got %v", got)
	}
}

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	if err := os.WriteFile(configPath, []byte("AUTH_TEST_ENV_KEY=loaded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if !loadEnvFiles() {
		t.Fatal("expected config.env to load")
	}
	if os.Getenv("AUTH_TEST_ENV_KEY") != "loaded" {
		t.Fatalf("env=%q", os.Getenv("AUTH_TEST_ENV_KEY"))
	}
}

func TestLoadEnvFiles_Missing(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if loadEnvFiles() {
		t.Fatal("expected missing config.env")
	}
}

func TestWarnIfConfigMissing(t *testing.T) {
	warnIfConfigMissing(true)
	warnIfConfigMissing(false)
}

func TestBuildDSN(t *testing.T) {
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "auth_db")

	dsn := buildDSN()
	want := "app:secret@tcp(db.internal:3307)/auth_db?parseTime=true&charset=utf8mb4&loc=Local&tls=false&interpolateParams=true"
	if dsn != want {
		t.Fatalf("got %q want %q", dsn, want)
	}
}

func TestBuildMySQLConfig(t *testing.T) {
	cfg, err := buildMySQLConfig(buildDSN())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Params["charset"] != "utf8mb4" {
		t.Fatalf("charset=%q", cfg.Params["charset"])
	}
	if _, err := buildMySQLConfig(":::not-a-dsn"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestBuildRedisURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://explicit:6379/1")
	if got := buildRedisURL(); got != "redis://explicit:6379/1" {
		t.Fatalf("got %q", got)
	}

	_ = os.Unsetenv("REDIS_URL")
	t.Setenv("REDIS_HOST", "rhost")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_DB", "2")
	_ = os.Unsetenv("REDIS_PASSWORD")
	if got := buildRedisURL(); got != "redis://rhost:6380/2" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("REDIS_PASSWORD", "pw")
	if got := buildRedisURL(); got != "redis://:pw@rhost:6380/2" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("REDIS_PASSWORD", "p@ss")
	if got := buildRedisURL(); got != "redis://:p%40ss@rhost:6380/2" {
		t.Fatalf("got %q", got)
	}
}

func TestNewRedisOptions(t *testing.T) {
	opts, err := newRedisOptions("redis://localhost:6379/0")
	if err != nil {
		t.Fatal(err)
	}
	if opts == nil || opts.MaintNotificationsConfig == nil {
		t.Fatalf("opts=%+v", opts)
	}
	if _, err := newRedisOptions("://bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveAPIGatewayURL(t *testing.T) {
	t.Setenv("API_GATEWAY_URL", "https://gw")
	if got := resolveAPIGatewayURL(); got != "https://gw" {
		t.Fatalf("got %q", got)
	}
	_ = os.Unsetenv("API_GATEWAY_URL")
	t.Setenv("APP_URL", "https://app")
	if got := resolveAPIGatewayURL(); got != "https://app" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadRuntimeConfig(t *testing.T) {
	t.Setenv("ADMIN_PANEL_URL", "https://admin")
	t.Setenv("GRPC_PORT", "5111")
	t.Setenv("HTTP_PORT", "8111")
	t.Setenv("PROJECT_LOCALE", "fa")
	t.Setenv("APP_NAME", "TestApp")

	cfg := loadRuntimeConfig()
	if cfg.AdminPanelURL != "https://admin" || cfg.GRPCPort != "5111" || cfg.HTTPPort != "8111" ||
		cfg.ProjectLocale != "fa" || cfg.AppName != "TestApp" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.LevelsServiceAddr == "" || cfg.NotificationsAddr == "" {
		t.Fatalf("defaults missing: %+v", cfg)
	}
}

func TestConfigureDBPool(t *testing.T) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/auth_test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Setenv("DB_MAX_OPEN_CONNS", "11")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	t.Setenv("DB_CONN_MAX_LIFETIME", "2m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "1m")
	configureDBPool(db)
}

func TestOpenDatabaseAndPingFailure(t *testing.T) {
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "root")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")
	t.Setenv("DB_DATABASE", "metarang_db")

	cfg, err := buildMySQLConfig(buildDSN() + "&timeout=500ms")
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := pingDatabase(db); err == nil {
		t.Fatal("expected ping failure against closed port")
	}
}

func TestGrpcListenAddr(t *testing.T) {
	if got := grpcListenAddr("50051"); got != ":50051" {
		t.Fatalf("got %q", got)
	}
	if got := grpcListenAddr(""); got != ":50051" {
		t.Fatalf("got %q", got)
	}
}
