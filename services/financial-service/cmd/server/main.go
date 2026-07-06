package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"metargb/financial-service/internal/handler"
	"metargb/financial-service/internal/sadad"
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

	// Load config.env; file values apply only when not already set (docker-compose environment overrides DB_* etc.).
	configPaths := []string{
		"services/financial-service/config.env",
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
	}
	var loadedConfigPath string
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			loadedConfigPath = configPath
			break
		}
	}
	if loadedConfigPath != "" {
		log.Printf("Loaded configuration from %s", loadedConfigPath)
	} else {
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

	// Commercial-service client for wallet balance updates after payment
	var walletClient commercialpb.WalletServiceClient
	commercialAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	commercialConn, err := grpc.NewClient(commercialAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: failed to dial commercial service at %s — wallet updates disabled: %v", commercialAddr, err)
	} else {
		defer commercialConn.Close()
		log.Printf("Connected to commercial service at %s", commercialAddr)
		walletClient = commercialpb.NewWalletServiceClient(commercialConn)
	}

	// Initialize repositories
	orderRepo := repository.NewOrderRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	variableRepo := repository.NewVariableRepository(db)
	firstOrderRepo := repository.NewFirstOrderRepository(db)
	optionRepo := repository.NewOptionRepository(db)
	imageRepo := repository.NewImageRepository(db)

	// Initialize Sadad client (BankTest sandbox when SADAD_SANDBOX=true)
	sadadSandbox := parseBoolEnv("SADAD_SANDBOX", false)
	sadadClient := sadad.NewClientWithSandbox(sadadSandbox)
	if sadadSandbox {
		log.Println("Sadad payment gateway: sandbox mode (BankTest)")
	} else {
		log.Println("Sadad payment gateway: production mode")
	}

	sadadCallbackURL := resolveSadadCallbackURL()
	log.Printf("Sadad callback URL: %s", sadadCallbackURL)
	frontendURL := resolveFrontendURL()
	log.Printf("Frontend URL: %s", frontendURL)

	// Initialize order policy
	orderPolicy := service.NewOrderPolicy(db, firstOrderRepo)

	// Initialize Jalali converter
	jalaliConverter := service.NewJalaliConverter()

	// Initialize order service
	orderService := service.NewOrderService(
		orderRepo,
		transactionRepo,
		paymentRepo,
		variableRepo,
		firstOrderRepo,
		sadadClient,
		orderPolicy,
		jalaliConverter,
		walletClient,
		service.OrderConfig{
			SadadMerchantID:             getEnv("SADAD_MERCHANT_ID", ""),
			SadadTerminalID:             getEnv("SADAD_TERMINAL_ID", ""),
			SadadTransactionKey:         getEnv("SADAD_TRANSACTION_KEY", ""),
			SadadPaymentIdentityRial:    getEnv("SADAD_PAYMENT_IDENTITY_RIAL", ""),
			SadadPaymentIdentityNonRial: getEnv("SADAD_PAYMENT_IDENTITY_NON_RIAL", ""),
			SadadCallbackURL:            sadadCallbackURL,
			FrontendURL:                 frontendURL,
			SadadSandbox:                sadadSandbox,
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

const paymentCallbackPath = "/api/payment/callback"

// resolveSadadCallbackURL returns the Sadad ReturnUrl base (without order_id query param).
// The gateway must redirect users to the API callback endpoint, never the frontend verify page.
// Supports ${PROJECT_URL} expansion in config.env (e.g. SADAD_CALLBACK_URL=${PROJECT_URL}/api/payment/callback).
func resolveSadadCallbackURL() string {
	for _, key := range []string{"SADAD_CALLBACK_URL", "PAYMENT_CALLBACK_URL"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			if expanded := strings.TrimSpace(os.ExpandEnv(raw)); expanded != "" {
				if normalized, ok := normalizePaymentCallbackURL(expanded); ok {
					return normalized
				}
				log.Printf("Warning: %s=%q is not a valid API callback URL; falling back to PROJECT_URL", key, expanded)
			}
		}
	}

	projectURL := strings.TrimSpace(os.ExpandEnv(getEnv("PROJECT_URL", "http://localhost:8000")))
	return strings.TrimSuffix(projectURL, "/") + paymentCallbackPath
}

func normalizePaymentCallbackURL(raw string) (string, bool) {
	if strings.Contains(raw, "/payment/verify") {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	path := strings.TrimSuffix(parsed.Path, "/")
	if path == paymentCallbackPath {
		return strings.TrimSuffix(raw, "/"), true
	}

	if path == "" || path == "/" {
		parsed.Path = paymentCallbackPath
		return strings.TrimSuffix(parsed.String(), "/"), true
	}

	return "", false
}

func resolveFrontendURL() string {
	raw := strings.TrimSpace(os.Getenv("FRONTEND_URL"))
	if raw == "" {
		return ""
	}
	expanded := strings.TrimSpace(os.ExpandEnv(raw))
	return strings.TrimSuffix(normalizeURLScheme(expanded), "/")
}

func normalizeURLScheme(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	if strings.Contains(rawURL, "://") {
		return rawURL
	}
	return "http://" + rawURL
}

func parseBoolEnv(key string, defaultValue bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	switch strings.ToLower(raw) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		log.Printf("Warning: invalid boolean for %s=%q, using default %t", key, raw, defaultValue)
		return defaultValue
	}
}
