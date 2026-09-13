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

	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
	"metarang/storage-service/internal/ftp"
	"metarang/storage-service/internal/handler"
	"metarang/storage-service/internal/repository"
	"metarang/storage-service/internal/service"
)

func main() {
	if !loadEnvFiles() {
		log.Printf("Warning: config.env not found, using environment variables only")
	}

	if err := sentry.InitFromEnv("storage-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	dsn := buildDSN()

	db, err := openDatabase(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	log.Println("Successfully connected to database")

	cfg := loadRuntimeConfig()
	ftpClient := ftp.NewFTPClient(
		cfg.FTPHost,
		cfg.FTPPort,
		cfg.FTPUser,
		cfg.FTPPassword,
		cfg.FTPBaseURL,
	)

	// Initialize chunk manager
	chunkManager, err := service.NewChunkManager(cfg.TempDir)
	if err != nil {
		log.Fatalf("Failed to initialize chunk manager: %v", err)
	}
	log.Printf("Chunk manager initialized with temp directory: %s", cfg.TempDir)

	// Initialize repositories
	imageRepo := repository.NewImageRepository(db)

	if err := ensureUploadsDir(cfg.UploadsDir); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	log.Printf("✅ Uploads directory initialized: %s", cfg.UploadsDir)

	// Initialize services
	storageService := service.NewStorageService(ftpClient, chunkManager, cfg.UploadsDir)
	imageService := service.NewImageService(imageRepo, ftpClient)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("storage_service")
	metrics.StartHTTPServer(cfg.MetricsPort)
	serverOpts, err := grpcutil.ServerOptions(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB for file uploads
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)
	if err != nil {
		log.Fatalf("Failed to configure gRPC server: %v", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	// Register gRPC handlers
	handler.RegisterStorageHandler(grpcServer, storageService)
	handler.RegisterImageHandler(grpcServer, imageService)

	// Create HTTP handler for REST API
	httpHandler := handler.NewHTTPHandler(storageService, cfg.UploadsDir)

	// Start gRPC server
	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %s: %v", cfg.GRPCPort, err)
	}

	log.Printf("✅ gRPC server listening on port %s", cfg.GRPCPort)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP server for REST API
	log.Printf("✅ HTTP server listening on port %s", cfg.HTTPPort)
	log.Printf("📤 Chunk upload endpoint: http://localhost:%s/upload", cfg.HTTPPort)
	log.Printf("📁 Static uploads: http://localhost:%s/uploads/", cfg.HTTPPort)

	go func() {
		if err := handler.StartHTTPServer(httpHandler, cfg.HTTPPort); err != nil {
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

func buildDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metarang_db"),
	)
}

type runtimeConfig struct {
	TempDir     string
	UploadsDir  string
	GRPCPort    string
	HTTPPort    string
	MetricsPort string
	FTPHost     string
	FTPPort     string
	FTPUser     string
	FTPPassword string
	FTPBaseURL  string
}

func loadRuntimeConfig() runtimeConfig {
	return runtimeConfig{
		TempDir:     getEnv("TEMP_DIR", "/tmp/storage-chunks"),
		UploadsDir:  getEnv("UPLOAD_DIR", "uploads"),
		GRPCPort:    getEnv("GRPC_PORT", "50059"),
		HTTPPort:    getEnv("HTTP_PORT", "8059"),
		MetricsPort: getEnv("METRICS_PORT", "9090"),
		FTPHost:     getEnv("FTP_HOST", "localhost"),
		FTPPort:     getEnv("FTP_PORT", "21"),
		FTPUser:     getEnv("FTP_USER", ""),
		FTPPassword: getEnv("FTP_PASSWORD", ""),
		FTPBaseURL:  getEnv("FTP_BASE_URL", ""),
	}
}

func ensureUploadsDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func loadEnvFiles() bool {
	configPaths := []string{
		"services/storage-service/config.env",
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

func openDatabase(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
