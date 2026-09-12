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

	authpb "metarang/shared/pb/auth"
	storagepb "metarang/shared/pb/storage"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
	"metarang/support-service/internal/handler"
	"metarang/support-service/internal/middleware"
	"metarang/support-service/internal/repository"
	"metarang/support-service/internal/service"
)

func main() {
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/support-service/config.env",
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

	if err := sentry.InitFromEnv("support-service"); err != nil {
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

	ticketRepo := repository.NewTicketRepository(db)
	reportRepo := repository.NewReportRepository(db)
	userEventRepo := repository.NewUserEventRepository(db)
	noteRepo := repository.NewNoteRepository(db)

	notificationServiceAddr := getEnv("NOTIFICATION_SERVICE_ADDR", "notifications-service:50058")

	ticketService := service.NewTicketService(ticketRepo, notificationServiceAddr)
	reportService := service.NewReportService(reportRepo)
	userEventService := service.NewUserEventService(userEventRepo)
	noteService := service.NewNoteService(noteRepo)

	serviceMetrics := metrics.NewMetrics("support_service")
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

	ticketHandler := handler.RegisterTicketHandler(grpcServer, ticketService)
	reportHandler := handler.RegisterReportHandler(grpcServer, reportService)
	handler.RegisterUserEventHandler(grpcServer, userEventService)
	noteHandler := handler.RegisterNoteHandler(grpcServer, noteService)

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

	port := getEnv("GRPC_PORT", "50056")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("gRPC server listening on port %s", port)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	var fileStorage service.FileStorage
	storageAddr := getEnv("STORAGE_SERVICE_ADDR", "storage-service:50060")
	storageConn, err := grpcutil.NewClient(storageAddr)
	if err != nil {
		log.Printf("Warning: failed to connect to storage service at %s: %v", storageAddr, err)
	} else {
		defer func() { _ = storageConn.Close() }()
		fileStorage = service.NewGRPCFileStorage(storagepb.NewFileStorageServiceClient(storageConn))
		log.Printf("Created storage service client for %s", storageAddr)
	}

	httpHandler := handler.NewHTTPSupportHandler(
		ticketHandler,
		reportHandler,
		noteHandler,
		fileStorage,
		getEnv("APP_URL", "http://localhost:8000"),
	)
	httpPort := getEnv("HTTP_PORT", "8070")
	authMW := middleware.AuthMiddleware(authClient)

	log.Printf("HTTP server listening on port %s", httpPort)
	go func() {
		if err := handler.StartHTTPServer(httpHandler, httpPort, authMW); err != nil {
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
