package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"metarang/shared/pkg/sentry"
	"metarang/websocket-gateway/internal/auth"
	"metarang/websocket-gateway/internal/hub"
	"metarang/websocket-gateway/internal/redisbus"
)

func main() {
	loadConfig()

	if err := sentry.InitFromEnv("websocket-gateway"); err != nil {
		log.Printf("Warning: failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	port := getEnv("PORT", "3002")
	redisURL := getEnv("REDIS_URL", "redis://redis:6379")
	authAddr := getEnv("AUTH_SERVICE_ADDR", "auth-service:50051")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validator, err := auth.NewValidator(ctx, authAddr)
	if err != nil {
		log.Fatalf("Failed to connect to auth service: %v", err)
	}

	eventHub := hub.New(validator)
	subscriber, err := redisbus.NewSubscriber(ctx, redisURL, eventHub)
	if err != nil {
		log.Fatalf("Failed to subscribe to Redis: %v", err)
	}
	defer func() {
		if err := subscriber.Close(); err != nil {
			log.Printf("Failed to close Redis subscriber: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/socket.io/", eventHub)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		connections, users := eventHub.Stats()
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "healthy",
			"connections": connections,
			"users":       users,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		connections, users := eventHub.Stats()
		writeJSON(w, http.StatusOK, map[string]any{
			"totalConnections": connections,
			"totalUsers":       users,
			"uptime":           time.Since(startedAt).Seconds(),
			"timestamp":        time.Now().UTC().Format(time.RFC3339),
		})
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("WebSocket gateway listening on port %s", port)
		log.Printf("Redis URL: %s", redisURL)
		log.Printf("Auth Service: %s", authAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}

var startedAt = time.Now()

func loadConfig() {
	paths := []string{
		"services/websocket-gateway/config.env",
		"config.env",
		"./config.env",
		"../../config.env",
	}
	for _, path := range paths {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
