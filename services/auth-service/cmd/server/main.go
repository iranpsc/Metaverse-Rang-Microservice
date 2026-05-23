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

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"metargb/auth-service/internal/handler"
	"metargb/auth-service/internal/pubsub"
	"metargb/auth-service/internal/repository"
	"metargb/auth-service/internal/service"
	notificationspb "metargb/shared/pb/notifications"
	pb "metargb/shared/pb/auth"
	storagepb "metargb/shared/pb/storage"
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
		"services/auth-service/config.env",
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

	// Database connection with proper UTF-8 encoding for Persian/Farsi text
	// Using utf8mb4 charset for proper Persian/Farsi support
	// interpolateParams=true helps with proper handling of multi-byte characters in parameterized queries
	// Note: collation is not a valid DSN parameter - it's automatically set based on charset
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local&tls=false&interpolateParams=true",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_DATABASE", "metargb_db"),
	)

	// Parse DSN to get config
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		log.Fatalf("Failed to parse DSN: %v", err)
	}

	// Ensure charset is explicitly set to utf8mb4 in connection parameters
	// The collation will be automatically set to utf8mb4_unicode_ci by MySQL based on the charset
	// Note: parseTime and interpolateParams are DSN-level settings, not connection parameters
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["charset"] = "utf8mb4"
	// interpolateParams is already in DSN, so it's handled automatically

	// Create connector with proper charset configuration
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	// Open database using connector
	db := sql.OpenDB(connector)
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Explicitly set charset to UTF-8 for proper Persian/Farsi text handling
	// SET NAMES sets character_set_client, character_set_connection, and character_set_results
	// This ensures all queries return UTF-8 encoded strings
	// Note: This is executed on the test connection; the connector config ensures all new connections use utf8mb4
	if _, err := db.ExecContext(ctx, "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		log.Printf("Warning: Failed to set charset to utf8mb4: %v", err)
	} else {
		log.Println("Successfully set database charset to utf8mb4 for UTF-8/Persian text support")
	}

	log.Println("Successfully connected to database")

	// Initialize Redis connection for caching and pub/sub
	redisURL := getEnv("REDIS_URL", "")
	if redisURL == "" {
		// Construct REDIS_URL from individual components if not set
		redisHost := getEnv("REDIS_HOST", "localhost")
		redisPort := getEnv("REDIS_PORT", "6379")
		redisPassword := getEnv("REDIS_PASSWORD", "")
		redisDB := getEnv("REDIS_DB", "0")
		if redisPassword != "" {
			redisURL = fmt.Sprintf("redis://:%s@%s:%s/%s", redisPassword, redisHost, redisPort, redisDB)
		} else {
			redisURL = fmt.Sprintf("redis://%s:%s/%s", redisHost, redisPort, redisDB)
		}
	}

	// Parse Redis URL for cache client
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		cancel()
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisOpts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}
	redisClient := redis.NewClient(redisOpts)

	// Test Redis connection
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	cancel()
	log.Println("Successfully connected to Redis")

	// Initialize Redis publisher for WebSocket broadcasting
	redisPublisher, err := pubsub.NewRedisPublisher(redisURL)
	if err != nil {
		log.Fatalf("Failed to create Redis publisher: %v", err)
	}

	// Initialize repositories
	adminPanelURL := getEnv("ADMIN_PANEL_URL", "")
	userRepo := repository.NewUserRepository(db, adminPanelURL)
	tokenRepo := repository.NewTokenRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)
	accountSecurityRepo := repository.NewAccountSecurityRepository(db)
	kycRepo := repository.NewKYCRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	citizenRepo := repository.NewCitizenRepository(db)
	personalInfoRepo := repository.NewPersonalInfoRepository(db)
	profilePhotoRepo := repository.NewProfilePhotoRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	searchRepo := repository.NewSearchRepository(db)

	// Initialize observer service for activity tracking and events
	observerService := service.NewObserverServiceWithSettings(
		userRepo,
		settingsRepo,
		activityRepo,
		redisPublisher,
	)

	// Initialize helper service for cross-service integrations
	helperService := service.NewHelperService(
		getEnv("LEVELS_SERVICE_ADDR", "levels-service:50054"),
		getEnv("FEATURES_SERVICE_ADDR", "features-service:50053"),
		getEnv("COMMERCIAL_SERVICE_ADDR", "commercial-service:50052"),
	)

	// Initialize notifications SMS client (optional - service can work without it)
	var smsClient notificationspb.SMSServiceClient
	notificationsAddr := getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications-service:50058")
	notificationsConn, err := grpc.Dial(notificationsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to notifications service: %v (continuing without SMS support)", err)
	} else {
		defer notificationsConn.Close()
		smsClient = notificationspb.NewSMSServiceClient(notificationsConn)
		log.Println("Successfully connected to notifications service")
	}

	// Initialize services
	authService := service.NewAuthService(
		userRepo,
		tokenRepo,
		cacheRepo,
		accountSecurityRepo,
		activityRepo,
		observerService,
		helperService,
		smsClient,
		getEnv("OAUTH_SERVER_URL", ""),
		getEnv("OAUTH_CLIENT_ID", ""),
		getEnv("OAUTH_CLIENT_SECRET", ""),
		getEnv("APP_URL", "http://localhost:8000"),
		getEnv("FRONT_END_URL", "http://localhost:3000"),
		service.IsProductionEnv(getEnv("APP_ENV", "local")),
	)
	// Initialize user service with all dependencies for Users API
	userService := service.NewUserServiceWithDependencies(
		userRepo,
		kycRepo,
		settingsRepo,
		profilePhotoRepo,
	)
	kycService := service.NewKYCService(kycRepo, userRepo)
	citizenService := service.NewCitizenService(
		citizenRepo,
		userRepo,
		helperService,
		getEnv("APP_URL", "http://localhost:8000"),
	)
	personalInfoService := service.NewPersonalInfoService(personalInfoRepo)
	profileLimitationRepo := repository.NewProfileLimitationRepository(db)
	profileLimitationService := service.NewProfileLimitationService(profileLimitationRepo, userRepo)
	settingsService := service.NewSettingsService(settingsRepo)

	// Get API Gateway URL for profile photo URLs - ensure it's not empty
	apiGatewayURL := getEnv("API_GATEWAY_URL", "")
	if apiGatewayURL == "" {
		apiGatewayURL = getEnv("APP_URL", "http://localhost:8000")
	}
	log.Printf("Profile photo service using API Gateway URL: %s", apiGatewayURL)

	// Initialize profile photo service (storage client can be added later when proto files are generated)
	// For now, service works without storage client (files can be uploaded via HTTP endpoint)
	profilePhotoService := service.NewProfilePhotoService(profilePhotoRepo, nil, apiGatewayURL)

	// Initialize storage service client for profile photo uploads
	storageServiceAddr := getEnv("STORAGE_SERVICE_ADDR", "storage-service:50060")
	var storageClient storagepb.FileStorageServiceClient
	storageConn, err := grpc.NewClient(storageServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Failed to connect to storage service: %v (profile photo uploads will fail)", err)
		storageClient = nil
	} else {
		defer storageConn.Close()
		storageClient = storagepb.NewFileStorageServiceClient(storageConn)
		log.Printf("Successfully connected to storage service at %s", storageServiceAddr)
	}

	// Initialize user events service
	userEventsService := service.NewUserEventsService(activityRepo, userRepo)

	// Initialize search service
	searchService := service.NewSearchService(searchRepo)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create profile photo handler instance (needed by auth handler)
	profilePhotoHandler := &handler.ProfilePhotoHandler{
		ProfilePhotoService: profilePhotoService,
		StorageClient:       storageClient,
		ApiGatewayURL:       apiGatewayURL,
	}

	// Register handlers
	projectLocale := getEnv("PROJECT_LOCALE", "EN")
	handler.SetProjectLocale(projectLocale)
	handler.RegisterAuthHandler(grpcServer, authService, tokenRepo, profilePhotoHandler, projectLocale)
	handler.RegisterUserHandler(grpcServer, userService, profileLimitationService, helperService)
	handler.RegisterKYCHandler(grpcServer, kycService, storageClient, apiGatewayURL)
	handler.RegisterCitizenHandler(grpcServer, citizenService)
	handler.RegisterPersonalInfoHandler(grpcServer, personalInfoService)
	handler.RegisterProfileLimitationHandler(grpcServer, profileLimitationService)
	// Register profile photo handler (also register it separately for its own gRPC service)
	pb.RegisterProfilePhotoServiceServer(grpcServer, profilePhotoHandler)
	handler.RegisterSettingsHandler(grpcServer, settingsService)
	handler.RegisterUserEventsHandler(grpcServer, userEventsService, userRepo)
	handler.RegisterSearchHandler(grpcServer, searchService)

	// Start gRPC server
	port := getEnv("GRPC_PORT", "50051")
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Auth service listening on port %s", port)

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
