package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"

	"metarang/notifications-service/internal/handler"
	"metarang/notifications-service/internal/middleware"
	"metarang/notifications-service/internal/repository"
	"metarang/notifications-service/internal/service"
	authpb "metarang/shared/pb/auth"
	grpcutil "metarang/shared/pkg/grpc"
	"metarang/shared/pkg/metrics"
	"metarang/shared/pkg/sentry"
)

func main() {
	warnIfConfigMissing(loadEnvFiles())

	if err := sentry.InitFromEnv("notifications-service"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to prepare database connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := pingDatabase(db); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to database")

	metrics.StartHTTPServer(getEnv("METRICS_PORT", "9090"))

	grpcServer, err := newGRPCServer()
	if err != nil {
		log.Fatalf("Failed to configure gRPC server: %v", err)
	}

	notificationHandler := registerNotificationServices(grpcServer, db)

	authClient, authConn := dialAuthClient()
	if authConn != nil {
		defer func() { _ = authConn.Close() }()
	}

	port := getEnv("GRPC_PORT", "50058")
	listener, err := net.Listen("tcp", grpcListenAddr(port))
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("gRPC server listening on port %s", port)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	httpHandler := handler.NewHTTPNotificationHandler(notificationHandler)
	httpPort := getEnv("HTTP_PORT", "8063")
	authMW := middleware.AuthMiddleware(authClient)

	log.Printf("HTTP server listening on port %s", httpPort)
	go func() {
		if err := handler.StartHTTPServer(httpHandler, httpPort, authMW); err != nil {
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

func newGRPCServer() (*grpc.Server, error) {
	serviceMetrics := metrics.NewMetrics("notifications_service")
	serverOpts, err := grpcutil.ServerOptions(
		grpc.ChainUnaryInterceptor(
			sentry.UnaryServerInterceptor(),
			metrics.UnaryServerInterceptor(serviceMetrics),
		),
	)
	if err != nil {
		return nil, err
	}
	return grpc.NewServer(serverOpts...), nil
}

func registerNotificationServices(grpcServer *grpc.Server, db *sql.DB) *handler.NotificationHandler {
	notificationRepo := repository.NewNotificationRepository(db)

	smsCfg := loadSMSChannelConfig()
	logSMSConfig(smsCfg)
	smsChannel := service.NewSMSChannel(smsCfg)
	emailChannel := service.NewEmailChannelFromConfig(loadEmailChannelConfig())

	notificationService := service.NewNotificationService(notificationRepo, smsChannel, emailChannel)
	smsService := service.NewSMSService(smsChannel)
	emailService := service.NewEmailService(emailChannel)

	notificationHandler := handler.RegisterNotificationHandler(grpcServer, notificationService)
	handler.RegisterSMSHandler(grpcServer, smsService)
	handler.RegisterEmailHandler(grpcServer, emailService)
	return notificationHandler
}

func dialAuthClient() (authpb.AuthServiceClient, *grpc.ClientConn) {
	authAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")
	authConn, err := grpcutil.NewClient(authAddr)
	if err != nil {
		log.Printf("Warning: failed to connect to auth service at %s: %v", authAddr, err)
		return nil, nil
	}
	log.Printf("Created auth service client for %s", authAddr)
	return authpb.NewAuthServiceClient(authConn), authConn
}
