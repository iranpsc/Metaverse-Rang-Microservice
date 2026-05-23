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

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"metargb/training-service/internal/client"
	"metargb/training-service/internal/handler"
	"metargb/training-service/internal/repository"
	"metargb/training-service/internal/service"
	"metargb/shared/pkg/metrics"
)

func main() {
	// Panic recovery to catch any early failures
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Panic: %v", r)
		}
	}()

	// Load environment variables from config.env
	// Try multiple possible paths for config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/training-service/config.env",
	}
	var configLoaded bool
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			configLoaded = true
			break
		}
	}
	if !configLoaded {
		log.Printf("Warning: config.env not found, using environment variables only")
	}

	// Database connection with proper UTF-8 encoding for Persian/Farsi text
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local&tls=false&interpolateParams=true",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metargb_db"),
	)

	// Parse DSN to get config
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		log.Fatalf("Failed to parse DSN: %v", err)
	}

	// Ensure charset is explicitly set to utf8mb4 in connection parameters
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["charset"] = "utf8mb4"

	// Create connector with proper charset configuration
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	// Open database using connector
	db := sql.OpenDB(connector)
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Explicitly set charset to UTF-8 for proper Persian/Farsi text handling
	if _, err := db.ExecContext(ctx, "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		log.Printf("Warning: Failed to set charset to utf8mb4: %v", err)
	} else {
		log.Println("Successfully set database charset to utf8mb4 for UTF-8/Persian text support")
	}

	log.Println("Successfully connected to database")

	// Initialize Auth Service client (optional - falls back to direct DB queries)
	var authClient *client.AuthClient
	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authClient, err = client.NewAuthClient(authServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to auth service at %s: %v (falling back to direct DB queries)", authServiceAddr, err)
		authClient = nil
	} else {
		defer authClient.Close()
		log.Printf("Successfully connected to auth service at %s", authServiceAddr)
	}

	// Initialize repositories
	videoRepo := repository.NewVideoRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	userRepo := repository.NewUserRepository(db, authClient)

	// Set project locale for validation messages
	handler.SetProjectLocale(getEnv("PROJECT_LOCALE", "EN"))

	// Initialize services
	videoService := service.NewVideoService(videoRepo, categoryRepo, userRepo)
	categoryService := service.NewCategoryService(categoryRepo, videoRepo)
	commentService := service.NewCommentService(commentRepo, userRepo)
	replyService := service.NewReplyService(commentRepo, userRepo)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("training_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(serviceMetrics)),
	)

	// Register handlers
	handler.RegisterVideoHandler(grpcServer, videoService)
	handler.RegisterCategoryHandler(grpcServer, categoryService, videoService)
	handler.RegisterCommentHandler(grpcServer, commentService)
	handler.RegisterReplyHandler(grpcServer, replyService)

	// Start gRPC server
	port := getEnv("GRPC_PORT", "50057")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Training service listening on port %s", port)

	// Graceful shutdown
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
