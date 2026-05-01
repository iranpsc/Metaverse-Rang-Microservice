package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"metargb/features-service/internal/client"
	"metargb/features-service/internal/events"
	"metargb/features-service/internal/handler"
	"metargb/features-service/internal/metrics"
	"metargb/features-service/internal/repository"
	"metargb/features-service/internal/service"
	"metargb/features-service/pkg/threed_client"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/auth"
	"metargb/shared/pkg/db"
	"metargb/shared/pkg/logger"
	sharedmetrics "metargb/shared/pkg/metrics"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Initialize logger
	log := logger.NewLogger("features-service")
	log.Info("Starting Features Service...")

	// Load configuration from environment
	// Construct DSN from individual environment variables
	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		getEnv("DB_USER", "metargb_user"),
		getEnv("DB_PASSWORD", "metargb_password"),
		getEnv("DB_HOST", "mysql"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metargb_db"),
	)
	port := getEnv("GRPC_PORT", "50053")
	metricsPort := getEnv("METRICS_PORT", "9090")
	threeDMetaURL := getEnv("THREE_D_META_URL", "http://3d-meta-api")

	// Initialize database connection
	database, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	defer database.Close()

	// Test database connection
	if err := database.Ping(); err != nil {
		log.Fatal("Failed to ping database", "error", err)
	}

	// Validate schema
	schemaGuard := db.NewSchemaGuard(database)
	if err := schemaGuard.ValidateTable(db.TableSchema{
		Name: "features",
		Columns: []db.ColumnType{
			{Name: "id", DataType: "bigint"},
			{Name: "map_id", DataType: "bigint"},
			{Name: "owner_id", DataType: "bigint"},
			{Name: "type", DataType: "varchar"},
		},
	}); err != nil {
		log.Warn("Schema validation warning", "error", err)
	}

	log.Info("Database connected and schema validated")

	// Initialize repositories
	featureRepo := repository.NewFeatureRepository(database)
	propertiesRepo := repository.NewPropertiesRepository(database)
	geometryRepo := repository.NewGeometryRepository(database)
	tradeRepo := repository.NewTradeRepository(database)
	buyRequestRepo := repository.NewBuyRequestRepository(database)
	sellRequestRepo := repository.NewSellRequestRepository(database)
	hourlyProfitRepo := repository.NewHourlyProfitRepository(database)
	buildingRepo := repository.NewBuildingRepository(database)
	imageRepo := repository.NewImageRepository(database)
	lockedAssetRepo := repository.NewLockedAssetRepository(database)
	featureLimitRepo := repository.NewFeatureLimitRepository(database)
	mapRepo := repository.NewMapRepository(database)
	variableRepo := repository.NewVariableRepository(database)

	// Initialize 3D client
	threeDClient := threed_client.New(threeDMetaURL)

	// Initialize commercial client for wallet operations
	commercialServiceAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	commercialClient, err := client.NewCommercialClient(commercialServiceAddr)
	if err != nil {
		log.Warn("Failed to connect to commercial service - marketplace features disabled", "error", err)
		commercialClient = nil
	} else {
		log.Info("Connected to commercial service", "addr", commercialServiceAddr)
		defer commercialClient.Close()

		// Configure timeout and retries from environment
		if timeoutStr := getEnv("COMMERCIAL_SERVICE_TIMEOUT", "3s"); timeoutStr != "" {
			if timeout, err := time.ParseDuration(timeoutStr); err == nil {
				commercialClient.SetTimeout(timeout)
			}
		}
		if retriesStr := getEnv("COMMERCIAL_SERVICE_RETRIES", "3"); retriesStr != "" {
			if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
				commercialClient.SetMaxRetries(retries)
			}
		}
	}

	// Initialize notification client for profit notifications
	notificationServiceAddr := getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications-service:50058")
	notificationClient, err := client.NewNotificationClient(notificationServiceAddr)
	if err != nil {
		log.Warn("Failed to connect to notification service - notifications disabled", "error", err)
		notificationClient = nil
	} else {
		log.Info("Connected to notification service", "addr", notificationServiceAddr)
		defer notificationClient.Close()
	}

	// Initialize Redis event broadcaster
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	broadcastChannel := getEnv("BROADCAST_CHANNEL", "feature-events")
	eventBroadcaster, err := events.NewRedisBroadcaster(redisAddr, redisPassword, broadcastChannel)
	if err != nil {
		log.Warn("Failed to connect to Redis - event broadcasting disabled", "error", err)
		eventBroadcaster = nil
	} else {
		log.Info("Connected to Redis for event broadcasting", "addr", redisAddr, "channel", broadcastChannel)
		defer eventBroadcaster.Close()
	}

	// Initialize marketplace metrics
	marketplaceMetrics := metrics.NewMarketplaceMetrics()

	// Initialize pricing service
	pricingService := service.NewFeaturePricingService(
		featureRepo,
		propertiesRepo,
		database,
		log,
	)

	// Initialize services
	featureService := service.NewFeatureService(
		featureRepo,
		propertiesRepo,
		geometryRepo,
		imageRepo,
		buildingRepo,
		tradeRepo,
		hourlyProfitRepo,
		pricingService,
		database,
	)

	// Initialize marketplace service with all dependencies
	marketplaceService := service.NewMarketplaceService(
		featureRepo,
		propertiesRepo,
		geometryRepo,
		tradeRepo,
		buyRequestRepo,
		sellRequestRepo,
		lockedAssetRepo,
		hourlyProfitRepo,
		featureLimitRepo,
		variableRepo,
		commercialClient,
		notificationClient,
		eventBroadcaster,
		marketplaceMetrics,
		database,
		log,
	)

	profitService := service.NewProfitService(
		hourlyProfitRepo,
		featureRepo,
		propertiesRepo,
		commercialClient,
		notificationClient,
		database,
		log,
	)

	buildingService := service.NewBuildingService(
		buildingRepo,
		featureRepo,
		geometryRepo,
		hourlyProfitRepo,
		threeDClient,
	)

	// Set commercial client for building service (for wallet operations)
	if commercialClient != nil {
		buildingService.SetCommercialClient(commercialClient)
	}

	mapService := service.NewMapService(
		mapRepo,
		featureRepo,
	)

	// Initialize gRPC handlers
	handler.SetProjectLocale(getEnv("PROJECT_LOCALE", "EN"))
	featureHandler := handler.NewFeatureHandler(featureService)
	marketplaceHandler := handler.NewMarketplaceHandler(marketplaceService, geometryRepo, featureRepo)
	profitHandler := handler.NewProfitHandler(profitService)
	buildingHandler := handler.NewBuildingHandler(buildingService)
	mapHandler := handler.NewMapHandler(mapService)

	// Initialize token validator for authentication
	// Connect to auth service for token validation
	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warn("Failed to connect to auth service - authentication disabled", "error", err)
	} else {
		defer authConn.Close()
		log.Info("Connected to auth service", "addr", authServiceAddr)
	}

	// Create token validator using auth service
	var tokenValidator auth.TokenValidator
	if authConn != nil {
		tokenValidator = auth.NewAuthServiceTokenValidator(authConn)
	}

	// Create gRPC server with interceptors
	serviceMetrics := sharedmetrics.NewMetrics("features")

	// Build interceptor chain
	interceptors := []grpc.UnaryServerInterceptor{
		logger.UnaryServerInterceptor(log),
		sharedmetrics.UnaryServerInterceptor(serviceMetrics),
	}

	// Add auth interceptor if token validator is available
	if tokenValidator != nil {
		interceptors = append(interceptors, auth.UnaryServerInterceptor(tokenValidator))
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	// Register services
	pb.RegisterFeatureServiceServer(grpcServer, featureHandler)
	pb.RegisterFeatureMarketplaceServiceServer(grpcServer, marketplaceHandler)
	pb.RegisterFeatureProfitServiceServer(grpcServer, profitHandler)
	pb.RegisterBuildingServiceServer(grpcServer, buildingHandler)
	pb.RegisterMapsServiceServer(grpcServer, mapHandler)

	// Enable reflection for debugging
	reflection.Register(grpcServer)

	// Metrics are exposed via Prometheus client library
	// Start HTTP server for metrics endpoint if needed
	log.Info("Metrics available on /metrics endpoint", "port", metricsPort)

	// Start hourly profit calculator background job
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go profitService.StartHourlyProfitCalculator(ctx, log)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal("Failed to listen", "error", err, "port", port)
	}

	log.Info("Features Service started", "port", port, "metrics_port", metricsPort)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutting down gracefully...")
		cancel() // Stop background jobs
		grpcServer.GracefulStop()
		database.Close()
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
