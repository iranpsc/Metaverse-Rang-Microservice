package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"metarang/shared/pkg/logger"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
	"metarang/social-service/internal/client"
	"metarang/social-service/internal/handler"
	"metarang/social-service/internal/repository"
	"metarang/social-service/internal/service"
)

func main() {
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/social-service/config.env",
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
		log.Println("Warning: config.env not found, using environment variables only")
	}

	structLog := logger.NewLogger("social-service")
	structLog.Info("Starting Social Service...")

	if err := sentry.InitFromEnv("social-service"); err != nil {
		structLog.Warn("Failed to initialize Sentry", "error", err)
	}
	defer sentry.Flush(2 * time.Second)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metarang_db"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		structLog.Fatal("Failed to connect to database", "error", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		structLog.Fatal("Failed to ping database", "error", err)
	}
	structLog.Info("Successfully connected to database")

	challengeRepo := repository.NewChallengeRepository(db)
	followRepo := repository.NewFollowRepository(db)
	userRepo := repository.NewUserRepository(db)

	var commercialClient client.CommercialClient
	commercialAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	commercialClient, err = client.NewCommercialClient(commercialAddr)
	if err != nil {
		structLog.Warn("Failed to connect to commercial service - wallet credits disabled", "error", err)
		commercialClient = nil
	} else {
		structLog.Info("Connected to commercial service", "addr", commercialAddr)
		defer commercialClient.Close()
	}

	var levelsClient client.LevelsClient
	levelsAddr := getEnv("LEVELS_SERVICE_ADDR", "levels-service:50054")
	levelsClient, err = client.NewLevelsClient(levelsAddr)
	if err != nil {
		structLog.Warn("Failed to connect to levels service - follower score updates disabled", "error", err)
		levelsClient = nil
	} else {
		structLog.Info("Connected to levels service", "addr", levelsAddr)
		defer levelsClient.Close()
	}

	var authClient client.AuthClient
	authAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authClient, err = client.NewAuthClient(authAddr)
	if err != nil {
		structLog.Warn("Failed to connect to auth service - follow authorization unavailable", "error", err)
		authClient = nil
	} else {
		structLog.Info("Connected to auth service", "addr", authAddr)
		defer authClient.Close()
	}

	challengeService := service.NewChallengeService(
		challengeRepo,
		commercialClient,
		service.ChallengeConfig{
			Locale:     getEnv("APP_LOCALE", getEnv("PROJECT_LOCALE", "EN")),
			ProjectURL: getEnv("PROJECT_URL", getEnv("APP_URL", "")),
		},
	)
	followService := service.NewFollowService(followRepo, userRepo, authClient, levelsClient)

	serviceMetrics := metrics.NewMetrics("social_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			logger.UnaryServerInterceptor(structLog),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)
	handler.RegisterChallengeHandler(grpcServer, challengeService)
	handler.RegisterFollowHandler(grpcServer, followService)
	reflection.Register(grpcServer)

	port := getEnv("GRPC_PORT", "50061")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		structLog.Fatal("Failed to listen", "error", err, "port", port)
	}

	structLog.Info("Social service listening", "port", port)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			structLog.Fatal("Failed to serve", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	structLog.Info("Shutting down server...")
	grpcServer.GracefulStop()
	structLog.Info("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
