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

	"metarang/calendar-service/internal/handler"
	"metarang/calendar-service/internal/middleware"
	"metarang/calendar-service/internal/repository"
	"metarang/calendar-service/internal/service"
	authpb "metarang/shared/pb/auth"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
)

func main() {
	// Load environment variables from config.env
	configPaths := []string{
		"services/calendar-service/config.env",
		"config.env",
		"./config.env",
		"../../config.env",
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

	if err := sentry.InitFromEnv("calendar-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
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
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

	calendarRepo := repository.NewCalendarRepository(db)
	calendarService := service.NewCalendarService(calendarRepo)

	serviceMetrics := metrics.NewMetrics("calendar_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	serverOpts, err := grpcutil.ServerOptions(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)
	if err != nil {
		log.Fatalf("Failed to configure gRPC server: %v", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	calendarHandler := handler.RegisterCalendarHandler(grpcServer, calendarService)

	// Auth client for HTTP middleware (ValidateToken)
	var authClient authpb.AuthServiceClient
	authAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpcutil.NewClient(authAddr)
	if err != nil {
		log.Printf("Warning: failed to connect to auth service at %s: %v", authAddr, err)
	} else {
		defer func() { _ = authConn.Close() }()
		authClient = authpb.NewAuthServiceClient(authConn)
		log.Printf("Created auth service client for %s", authAddr)
	}

	grpcPort := getEnv("GRPC_PORT", "50059")
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %s: %v", grpcPort, err)
	}

	log.Printf("gRPC server listening on port %s", grpcPort)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	httpHandler := handler.NewHTTPCalendarHandler(calendarHandler)
	httpPort := getEnv("HTTP_PORT", "8060")
	authMW := middleware.AuthMiddleware(authClient)
	optionalAuthMW := middleware.OptionalAuthMiddleware(authClient)

	log.Printf("HTTP server listening on port %s", httpPort)
	go func() {
		if err := handler.StartHTTPServer(httpHandler, httpPort, authMW, optionalAuthMW); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

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
