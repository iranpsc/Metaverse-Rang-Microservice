package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"metarang/notifications-service/internal/service"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Invalid value for %s: %v, falling back to default %d", key, err, defaultValue)
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		log.Printf("Invalid duration for %s: %v, falling back to default %s", key, err, defaultValue)
		return defaultValue
	}
	return value
}

func loadEnvFiles() bool {
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/notifications-service/config.env",
	}
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			log.Printf("Loaded config from: %s", configPath)
			return true
		}
	}
	return false
}

func warnIfConfigMissing(loaded bool) {
	if !loaded {
		log.Printf("Warning: config.env not found, using environment variables only")
	}
}

func loadSMSChannelConfig() service.SMSChannelConfig {
	return service.SMSChannelConfig{
		Provider: getEnv("SMS_PROVIDER", getEnv("KAVENEGAR_PROVIDER", "kavenegar")),
		APIKey:   service.ResolveSMSAPIKey(),
		Sender:   service.ResolveSMSSender(getEnv("SMS_SENDER", "10008663")),
	}
}

func logSMSConfig(cfg service.SMSChannelConfig) {
	if cfg.Provider == "" || service.IsPlaceholderSMSAPIKey(cfg.APIKey) {
		log.Printf("WARNING: SMS not fully configured (SMS_PROVIDER=%s, API key set=%v). SMS features will not work.", cfg.Provider, cfg.APIKey != "")
		log.Printf("Set SMS_API_KEY in services/notifications-service/config.env.")
		return
	}
	log.Printf("SMS configured: provider=%s, sender=%s, api_key_from=%s, api_key=%s",
		cfg.Provider, cfg.Sender, service.SMSAPIKeySource(), service.MaskAPIKey(cfg.APIKey))
}

func loadEmailChannelConfig() service.EmailChannelConfig {
	return service.EmailChannelConfig{
		Host:      getEnv("SMTP_HOST", ""),
		Port:      getEnv("SMTP_PORT", "587"),
		Username:  getEnv("SMTP_USERNAME", ""),
		Password:  getEnv("SMTP_PASSWORD", ""),
		FromName:  getEnv("SMTP_FROM_NAME", "metarang Notifications"),
		FromEmail: getEnv("SMTP_FROM_EMAIL", ""),
	}
}

func grpcListenAddr(port string) string {
	if port == "" {
		port = "50058"
	}
	return ":" + port
}

func configureDBPool(db *sql.DB) {
	db.SetMaxOpenConns(getEnvAsInt("DB_MAX_OPEN_CONNS", 25))
	db.SetMaxIdleConns(getEnvAsInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute))
}

func setupDatabase() (*sql.DB, error) {
	port, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT value: %w", err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		port,
		getEnv("DB_DATABASE", "metarang_db"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	configureDBPool(db)
	return db, nil
}

func pingDatabase(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}
