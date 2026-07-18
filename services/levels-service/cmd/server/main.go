package main

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metarang/levels-service/internal/client"
	"metarang/levels-service/internal/handler"
	"metarang/levels-service/internal/repository"
	"metarang/levels-service/internal/service"
	pb "metarang/shared/pb/levels"
	"metarang/shared/pkg/db"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/logger"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"

	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load environment variables from config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/levels-service/config.env",
	}
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			break
		}
	}

	// Initialize logger
	log := logger.NewLogger("levels-service")
	log.Info("Starting Levels Service...")

	if err := sentry.InitFromEnv("levels-service"); err != nil {
		log.Warn("Failed to initialize Sentry", "error", err)
	}
	defer sentry.Flush(2 * time.Second)

	// Load configuration from environment
	// Construct DSN from individual environment variables
	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "metarang_user"),
		getEnv("DB_PASSWORD", "metarang_password"),
		getEnv("DB_HOST", "mysql"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metarang_db"),
	)
	port := getEnv("GRPC_PORT", "50054")
	metricsPort := getEnv("METRICS_PORT", "9090")
	commercialServiceAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")

	// Initialize database connection
	database, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	defer func() { _ = database.Close() }()

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(5 * time.Minute)

	// Test database connection
	if err := database.Ping(); err != nil {
		log.Fatal("Failed to ping database", "error", err)
	}

	// Validate schema
	schemaGuard := db.NewSchemaGuard(database)
	if err := schemaGuard.ValidateTable(db.TableSchema{
		Name: "levels",
		Columns: []db.ColumnType{
			{Name: "id", DataType: "bigint"},
			{Name: "name", DataType: "varchar"},
			{Name: "slug", DataType: "varchar"},
			{Name: "score", DataType: "int"},
		},
	}); err != nil {
		log.Warn("Schema validation warning", "error", err)
	}

	log.Info("Database connected and schema validated")

	// Get admin_panel_url for image URL formatting
	adminPanelURL := getEnv("ADMIN_PANEL_URL", "")

	// Initialize repositories
	levelRepo := repository.NewLevelRepository(database, adminPanelURL)
	activityRepo := repository.NewActivityRepository(database)
	userLogRepo := repository.NewUserLogRepository(database)

	commercialClient, err := client.NewCommercialClient(commercialServiceAddr)
	if err != nil {
		log.Fatal("Failed to connect to commercial service", "error", err, "address", commercialServiceAddr)
	}
	defer func() { _ = commercialClient.Close() }()

	// Initialize services
	levelService := service.NewLevelService(levelRepo, userLogRepo, commercialClient)
	activityService := service.NewActivityService(activityRepo, userLogRepo, levelRepo, commercialClient)

	// Initialize gRPC handlers
	levelHandler := handler.NewLevelHandler(levelService)
	activityHandler := handler.NewActivityHandler(activityService)

	// Create gRPC server with interceptors
	serviceMetrics := metrics.NewMetrics("levels_service")
	metrics.StartHTTPServer(metricsPort)
	serverOpts, err := grpcutil.ServerOptions(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			logger.UnaryServerInterceptor(log),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)
	if err != nil {
		log.Fatal("Failed to configure gRPC server", "error", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	// Register services
	pb.RegisterLevelServiceServer(grpcServer, levelHandler)
	pb.RegisterActivityServiceServer(grpcServer, activityHandler)

	// Enable reflection for debugging
	reflection.Register(grpcServer)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal("Failed to listen", "error", err, "port", port)
	}

	log.Info("Levels Service started", "port", port, "metrics_port", metricsPort)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutting down gracefully...")
		grpcServer.GracefulStop()
		_ = database.Close()
		log.Info("Shutdown complete")
	}()

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve", "error", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
