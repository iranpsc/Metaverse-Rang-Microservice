package main

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"metarang/buildings-service/internal/client"
	"metarang/buildings-service/internal/handler"
	"metarang/buildings-service/internal/middleware"
	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/internal/service"
	"metarang/buildings-service/pkg/threed_client"
	authpb "metarang/shared/pb/auth"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"
	"metarang/shared/pkg/db"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/logger"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configPaths := []string{
		"services/buildings-service/config.env",
		"config.env",
		"./config.env",
		"../../config.env",
	}
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			break
		}
	}

	log := logger.NewLogger("buildings-service")
	log.Info("Starting Buildings Service...")

	if err := sentry.InitFromEnv("buildings-service"); err != nil {
		log.Warn("Failed to initialize Sentry", "error", err)
	}
	defer sentry.Flush(2 * time.Second)

	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local",
		getEnv("DB_USER", "metarang_user"),
		getEnv("DB_PASSWORD", "metarang_password"),
		getEnv("DB_HOST", "mysql"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metarang_db"),
	)
	port := getEnv("GRPC_PORT", "50063")
	httpPort := getEnv("HTTP_PORT", "8072")
	metricsPort := getEnv("METRICS_PORT", "9090")
	threeDMetaURL := getEnv("THREE_D_META_URL", "http://3d-meta-api")

	database, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	defer func() { _ = database.Close() }()

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(5 * time.Minute)

	if err := database.Ping(); err != nil {
		log.Fatal("Failed to ping database", "error", err)
	}

	schemaGuard := db.NewSchemaGuard(database)
	if err := schemaGuard.ValidateTable(db.TableSchema{
		Name: "buildings",
		Columns: []db.ColumnType{
			{Name: "id", DataType: "bigint"},
			{Name: "feature_id", DataType: "bigint"},
		},
	}); err != nil {
		log.Warn("Schema validation warning", "error", err)
	}

	log.Info("Database connected and schema validated")

	buildingRepo := repository.NewBuildingRepository(database)
	featureRepo := repository.NewFeatureRepository(database)
	geometryRepo := repository.NewGeometryRepository(database)
	hourlyProfitRepo := repository.NewHourlyProfitRepository(database)
	userRepo := repository.NewUserRepository(database)

	threeDClient := threed_client.New(threeDMetaURL)

	commercialServiceAddr := getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052")
	commercialClient, err := client.NewCommercialClient(commercialServiceAddr)
	if err != nil {
		log.Warn("Failed to connect to commercial service - wallet operations disabled", "error", err)
		commercialClient = nil
	} else {
		log.Info("Connected to commercial service", "addr", commercialServiceAddr)
		defer func() { _ = commercialClient.Close() }()
		if timeout, ok := parsePositiveDuration(getEnv("COMMERCIAL_SERVICE_TIMEOUT", "3s")); ok {
			commercialClient.SetTimeout(timeout)
		}
		if retries, ok := parsePositiveInt(getEnv("COMMERCIAL_SERVICE_RETRIES", "3")); ok {
			commercialClient.SetMaxRetries(retries)
		}
	}

	buildingService := service.NewBuildingService(
		buildingRepo,
		featureRepo,
		geometryRepo,
		hourlyProfitRepo,
		threeDClient,
	)
	if commercialClient != nil {
		buildingService.SetCommercialClient(commercialClient)
	}

	completedBuildingService := service.NewCompletedBuildingService(buildingRepo)
	citizenBuildingsService := service.NewCitizenBuildingsService(buildingRepo, userRepo, nil)

	handler.SetProjectLocale(getEnv("PROJECT_LOCALE", "EN"))
	buildingHandler := handler.NewBuildingHandler(buildingService, completedBuildingService)
	citizenBuildingsHandler := handler.NewCitizenBuildingsHandler(citizenBuildingsService)

	authServiceAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpcutil.NewClient(authServiceAddr)
	if err != nil {
		log.Warn("Failed to connect to auth service - authentication disabled", "error", err)
	} else {
		defer func() { _ = authConn.Close() }()
		log.Info("Connected to auth service", "addr", authServiceAddr)
	}

	var tokenValidator auth.TokenValidator
	var authClient authpb.AuthServiceClient
	var citizenClient authpb.CitizenServiceClient
	if authConn != nil {
		tokenValidator = auth.NewAuthServiceTokenValidator(authConn)
		authClient = authpb.NewAuthServiceClient(authConn)
		citizenClient = authpb.NewCitizenServiceClient(authConn)
	}

	serviceMetrics := metrics.NewMetrics("buildings_service")
	metrics.StartHTTPServer(metricsPort)

	interceptors := []grpc.UnaryServerInterceptor{
		sentry.UnaryServerInterceptor(),
		logger.UnaryServerInterceptor(log),
		metrics.UnaryServerInterceptor(serviceMetrics),
	}
	if tokenValidator != nil {
		interceptors = append(interceptors, auth.UnaryServerInterceptor(tokenValidator))
	}

	serverOpts, err := grpcutil.ServerOptions(
		grpc.ChainUnaryInterceptor(interceptors...),
	)
	if err != nil {
		log.Fatal("Failed to configure gRPC server", "error", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	pb.RegisterBuildingServiceServer(grpcServer, buildingHandler)
	pb.RegisterCitizenBuildingsServiceServer(grpcServer, citizenBuildingsHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("Failed to listen", "error", err, "port", port)
	}

	httpHandlers := handler.HTTPServerHandlers{
		Buildings:        handler.NewHTTPBuildingsHandler(buildingHandler),
		CitizenBuildings: handler.NewHTTPCitizenBuildingsHandler(citizenBuildingsHandler, citizenClient),
	}
	authMiddleware := middleware.AuthMiddleware(authClient)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(authClient)
	accountSecurityMiddleware := middleware.AccountSecurityMiddleware(authClient)

	log.Info("Buildings Service started", "grpc_port", port, "http_port", httpPort, "metrics_port", metricsPort)

	go func() {
		log.Info("HTTP server listening", "port", httpPort)
		if err := handler.StartHTTPServer(httpHandlers, httpPort, authMiddleware, optionalAuthMiddleware, accountSecurityMiddleware); err != nil {
			log.Fatal("Failed to serve HTTP", "error", err)
		}
	}()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to serve gRPC", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down gracefully...")
	grpcServer.GracefulStop()
	_ = database.Close()
	log.Info("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parsePositiveDuration(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d, true
}

func parsePositiveInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
