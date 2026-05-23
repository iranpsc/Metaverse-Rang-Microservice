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
	"google.golang.org/grpc/credentials/insecure"

	"metargb/financial-service/internal/handler"
	"metargb/financial-service/internal/parsian"
	"metargb/financial-service/internal/repository"
	"metargb/financial-service/internal/service"
	commercialpb "metargb/shared/pb/commercial"
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
		"services/financial-service/config.env",
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
	if err := db.PingContext(ctx); err != nil {
		cancel()
		log.Fatalf("Failed to ping database: %v", err)
	}
	cancel()
	log.Println("Successfully connected to database")

	// Initialize repositories
	orderRepo := repository.NewOrderRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	variableRepo := repository.NewVariableRepository(db)
	firstOrderRepo := repository.NewFirstOrderRepository(db)
	optionRepo := repository.NewOptionRepository(db)
	imageRepo := repository.NewImageRepository(db)

	// Initialize Parsian client
	parsianClient := parsian.NewClient()

	// Initialize order policy
	orderPolicy := service.NewOrderPolicy(db, firstOrderRepo)

	// Initialize Jalali converter
	jalaliConverter := service.NewJalaliConverter()

	// Connect to commercial-service for wallet balance updates
	var walletClient commercialpb.WalletServiceClient
	commercialAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	commercialConn, err := grpc.Dial(commercialAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to commercial service at %s - wallet updates disabled: %v", commercialAddr, err)
	} else {
		defer commercialConn.Close()
		walletClient = commercialpb.NewWalletServiceClient(commercialConn)
		log.Printf("Connected to commercial service at %s", commercialAddr)
	}

	// Initialize order service
	orderService := service.NewOrderService(
		orderRepo,
		transactionRepo,
		paymentRepo,
		variableRepo,
		firstOrderRepo,
		parsianClient,
		orderPolicy,
		jalaliConverter,
		walletClient,
		service.OrderConfig{
			ParsianMerchantID:            getEnv("PARSIAN_MERCHANT_ID", ""),
			ParsianLoanAccountMerchantID: getEnv("PARSIAN_LOAN_ACCOUNT_MERCHANT_ID", ""),
			ParsianCallbackURL:           getEnv("PARSIAN_CALLBACK_URL", ""),
			FrontendURL:                  getEnv("FRONTEND_URL", ""),
		},
	)

	// Initialize store service
	storeService := service.NewStoreService(
		optionRepo,
		variableRepo,
		imageRepo,
	)

	// Create gRPC server
	serviceMetrics := metrics.NewMetrics("financial_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(serviceMetrics)),
	)

	// Register handlers
	handler.RegisterOrderHandler(grpcServer, orderService)
	handler.RegisterStoreHandler(grpcServer, storeService)

	// Start gRPC server
	port := getEnv("GRPC_PORT", "50058")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Financial service listening on port %s", port)

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
