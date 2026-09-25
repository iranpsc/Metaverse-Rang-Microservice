package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"metarang/shared/pkg/db"
	"metarang/shared/pkg/logger"
	"metarang/shared/pkg/sentry"
	"metarang/sitemap-generator-service/internal/handler"
	"metarang/sitemap-generator-service/internal/repository"
	"metarang/sitemap-generator-service/internal/service"
)

func main() {
	configPaths := []string{
		"services/sitemap-generator-service/config.env",
		"config.env",
		"./config.env",
		"../../config.env",
	}
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			break
		}
	}

	log := logger.NewLogger("sitemap-generator-service")
	log.Info("Starting Sitemap Generator Service...")

	if err := sentry.InitFromEnv("sitemap-generator-service"); err != nil {
		log.Warn("Failed to initialize Sentry", "error", err)
	}
	defer sentry.Flush(2 * time.Second)

	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	conn, err := db.NewConnection(db.Config{
		Host:     getEnv("DB_HOST", "mysql"),
		Port:     dbPort,
		User:     getEnv("DB_USER", "metarang_user"),
		Password: getEnv("DB_PASSWORD", "metarang_password"),
		Database: getEnv("DB_DATABASE", "metarang_db"),
	})
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	defer func() { _ = conn.DB.Close() }()
	log.Info("Database connected")

	exportPath := getEnv("SITEMAPS_EXPORT_PATH", "./sitemaps")
	writer, err := service.NewFileWriter(exportPath)
	if err != nil {
		log.Fatal("Failed to initialize sitemap writer", "error", err, "path", exportPath)
	}

	templatesPath := getEnv("TEMPLATES_PATH", "./templates/templates.json")
	adminPanelURL := getEnv("ADMIN_PANEL_URL", "https://admin.metarang.com")
	interval := parseDurationHours(getEnv("SITEMAP_SCHEDULER_INTERVAL_HOURS", "3"), 3*time.Hour)

	repo := repository.NewSitemapRepository(conn.DB)
	sitemapService := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath: templatesPath,
		AdminPanelURL: adminPanelURL,
	}).WithLogger(log)

	httpPort := getEnv("HTTP_PORT", "8071")
	httpHandler := handler.NewHTTPHandler(sitemapService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sitemapService.StartScheduler(ctx, interval, nil)

	go func() {
		log.Info("HTTP server listening", "port", httpPort)
		if err := handler.StartHTTPServer(httpHandler, httpPort); err != nil {
			log.Fatal("Failed to serve HTTP", "error", err)
		}
	}()

	log.Info("Sitemap Generator Service started",
		"http_port", httpPort,
		"export_path", exportPath,
		"templates_path", templatesPath,
		"scheduler_interval", interval.String(),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down gracefully...")
	cancel()
	log.Info("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDurationHours(raw string, fallback time.Duration) time.Duration {
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return fallback
	}
	return time.Duration(hours) * time.Hour
}
