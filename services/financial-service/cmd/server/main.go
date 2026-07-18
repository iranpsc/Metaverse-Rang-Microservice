package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"metarang/financial-service/internal/config"
	"metarang/financial-service/internal/handler"
	"metarang/financial-service/internal/repository"
	"metarang/financial-service/internal/sadad"
	"metarang/financial-service/internal/service"
	commercialpb "metarang/shared/pb/commercial"
	notificationspb "metarang/shared/pb/notifications"
	"metarang/shared/pkg/auth"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
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

	if err := sentry.InitFromEnv("financial-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	// Database connection
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
	commercialConn, err := grpcutil.NewClient(commercialAddr)
	if err != nil {
		log.Printf("Warning: failed to dial commercial service at %s — wallet updates disabled: %v", commercialAddr, err)
	} else {
		defer func() { _ = commercialConn.Close() }()
		log.Printf("Connected to commercial service at %s", commercialAddr)
		walletClient = commercialpb.NewWalletServiceClient(commercialConn)
	}

	// Notifications-service client for post-payment SMS (optional)
	var smsClient notificationspb.SMSServiceClient
	notificationsAddr := getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications-service:50058")
	notificationsConn, err := grpcutil.NewClient(notificationsAddr)
	if err != nil {
		log.Printf("Warning: failed to dial notifications service at %s — payment SMS disabled: %v", notificationsAddr, err)
	} else {
		defer func() { _ = notificationsConn.Close() }()
		log.Printf("Connected to notifications service at %s", notificationsAddr)
		smsClient = notificationspb.NewSMSServiceClient(notificationsConn)
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

	sadadCallbackURL := config.ResolveSadadCallbackURL()
	log.Printf("Sadad callback URL: %s", sadadCallbackURL)
	frontendURL := config.ResolveFrontendURL()
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
		smsClient,
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

	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpcutil.NewClient(authServiceAddr)
	if err != nil {
		log.Printf("Warning: failed to dial auth service at %s — user auth disabled: %v", authServiceAddr, err)
	} else {
		defer func() { _ = authConn.Close() }()
		log.Printf("Connected to auth service at %s", authServiceAddr)
	}

	var tokenValidator auth.TokenValidator
	if authConn != nil {
		tokenValidator = auth.NewAuthServiceTokenValidator(authConn)
	}

	interceptors := []grpc.UnaryServerInterceptor{
		sentry.UnaryServerInterceptor(),
		metrics.UnaryServerInterceptor(serviceMetrics),
	}
	if tokenValidator != nil {
		interceptors = append(interceptors, auth.UnaryServerInterceptor(tokenValidator))
	}

	serverOpts, err := grpcutil.ServerOptions(grpc.ChainUnaryInterceptor(interceptors...))
	if err != nil {
		log.Fatalf("Failed to configure gRPC server: %v", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)

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
