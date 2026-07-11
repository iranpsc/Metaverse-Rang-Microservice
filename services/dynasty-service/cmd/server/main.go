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

	"metargb/dynasty-service/internal/client"
	"metargb/dynasty-service/internal/handler"
	"metargb/dynasty-service/internal/repository"
	"metargb/dynasty-service/internal/service"

	dynastypb "metargb/shared/pb/dynasty"
	"metargb/shared/pkg/metrics"
	"metargb/shared/pkg/sentry"
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

	if err := sentry.InitFromEnv("dynasty-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

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
	variableRepo := repository.NewVariableRepository(db)
	userVariableRepo := repository.NewUserVariableRepository(db)

	// Notification service client (for sending notifications)
	notificationServiceAddr := getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:50058")
	var notificationPort service.NotificationPort
	if notif, err := client.NewNotificationClient(notificationServiceAddr); err != nil {
		log.Printf("Warning: notification service unavailable (%v); dynasty join-request notifications disabled", err)
	} else {
		notificationPort = notif
		defer func() {
			if err := notif.Close(); err != nil {
				log.Printf("notification client close: %v", err)
			}
		}()
	}

	var walletPort service.WalletPort
	commercialAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "localhost:50052")
	if comm, err := client.NewCommercialClient(commercialAddr); err != nil {
		log.Printf("Warning: commercial service unavailable (%v); dynasty prize claims will fail until it is reachable", err)
	} else {
		walletPort = comm
		defer func() {
			if err := comm.Close(); err != nil {
				log.Printf("commercial client close: %v", err)
			}
		}()
	}

	// Initialize services
	dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, notificationServiceAddr)
	joinRequestService := service.NewJoinRequestService(joinRequestRepo, dynastyRepo, familyRepo, prizeRepo, notificationPort, notificationServiceAddr)
	familyService := service.NewFamilyService(familyRepo, dynastyRepo)
	prizeService := service.NewPrizeService(db, prizeRepo, variableRepo, userVariableRepo, walletPort)
	permissionService := service.NewPermissionService(permissionRepo, joinRequestRepo, familyRepo, dynastyRepo)
	userSearchService := service.NewUserSearchService(db)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("dynasty_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)

	// Create dedicated handlers for each service
	handler.SetProjectLocale(getEnv("PROJECT_LOCALE", "EN"))
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
