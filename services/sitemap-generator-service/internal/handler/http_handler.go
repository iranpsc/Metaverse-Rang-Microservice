// Package handler serves HTTP health and regenerate endpoints for sitemap generation.
package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"metarang/sitemap-generator-service/internal/service"
)

// Generator triggers sitemap generation.
type Generator interface {
	GenerateAll(ctx context.Context) error
}

// HTTPHandler serves health and manual regenerate endpoints.
type HTTPHandler struct {
	generator  Generator
	management *service.ManagementService
}

// NewHTTPHandler creates an HTTPHandler.
func NewHTTPHandler(generator Generator, management ...*service.ManagementService) *HTTPHandler {
	h := &HTTPHandler{generator: generator}
	if len(management) > 0 {
		h.management = management[0]
	}
	return h
}

// StartHTTPServer starts the HTTP server on the given port.
func StartHTTPServer(h *HTTPHandler, port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/generate", h.Generate)
	mux.HandleFunc("/management/sitemaps", h.Sitemaps)
	mux.HandleFunc("/management/generate", h.ManagementGenerate)
	mux.HandleFunc("/management/upload", h.Upload)
	mux.HandleFunc("/management/delete/", h.Delete)
	mux.HandleFunc("/management/logs", h.Logs)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func (h *HTTPHandler) authorized(r *http.Request) bool {
	return validateServiceToken(r.Header.Get("X-Service-Token"))
}

func (h *HTTPHandler) Sitemaps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !h.authorized(r) || h.management == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	items, err := h.management.ListFiles()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	language, fileType, status := r.URL.Query().Get("language"), r.URL.Query().Get("type"), r.URL.Query().Get("status")
	filtered := items[:0]
	for _, item := range items {
		if language != "" && item.Language != language {
			continue
		}
		if fileType != "" && item.Type != fileType {
			continue
		}
		if status != "" && item.Status != status {
			continue
		}
		filtered = append(filtered, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": filtered})
}

func (h *HTTPHandler) ManagementGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !h.authorized(r) || h.management == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	trigger, target, language := "manual", "all", ""
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Target   string `json:"target"`
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.Target != "" {
				target = body.Target
			}
			language = body.Language
		}
	}
	run, err := h.management.Generate(r.Context(), trigger, target, language)
	if err != nil {
		if errors.Is(err, service.ErrGenerationInProgress) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "generation already in progress", "run": run})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": run})
}

func (h *HTTPHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !h.authorized(r) || h.management == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}
	language := r.FormValue("language")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "file is required")
		return
	}
	item, uploadErr := h.management.Upload(language, header.Filename, file)
	closeErr := file.Close()
	if uploadErr != nil {
		writeJSONError(w, http.StatusBadRequest, uploadErr.Error())
		return
	}
	if closeErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "close uploaded file")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *HTTPHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete || !h.authorized(r) || h.management == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/management/delete/"), "/"), "/")
	if len(parts) != 2 {
		writeJSONError(w, http.StatusBadRequest, "language and filename are required")
		return
	}
	if err := h.management.Delete(parts[0], parts[1]); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": nil})
}

func (h *HTTPHandler) Logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !h.authorized(r) || h.management == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	items, total, err := h.management.LogsPage(r.Context(), page, perPage)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	lastPage := (total + perPage - 1) / perPage
	if lastPage == 0 {
		lastPage = 1
	}
	from := 0
	to := 0
	if len(items) > 0 {
		from = (page-1)*perPage + 1
		to = from + len(items) - 1
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"logs":       items,
			"pagination": map[string]int{"current_page": page, "last_page": lastPage, "per_page": perPage, "total": total, "from": from, "to": to},
		},
	})
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func validateServiceToken(token string) bool {
	secret := os.Getenv("INTERNAL_SERVICE_SECRET")
	if secret == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}
