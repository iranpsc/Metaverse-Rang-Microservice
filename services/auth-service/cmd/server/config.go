package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return n
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultValue
	}
	return d
}

func loadEnvFiles() bool {
	configPaths := []string{
		"services/auth-service/config.env",
		"config.env",
		"./config.env",
		"../../config.env",
	}
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
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

func buildDSN() string {
	user := getEnv("DB_USER", "root")
	password := getEnv("DB_PASSWORD", "")
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	database := getEnv("DB_DATABASE", "metarang_db")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local&tls=false&interpolateParams=true",
		user, password, host, port, database)
}

func buildMySQLConfig(dsn string) (*mysql.Config, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["charset"] = "utf8mb4"
	return cfg, nil
}

func openDatabase(cfg *mysql.Config) (*sql.DB, error) {
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	configureDBPool(db)
	return db, nil
}

func configureDBPool(db *sql.DB) {
	maxOpen := getEnvAsInt("DB_MAX_OPEN_CONNS", 25)
	maxIdle := getEnvAsInt("DB_MAX_IDLE_CONNS", 5)
	lifetime := getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	idleTime := getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(idleTime)
}

func pingDatabase(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func buildRedisURL() string {
	redisURL := getEnv("REDIS_URL", "")
	if redisURL != "" {
		return redisURL
	}
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := getEnv("REDIS_DB", "0")
	if redisPassword != "" {
		return fmt.Sprintf("redis://:%s@%s:%s/%s", redisPassword, redisHost, redisPort, redisDB)
	}
	return fmt.Sprintf("redis://%s:%s/%s", redisHost, redisPort, redisDB)
}

func newRedisOptions(redisURL string) (*redis.Options, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	opts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}
	return opts, nil
}

func resolveAPIGatewayURL() string {
	apiGatewayURL := getEnv("API_GATEWAY_URL", "")
	if apiGatewayURL != "" {
		return apiGatewayURL
	}
	return getEnv("APP_URL", "http://localhost:8000")
}

type runtimeConfig struct {
	AdminPanelURL         string
	NotificationsAddr     string
	LevelsServiceAddr     string
	FeaturesServiceAddr   string
	CommercialServiceAddr string
	AppName               string
	AppURL                string
	OAuthServerURL        string
	OAuthClientID         string
	OAuthClientSecret     string
	FrontEndURL           string
	AppEnv                string
	StorageServiceAddr    string
	MetricsPort           string
	ProjectLocale         string
	GRPCPort              string
	HTTPPort              string
}

func loadRuntimeConfig() runtimeConfig {
	cfg := runtimeConfig{}
	cfg.AdminPanelURL = getEnv("ADMIN_PANEL_URL", "")
	cfg.NotificationsAddr = getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications-service:50058")
	cfg.LevelsServiceAddr = getEnv("LEVELS_SERVICE_ADDR", "levels-service:50054")
	cfg.FeaturesServiceAddr = getEnv("FEATURES_SERVICE_ADDR", "features-service:50053")
	cfg.CommercialServiceAddr = getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	cfg.AppName = getEnv("APP_NAME", "Metarang")
	cfg.AppURL = getEnv("APP_URL", "http://localhost:8000")
	cfg.OAuthServerURL = getEnv("OAUTH_SERVER_URL", "")
	cfg.OAuthClientID = getEnv("OAUTH_CLIENT_ID", "")
	cfg.OAuthClientSecret = getEnv("OAUTH_CLIENT_SECRET", "")
	cfg.FrontEndURL = getEnv("FRONT_END_URL", "http://localhost:3000")
	cfg.AppEnv = getEnv("APP_ENV", "local")
	cfg.StorageServiceAddr = getEnv("STORAGE_SERVICE_ADDR", "storage-service:50060")
	cfg.MetricsPort = getEnv("METRICS_PORT", "9090")
	cfg.ProjectLocale = getEnv("PROJECT_LOCALE", "EN")
	cfg.GRPCPort = getEnv("GRPC_PORT", "50051")
	cfg.HTTPPort = getEnv("HTTP_PORT", "8066")
	return cfg
}

func grpcListenAddr(port string) string {
	if port == "" {
		port = "50051"
	}
	return ":" + port
}
