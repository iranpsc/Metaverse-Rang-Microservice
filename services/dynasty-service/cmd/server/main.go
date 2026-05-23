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

	"metargb/dynasty-service/internal/handler"
	"metargb/dynasty-service/internal/repository"
	"metargb/dynasty-service/internal/service"

	dynastypb "metargb/shared/pb/dynasty"
	"metargb/shared/pkg/metrics"
)

func main() {
	// Load environment variables from config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/dynasty-service/config.env",
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

	// Database connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metargb_db"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

	// Initialize repositories
	dynastyRepo := repository.NewDynastyRepository(db)
	joinRequestRepo := repository.NewJoinRequestRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	permissionRepo := repository.NewPermissionRepository(db)

	// Notification service client (for sending notifications)
	notificationServiceAddr := getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:50058")

	// Initialize services
	dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, notificationServiceAddr)
	joinRequestService := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, notificationServiceAddr)
	familyService := service.NewFamilyService(familyRepo, dynastyRepo)
	prizeService := service.NewPrizeService(prizeRepo)
	permissionService := service.NewPermissionService(permissionRepo, joinRequestRepo, familyRepo, dynastyRepo)
	userSearchService := service.NewUserSearchService(db)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("dynasty_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(serviceMetrics)),
	)

	// Create dedicated handlers for each service
	dynastyHandler := handler.NewDynastyHandler(dynastyService)
	joinRequestHandler := handler.NewJoinRequestHandler(joinRequestService, permissionService, userSearchService)
	familyHandler := handler.NewFamilyHandler(familyService, permissionService)
	prizeHandler := handler.NewPrizeHandler(prizeService)

	// Register all services with their dedicated handlers
	dynastypb.RegisterDynastyServiceServer(grpcServer, dynastyHandler)
	dynastypb.RegisterJoinRequestServiceServer(grpcServer, joinRequestHandler)
	dynastypb.RegisterFamilyServiceServer(grpcServer, familyHandler)
	dynastypb.RegisterDynastyPrizeServiceServer(grpcServer, prizeHandler)

	// Start gRPC server
	port := getEnv("GRPC_PORT", "50055")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Dynasty service listening on port %s", port)

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
