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

	"metargb/shared/pkg/metrics"
	"metargb/shared/pkg/sentry"
	"metargb/storage-service/internal/ftp"
	"metargb/storage-service/internal/handler"
	"metargb/storage-service/internal/repository"
	"metargb/storage-service/internal/service"
)

func main() {
	// Load environment variables from config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/storage-service/config.env",
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

	if err := sentry.InitFromEnv("storage-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

	// Initialize FTP client
	ftpClient := ftp.NewFTPClient(
		getEnv("FTP_HOST", "localhost"),
		getEnv("FTP_PORT", "21"),
		getEnv("FTP_USER", ""),
		getEnv("FTP_PASSWORD", ""),
		getEnv("FTP_BASE_URL", ""),
	)

	// Initialize chunk manager
	tempDir := getEnv("TEMP_DIR", "/tmp/storage-chunks")
	chunkManager, err := service.NewChunkManager(tempDir)
	if err != nil {
		log.Fatalf("Failed to initialize chunk manager: %v", err)
	}
	log.Printf("Chunk manager initialized with temp directory: %s", tempDir)

	// Initialize repositories
	imageRepo := repository.NewImageRepository(db)

	uploadsDir := getEnv("UPLOAD_DIR", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	log.Printf("✅ Uploads directory initialized: %s", uploadsDir)

	// Initialize services
	storageService := service.NewStorageService(ftpClient, chunkManager, uploadsDir)
	imageService := service.NewImageService(imageRepo, ftpClient)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("storage_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB for file uploads
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)

	// Register gRPC handlers
	handler.RegisterStorageHandler(grpcServer, storageService)
	handler.RegisterImageHandler(grpcServer, imageService)

	// Create HTTP handler for REST API
	httpHandler := handler.NewHTTPHandler(storageService, uploadsDir)

	// Start gRPC server
	grpcPort := getEnv("GRPC_PORT", "50059")
	listener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %s: %v", grpcPort, err)
	}

	log.Printf("✅ gRPC server listening on port %s", grpcPort)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP server for REST API
	httpPort := getEnv("HTTP_PORT", "8059")
	log.Printf("✅ HTTP server listening on port %s", httpPort)
	log.Printf("📤 Chunk upload endpoint: http://localhost:%s/upload", httpPort)
	log.Printf("📁 Static uploads: http://localhost:%s/uploads/", httpPort)

	go func() {
		if err := handler.StartHTTPServer(httpHandler, httpPort); err != nil {
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
