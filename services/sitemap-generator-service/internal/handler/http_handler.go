// Package handler serves HTTP health and regenerate endpoints for sitemap generation.
package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"metarang/sitemap-generator-service/internal/service"
)

// Generator triggers sitemap generation.
type Generator interface {
	GenerateAll(ctx context.Context) error
}

// HTTPHandler serves health and manual regenerate endpoints.
type HTTPHandler struct {
	generator Generator
}

// NewHTTPHandler creates an HTTPHandler.
func NewHTTPHandler(generator Generator) *HTTPHandler {
	return &HTTPHandler{generator: generator}
}

// StartHTTPServer starts the HTTP server on the given port.
func StartHTTPServer(h *HTTPHandler, port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/generate", h.Generate)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

// Health returns a simple OK response.
func (h *HTTPHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Generate triggers an immediate sitemap regeneration.
// Requires X-Service-Token matching INTERNAL_SERVICE_SECRET.
func (h *HTTPHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !validateServiceToken(r.Header.Get("X-Service-Token")) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.generator.GenerateAll(r.Context()); err != nil {
		if errors.Is(err, service.ErrGenerationInProgress) {
			writeJSONError(w, http.StatusConflict, "generation already in progress")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "generation failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "generated"})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func validateServiceToken(token string) bool {
	secret := os.Getenv("INTERNAL_SERVICE_SECRET")
	if secret == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}
