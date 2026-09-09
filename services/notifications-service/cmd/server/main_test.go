package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestGetEnv(t *testing.T) {
	const key = "NOTIFICATIONS_TEST_GETENV_KEY"
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
	const key = "NOTIFICATIONS_TEST_INT_KEY"
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
	const key = "NOTIFICATIONS_TEST_DURATION_KEY"
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
	if err := os.WriteFile(configPath, []byte("NOTIFICATIONS_TEST_ENV_KEY=loaded\n"), 0644); err != nil {
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
	if os.Getenv("NOTIFICATIONS_TEST_ENV_KEY") != "loaded" {
		t.Fatalf("env=%q", os.Getenv("NOTIFICATIONS_TEST_ENV_KEY"))
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

func TestGrpcListenAddr(t *testing.T) {
	if got := grpcListenAddr("50058"); got != ":50058" {
		t.Fatalf("got %q", got)
	}
	if got := grpcListenAddr(""); got != ":50058" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadSMSChannelConfigAndLog(t *testing.T) {
	t.Setenv("SMS_PROVIDER", "kavenegar")
	t.Setenv("SMS_API_KEY", "change-me")
	t.Setenv("KAVENEGAR_API_KEY", "")
	t.Setenv("SMS_SENDER", "10001")

	cfg := loadSMSChannelConfig()
	if cfg.Provider != "kavenegar" {
		t.Fatalf("provider=%q", cfg.Provider)
	}
	if cfg.Sender != "10001" {
		t.Fatalf("sender=%q", cfg.Sender)
	}
	logSMSConfig(cfg)

	t.Setenv("SMS_API_KEY", "real-kavenegar-api-key-value")
	cfg = loadSMSChannelConfig()
	if cfg.APIKey != "real-kavenegar-api-key-value" {
		t.Fatalf("apiKey=%q", cfg.APIKey)
	}
	logSMSConfig(cfg)

	t.Setenv("SMS_PROVIDER", "")
	t.Setenv("KAVENEGAR_PROVIDER", "")
	t.Setenv("SMS_API_KEY", "")
	cfg = loadSMSChannelConfig()
	logSMSConfig(cfg)
}

func TestLoadEmailChannelConfig(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM_NAME", "Meta")
	t.Setenv("SMTP_FROM_EMAIL", "noreply@example.com")

	cfg := loadEmailChannelConfig()
	if cfg.Host != "smtp.example.com" || cfg.Port != "465" || cfg.Username != "mailer" || cfg.FromEmail != "noreply@example.com" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestConfigureDBPool(t *testing.T) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/notifications_test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Setenv("DB_MAX_OPEN_CONNS", "11")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	t.Setenv("DB_CONN_MAX_LIFETIME", "2m")
	configureDBPool(db)
}

func TestSetupDatabase_InvalidPort(t *testing.T) {
	t.Setenv("DB_PORT", "not-a-port")
	if _, err := setupDatabase(); err == nil {
		t.Fatal("expected invalid DB_PORT error")
	}
}

func TestNewGRPCServer(t *testing.T) {
	srv, err := newGRPCServer()
	if err != nil {
		t.Fatal(err)
	}
	if srv == nil {
		t.Fatal("expected gRPC server")
	}
	srv.Stop()
}

func TestRegisterNotificationServices(t *testing.T) {
	t.Setenv("SMS_PROVIDER", "unknown")
	t.Setenv("SMS_API_KEY", "")
	t.Setenv("KAVENEGAR_API_KEY", "")

	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/notifications_test")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := grpc.NewServer()
	defer srv.Stop()

	h := registerNotificationServices(srv, db)
	if h == nil {
		t.Fatal("expected notification handler")
	}
}

func TestDialAuthClient(t *testing.T) {
	t.Setenv("AUTH_SERVICE_ADDR", "127.0.0.1:1")
	client, conn := dialAuthClient()
	if conn != nil {
		defer conn.Close()
	}
	if (client == nil) != (conn == nil) {
		t.Fatal("auth client and connection should both be set or both be nil")
	}
}

func TestSetupDatabaseAndPingFailure(t *testing.T) {
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "root")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")
	t.Setenv("DB_DATABASE", "metarang_db")

	db, err := setupDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := pingDatabase(db); err == nil {
		t.Fatal("expected ping failure against closed port")
	}
}
