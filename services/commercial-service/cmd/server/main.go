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

	"metarang/commercial-service/internal/handler"
	"metarang/commercial-service/internal/repository"
	"metarang/commercial-service/internal/service"
	"metarang/shared/pkg/auth"
	"metarang/shared/pkg/db"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
)

func main() {
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

	if err := sentry.InitFromEnv("commercial-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		log.Fatalf("Invalid DB_PORT: %v", err)
	}
	conn, err := db.NewConnection(db.Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            dbPort,
		User:            getEnv("DB_USER", "root"),
		Password:        getEnv("DB_PASSWORD", ""),
		Database:        getEnv("DB_DATABASE", "metarang_db"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = conn.Close() }()
	db := conn.DB
	log.Println("Successfully connected to database")

	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	referralRepo := repository.NewReferralRepository(db)
	variableRepo := repository.NewVariableRepository(db)
	userVariableRepo := repository.NewUserVariableRepository(db)
	walletHistoryRepo := repository.NewWalletHistoryRepository(db)
	jalaliConverter := service.NewJalaliConverter()

	walletService := service.NewWalletService(walletRepo)
	transactionService := service.NewTransactionService(transactionRepo, jalaliConverter)
	referralService := service.NewReferralService(referralRepo, variableRepo, userVariableRepo, walletRepo)
	userVariableService := service.NewUserVariableService(userVariableRepo)
	incomeCalc := service.NewIncomeCalculator(walletHistoryRepo)
	spendingCalc := service.NewSpendingCalculator(walletHistoryRepo)
	walletHistoryService := service.NewWalletHistoryService(walletHistoryRepo, incomeCalc, spendingCalc)

	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpcutil.NewClient(authServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to auth service - authentication disabled: %v", err)
	} else {
		defer func() { _ = authConn.Close() }()
		log.Printf("Connected to auth service at %s", authServiceAddr)
	}

	var tokenValidator auth.TokenValidator
	if authConn != nil {
		tokenValidator = auth.NewAuthServiceTokenValidator(authConn)
	}

	serviceMetrics := metrics.NewMetrics("commercial_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))

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

	handler.RegisterWalletHandler(grpcServer, walletService)
	handler.RegisterTransactionHandler(grpcServer, transactionService)
	handler.RegisterReferralHandler(grpcServer, referralService)
	handler.RegisterUserVariableHandler(grpcServer, userVariableService)
	handler.RegisterWalletHistoryHandler(grpcServer, walletHistoryService)

	port := getEnv("GRPC_PORT", "50052")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Commercial service listening on port %s", port)

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
