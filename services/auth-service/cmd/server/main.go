package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"metarang/auth-service/internal/auth"
	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/middleware"
	"metarang/auth-service/internal/pubsub"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	levelspb "metarang/shared/pb/levels"
	notificationspb "metarang/shared/pb/notifications"
	storagepb "metarang/shared/pb/storage"
	sharedauth "metarang/shared/pkg/auth"
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

	warnIfConfigMissing(loadEnvFiles())

	if err := sentry.InitFromEnv("auth-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	cfgRuntime := loadRuntimeConfig()

	mysqlCfg, err := buildMySQLConfig(buildDSN())
	if err != nil {
		log.Fatalf("Failed to parse DSN: %v", err)
	}

	db, err := openDatabase(mysqlCfg)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := pingDatabase(db); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Explicitly set charset to UTF-8 for proper Persian/Farsi text handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		log.Printf("Warning: Failed to set charset to utf8mb4: %v", err)
	} else {
		log.Println("Successfully set database charset to utf8mb4 for UTF-8/Persian text support")
	}

	log.Println("Successfully connected to database")

	redisURL := buildRedisURL()
	redisOpts, err := newRedisOptions(redisURL)
	if err != nil {
		cancel()
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	cancel()
	log.Println("Successfully connected to Redis")

	redisPublisher, err := pubsub.NewRedisPublisher(redisURL)
	if err != nil {
		log.Fatalf("Failed to create Redis publisher: %v", err)
	}

	userRepo := repository.NewUserRepository(db, cfgRuntime.AdminPanelURL)
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
	resetRepo := repository.NewResetRepository(db)

	var smsClient notificationspb.SMSServiceClient
	var notificationClient notificationspb.NotificationServiceClient
	notificationsConn, err := grpcutil.NewClient(cfgRuntime.NotificationsAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to notifications service: %v (continuing without notification support)", err)
	} else {
		defer func() { _ = notificationsConn.Close() }()
		smsClient = notificationspb.NewSMSServiceClient(notificationsConn)
		notificationClient = notificationspb.NewNotificationServiceClient(notificationsConn)
		log.Println("Successfully connected to notifications service")
	}

	observerService := service.NewObserverServiceWithSettings(
		userRepo,
		settingsRepo,
		activityRepo,
		redisPublisher,
		notificationClient,
	)

	helperService := service.NewHelperService(
		cfgRuntime.LevelsServiceAddr,
		cfgRuntime.FeaturesServiceAddr,
		cfgRuntime.CommercialServiceAddr,
	)

	walletConnectionService := service.NewWalletConnectionService(
		userRepo,
		cacheRepo,
		accountSecurityRepo,
		activityRepo,
		cfgRuntime.AppName,
		cfgRuntime.AppURL,
	)
	authService := service.NewAuthService(
		userRepo,
		tokenRepo,
		cacheRepo,
		accountSecurityRepo,
		activityRepo,
		observerService,
		helperService,
		smsClient,
		cfgRuntime.OAuthServerURL,
		cfgRuntime.OAuthClientID,
		cfgRuntime.OAuthClientSecret,
		cfgRuntime.AppURL,
		cfgRuntime.FrontEndURL,
		service.IsProductionEnv(cfgRuntime.AppEnv),
		service.WithResetRepository(resetRepo),
	)
	userService := service.NewUserServiceWithDependencies(
		userRepo,
		kycRepo,
		settingsRepo,
		profilePhotoRepo,
	)
	citizenService := service.NewCitizenService(
		citizenRepo,
		userRepo,
		helperService,
		cfgRuntime.AppURL,
	)
	personalInfoService := service.NewPersonalInfoService(personalInfoRepo)
	profileLimitationRepo := repository.NewProfileLimitationRepository(db)
	profileLimitationService := service.NewProfileLimitationService(profileLimitationRepo, userRepo)
	settingsService := service.NewSettingsServiceWithResets(settingsRepo, resetRepo)

	apiGatewayURL := resolveAPIGatewayURL()
	log.Printf("Profile photo service using API Gateway URL: %s", apiGatewayURL)

	var storageClient storagepb.FileStorageServiceClient
	storageConn, err := grpcutil.NewClient(cfgRuntime.StorageServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to storage service: %v (file uploads will fail)", err)
		storageClient = nil
	} else {
		defer func() { _ = storageConn.Close() }()
		storageClient = storagepb.NewFileStorageServiceClient(storageConn)
		log.Printf("Successfully connected to storage service at %s", cfgRuntime.StorageServiceAddr)
	}
	fileStorage := service.NewGRPCFileStorage(storageClient)

	kycService := service.NewKYCService(kycRepo, userRepo, fileStorage, apiGatewayURL)
	profilePhotoService := service.NewProfilePhotoService(profilePhotoRepo, fileStorage, apiGatewayURL)
	userEventsService := service.NewUserEventsService(activityRepo, userRepo)
	searchService := service.NewSearchService(searchRepo)

	serviceMetrics := metrics.NewMetrics("auth_service")
	metrics.StartHTTPServer(cfgRuntime.MetricsPort)
	tokenValidator := auth.NewLocalTokenValidator(tokenRepo)
	serverOpts, err := grpcutil.ServerOptions(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
			sharedauth.UnaryServerInterceptor(tokenValidator),
		),
	)
	if err != nil {
		log.Fatalf("Failed to configure gRPC server: %v", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	profilePhotoHandler := handler.NewProfilePhotoHandler(profilePhotoService)

	projectLocale := cfgRuntime.ProjectLocale
	handler.SetProjectLocale(projectLocale)
	authHandler := handler.RegisterAuthHandler(grpcServer, authService, tokenRepo, profilePhotoService, projectLocale)
	walletHandler := handler.RegisterWalletConnectionHandler(grpcServer, walletConnectionService, projectLocale)
	userHandler := handler.RegisterUserHandler(grpcServer, userService, profileLimitationService, helperService)
	kycHandler := handler.RegisterKYCHandler(grpcServer, kycService, apiGatewayURL)
	citizenHandler := handler.RegisterCitizenHandler(grpcServer, citizenService)
	personalInfoHandler := handler.RegisterPersonalInfoHandler(grpcServer, personalInfoService)
	profileLimitationHandler := handler.RegisterProfileLimitationHandler(grpcServer, profileLimitationService)
	pb.RegisterProfilePhotoServiceServer(grpcServer, profilePhotoHandler)
	settingsHandler := handler.RegisterSettingsHandler(grpcServer, settingsService)
	userEventsHandler := handler.RegisterUserEventsHandler(grpcServer, userEventsService, userRepo)
	searchHandler := handler.RegisterSearchHandler(grpcServer, searchService)

	localClients := handler.NewLocalClients(
		authHandler,
		userHandler,
		kycHandler,
		citizenHandler,
		personalInfoHandler,
		profileLimitationHandler,
		profilePhotoHandler,
		settingsHandler,
		userEventsHandler,
		searchHandler,
		walletHandler,
	)

	var levelClient levelspb.LevelServiceClient
	if levelsConn, err := grpcutil.NewClient(cfgRuntime.LevelsServiceAddr); err != nil {
		log.Printf("Warning: Failed to connect to levels service: %v (GetMe level enrichment disabled)", err)
	} else {
		defer func() { _ = levelsConn.Close() }()
		levelClient = levelspb.NewLevelServiceClient(levelsConn)
		log.Printf("Connected to levels service at %s", cfgRuntime.LevelsServiceAddr)
	}

	httpAuthHandler := handler.NewHTTPAuthHandler(localClients, levelClient, projectLocale)
	httpWalletHandler := handler.NewHTTPWalletHandler(localClients.WalletConnection, projectLocale)
	authMiddleware := middleware.WithLastSeen(
		middleware.AuthMiddleware(tokenValidator),
		middleware.LastSeenMiddleware(userRepo, redisPublisher),
	)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(tokenValidator)
	guestMiddleware := middleware.GuestMiddleware(tokenValidator)

	listener, err := net.Listen("tcp", grpcListenAddr(cfgRuntime.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfgRuntime.GRPCPort, err)
	}

	log.Printf("Auth service listening on gRPC port %s", cfgRuntime.GRPCPort)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	go func() {
		log.Printf("Auth service listening on HTTP port %s", cfgRuntime.HTTPPort)
		if err := handler.StartHTTPServer(handler.HTTPServerHandlers{
			Auth:   httpAuthHandler,
			Wallet: httpWalletHandler,
		}, cfgRuntime.HTTPPort, authMiddleware, optionalAuthMiddleware, guestMiddleware); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}
