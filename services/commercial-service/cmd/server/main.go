package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"metargb/commercial-service/internal/handler"
	"metargb/commercial-service/internal/parsian"
	"metargb/commercial-service/internal/repository"
	"metargb/commercial-service/internal/service"
	"metargb/shared/pkg/auth"
	"metargb/shared/pkg/db"
)

func main() {
	// Load environment variables from config.env
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/commercial-service/config.env",
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

	// Database connection with retry (MySQL may still be starting when the container launches)
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		log.Fatalf("Invalid DB_PORT: %v", err)
	}
	conn, err := db.NewConnection(db.Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            dbPort,
		User:            getEnv("DB_USER", "root"),
		Password:        getEnv("DB_PASSWORD", ""),
		Database:        getEnv("DB_DATABASE", "metargb_db"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close()
	db := conn.DB
	log.Println("Successfully connected to database")

	// Initialize repositories
	walletRepo := repository.NewWalletRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	firstOrderRepo := repository.NewFirstOrderRepository(db)
	variableRepo := repository.NewVariableRepository(db)
	userVariableRepo := repository.NewUserVariableRepository(db)
	referralOrderRepo := repository.NewReferralRepository(db)

	// Initialize Parsian client
	parsianClient := parsian.NewClient()

	// Initialize helper services
	jalaliConverter := service.NewJalaliConverter()

	// Initialize order policy
	orderPolicy := service.NewOrderPolicy(firstOrderRepo)

	// Initialize referral service
	referralService := service.NewReferralService(
		referralOrderRepo,
		variableRepo,
		userVariableRepo,
		walletRepo,
	)

	// Payment configuration
	paymentConfig := &service.PaymentConfig{
		ParsianMerchantID:            getEnv("PARSIAN_PIN", ""),
		ParsianLoanAccountMerchantID: getEnv("PARSIAN_LOAN_ACCOUNT_PIN", ""),
		ParsianCallbackURL:           getEnv("PARSIAN_CALLBACK_URL", getEnv("PAYMENT_CALLBACK_URL", "http://localhost:8000/api/v2/payment/callback")),
	}

	// Initialize services
	walletService := service.NewWalletService(walletRepo)
	transactionService := service.NewTransactionService(transactionRepo, jalaliConverter)
	paymentService := service.NewPaymentService(
		orderRepo,
		transactionRepo,
		paymentRepo,
		walletRepo,
		firstOrderRepo,
		variableRepo,
		parsianClient,
		referralService,
		orderPolicy,
		jalaliConverter,
		paymentConfig,
	)

	// Initialize token validator for authentication
	// Connect to auth service for token validation
	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to auth service - authentication disabled: %v", err)
	} else {
		defer authConn.Close()
		log.Printf("Connected to auth service at %s", authServiceAddr)
	}

	// Create token validator using auth service
	var tokenValidator auth.TokenValidator
	if authConn != nil {
		tokenValidator = auth.NewAuthServiceTokenValidator(authConn)
	}

	// Build gRPC server options with interceptors
	var serverOpts []grpc.ServerOption
	if tokenValidator != nil {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(auth.UnaryServerInterceptor(tokenValidator)))
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(serverOpts...)

	// Register handlers
	handler.RegisterWalletHandler(grpcServer, walletService)
	handler.RegisterTransactionHandler(grpcServer, transactionService)
	handler.RegisterPaymentHandler(grpcServer, paymentService)

	// Start gRPC server
	port := getEnv("GRPC_PORT", "50052")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Commercial service listening on port %s", port)

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
