package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	corsOrigins := parseCORSOrigins(getEnv("CORS_ORIGIN", "http://localhost:5173"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validator, err := auth.NewValidator(ctx, authAddr)
	if err != nil {
		log.Fatalf("Failed to connect to auth service: %v", err)
	}

	eventHub := hub.New(validator, corsOrigins)
	defer func() {
		if err := eventHub.Close(); err != nil {
			log.Printf("Failed to close Socket.IO hub: %v", err)
		}
	}()

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
		Handler:           withCORS(mux, corsOrigins),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("WebSocket gateway listening on port %s", port)
		log.Printf("Redis URL: %s", redisURL)
		log.Printf("Auth Service: %s", authAddr)
		log.Printf("CORS origins: %v", corsOrigins)
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

func parseCORSOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			origins = append(origins, part)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}

func withCORS(next http.Handler, corsOrigins []string) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(corsOrigins))
	for _, origin := range corsOrigins {
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
