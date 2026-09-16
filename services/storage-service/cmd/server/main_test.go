package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnv(t *testing.T) {
	const key = "STORAGE_TEST_GETENV_KEY"
	t.Setenv(key, "custom")
	if got := getEnv(key, "default"); got != "custom" {
		t.Fatalf("got %q, want custom", got)
	}

	_ = os.Unsetenv(key)
	if got := getEnv(key, "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
}

func TestBuildDSN(t *testing.T) {
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "storage_db")

	dsn := buildDSN()
	want := "app:secret@tcp(db.internal:3307)/storage_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local"
	if dsn != want {
		t.Fatalf("buildDSN() = %q, want %q", dsn, want)
	}
}

func TestLoadRuntimeConfig(t *testing.T) {
	t.Setenv("TEMP_DIR", "/tmp/chunks")
	t.Setenv("UPLOAD_DIR", "/data/uploads")
	t.Setenv("GRPC_PORT", "5001")
	t.Setenv("HTTP_PORT", "8001")
	t.Setenv("METRICS_PORT", "9001")
	t.Setenv("FTP_HOST", "ftp.local")
	t.Setenv("FTP_PORT", "2121")
	t.Setenv("FTP_USER", "u")
	t.Setenv("FTP_PASSWORD", "p")
	t.Setenv("FTP_BASE_URL", "http://cdn/files")

	cfg := loadRuntimeConfig()
	if cfg.TempDir != "/tmp/chunks" || cfg.UploadsDir != "/data/uploads" || cfg.GRPCPort != "5001" ||
		cfg.HTTPPort != "8001" || cfg.MetricsPort != "9001" || cfg.FTPHost != "ftp.local" ||
		cfg.FTPPort != "2121" || cfg.FTPUser != "u" || cfg.FTPPassword != "p" || cfg.FTPBaseURL != "http://cdn/files" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestEnsureUploadsDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "uploads")
	if err := ensureUploadsDir(dir); err != nil {
		t.Fatalf("ensureUploadsDir: %v", err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("Stat: err=%v st=%+v", err, st)
	}
}

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	if err := os.WriteFile(configPath, []byte("STORAGE_TEST_ENV_KEY=loaded\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if !loadEnvFiles() {
		t.Fatal("expected config.env to load")
	}
	if os.Getenv("STORAGE_TEST_ENV_KEY") != "loaded" {
		t.Fatalf("env = %q", os.Getenv("STORAGE_TEST_ENV_KEY"))
	}
}

func TestOpenDatabase_InvalidDSN(t *testing.T) {
	if _, err := openDatabase("invalid-dsn-without-driver-syntax"); err == nil {
		t.Fatal("expected openDatabase to fail for invalid DSN")
	}
}

func TestOpenDatabase_PingFailure(t *testing.T) {
	_, err := openDatabase("root:root@tcp(127.0.0.1:1)/metarang_db?timeout=500ms")
	if err == nil {
		t.Fatal("expected ping failure against closed port")
	}
}
