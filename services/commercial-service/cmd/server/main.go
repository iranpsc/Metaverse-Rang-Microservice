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

	"metargb/commercial-service/internal/handler"
	"metargb/commercial-service/internal/repository"
	"metargb/commercial-service/internal/service"
	"metargb/shared/pkg/auth"
	"metargb/shared/pkg/db"
	"metargb/shared/pkg/metrics"
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

	walletRepo := repository.NewWalletRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	referralRepo := repository.NewReferralRepository(db)
	variableRepo := repository.NewVariableRepository(db)
	userVariableRepo := repository.NewUserVariableRepository(db)
	jalaliConverter := service.NewJalaliConverter()

	walletService := service.NewWalletService(walletRepo)
	transactionService := service.NewTransactionService(transactionRepo, jalaliConverter)
	referralService := service.NewReferralService(referralRepo, variableRepo, userVariableRepo, walletRepo)

	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, tokenValidator, err := auth.DialAuthService(authServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to auth service: %v", err)
	}
	defer authConn.Close()
	log.Printf("Connected to auth service at %s", authServiceAddr)

	serviceMetrics := metrics.NewMetrics("commercial_service")
	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(auth.ServerInterceptors(metrics.UnaryServerInterceptor(serviceMetrics), tokenValidator)...),
	)

	handler.RegisterWalletHandler(grpcServer, walletService)
	handler.RegisterTransactionHandler(grpcServer, transactionService)
	handler.RegisterReferralHandler(grpcServer, referralService)

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
