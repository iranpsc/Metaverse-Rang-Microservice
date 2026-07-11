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
	"metargb/support-service/internal/handler"
	"metargb/support-service/internal/repository"
	"metargb/support-service/internal/service"
)

func main() {
	// Load environment variables from config.env
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
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)

	handler.RegisterTicketHandler(grpcServer, ticketService)
	handler.RegisterReportHandler(grpcServer, reportService)
	handler.RegisterUserEventHandler(grpcServer, userEventService)
	handler.RegisterNoteHandler(grpcServer, noteService)

	port := getEnv("GRPC_PORT", "50056")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Support service listening on port %s", port)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
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
