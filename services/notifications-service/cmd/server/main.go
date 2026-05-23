package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"metargb/notifications-service/internal/handler"
	"metargb/notifications-service/internal/repository"
	"metargb/notifications-service/internal/service"
	"metargb/shared/pkg/metrics"
)

func main() {
	// Load environment variables from config.env
	// Try multiple possible paths for config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/notifications-service/config.env",
	}
	var configLoaded bool
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			configLoaded = true
			log.Printf("Loaded config from: %s", configPath)
			break
		}
	}
	if !configLoaded {
		log.Printf("Warning: config.env not found, using environment variables only")
	}

	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to prepare database connection: %v", err)
	}
	defer db.Close()

	if err := pingDatabase(db); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

	notificationRepo := repository.NewNotificationRepository(db)

	// SMS config: SMS_API_KEY or KAVENEGAR_API_KEY (same key as Laravel auth / auth-service)
	smsCfg := service.SMSChannelConfig{
		Provider: getEnv("SMS_PROVIDER", getEnv("KAVENEGAR_PROVIDER", "kavenegar")),
		APIKey:   service.ResolveSMSAPIKey(),
		Sender:   service.ResolveSMSSender(getEnv("SMS_SENDER", "10008663")),
	}
	if smsCfg.Provider == "" || service.IsPlaceholderSMSAPIKey(smsCfg.APIKey) {
		log.Printf("WARNING: SMS not fully configured (SMS_PROVIDER=%s, API key set=%v). SMS features will not work.", smsCfg.Provider, smsCfg.APIKey != "")
		log.Printf("Set SMS_API_KEY in services/notifications-service/config.env (same value as laravel-api config/kavenegar.php apikey).")
	} else {
		log.Printf("SMS configured: provider=%s, sender=%s, api_key_from=%s, api_key=%s",
			smsCfg.Provider, smsCfg.Sender, service.SMSAPIKeySource(), service.MaskAPIKey(smsCfg.APIKey))
	}
	smsChannel := service.NewSMSChannel(smsCfg)
	emailChannel := service.NewEmailChannel()

	notificationService := service.NewNotificationService(notificationRepo, smsChannel, emailChannel)
	smsService := service.NewSMSService(smsChannel)
	emailService := service.NewEmailService(emailChannel)

	serviceMetrics := metrics.NewMetrics("notifications_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(serviceMetrics)),
	)

	handler.RegisterNotificationHandler(grpcServer, notificationService)
	handler.RegisterSMSHandler(grpcServer, smsService)
	handler.RegisterEmailHandler(grpcServer, emailService)

	port := getEnv("GRPC_PORT", "50058")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Notification service listening on port %s", port)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
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
		getEnv("DB_DATABASE", "metargb_db"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(getEnvAsInt("DB_MAX_OPEN_CONNS", 25))
	db.SetMaxIdleConns(getEnvAsInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute))

	return db, nil
}

func pingDatabase(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

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
