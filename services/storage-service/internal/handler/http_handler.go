// Package handler provides HTTP and gRPC handlers for the storage service.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"metarang/shared/pkg/sentry"
	"metarang/storage-service/internal/service"
)

// HTTPHandler handles HTTP REST requests for chunk uploads
type HTTPHandler struct {
	storageService *service.StorageService
	uploadRoot     string
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(storageService *service.StorageService, uploadRoot string) *HTTPHandler {
	return &HTTPHandler{
		storageService: storageService,
		uploadRoot:     uploadRoot,
	}
}

// ChunkUploadHTTPRequest represents the HTTP request for chunk upload
type ChunkUploadHTTPRequest struct {
	UploadID    string `json:"upload_id"`
	ChunkIndex  int32  `json:"chunk_index"`
	TotalChunks int32  `json:"total_chunks"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	TotalSize   int64  `json:"total_size"`
	UploadPath  string `json:"upload_path,omitempty"`
}

// ChunkUploadHTTPResponse represents the HTTP response for chunk upload
type ChunkUploadHTTPResponse struct {
	Success        bool    `json:"success"`
	Message        string  `json:"message"`
	PercentageDone float64 `json:"done"`
	IsFinished     bool    `json:"is_finished,omitempty"`
	FileURL        string  `json:"path,omitempty"`
	FileName       string  `json:"name,omitempty"`
	MimeType       string  `json:"mime_type,omitempty"`
}

// HandleChunkUpload handles the chunk upload HTTP endpoint
// POST /upload — receive a chunk (legacy form fields or resumable.js fields)
// GET  /upload — resumable.js chunk existence test (200 = exists, 204 = missing)
func (h *HTTPHandler) HandleChunkUpload(w http.ResponseWriter, r *http.Request) {
	h.setUploadCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodGet:
		h.handleChunkTest(w, r)
		return
	case http.MethodPost:
		h.handleChunkPost(w, r)
		return
	default:
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *HTTPHandler) setUploadCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Cache-Control, X-Requested-With")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
}

// handleChunkTest answers resumable.js GET probes: 200 if chunk exists, 204 otherwise.
func (h *HTTPHandler) handleChunkTest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	uploadID := firstFormValue(r, "resumableIdentifier", "upload_id")
	if uploadID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	chunkIndex, ok := parseChunkIndex(r)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if h.storageService.HasUploadedChunk(uploadID, chunkIndex) {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) handleChunkPost(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with max memory of 10MB for metadata
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// Get file from form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get file: %v", err))
		return
	}
	defer func() { _ = file.Close() }()

	// Read chunk data
	chunkData, err := io.ReadAll(file)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read chunk: %v", err))
		return
	}

	uploadID := firstFormValue(r, "resumableIdentifier", "upload_id")
	if uploadID == "" {
		// Generate upload ID if not provided
		uploadID = fmt.Sprintf("upload_%d", fileHeader.Size)
	}

	chunkIndex, ok := parseChunkIndex(r)
	if !ok {
		chunkIndex = 0 // Default to 0 if not provided (single chunk)
	}

	totalChunks, err := strconv.ParseInt(firstFormValue(r, "resumableTotalChunks", "total_chunks"), 10, 32)
	if err != nil {
		totalChunks = 1 // Default to 1 if not provided (single chunk)
	}

	totalSize, err := strconv.ParseInt(firstFormValue(r, "resumableTotalSize", "total_size"), 10, 64)
	if err != nil {
		totalSize = fileHeader.Size // Use current chunk size if not provided
	}

	filename := firstFormValue(r, "resumableFilename", "filename")
	if filename == "" {
		filename = fileHeader.Filename
	}

	contentType := firstFormValue(r, "resumableType", "content_type")
	if contentType == "" {
		contentType = fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	uploadPath := r.FormValue("upload_path")

	// Handle chunk upload
	// Returns: isFinished, progress, filePath (relative path like "uploads/mime/date/"), finalFilename, mimeType, error
	isFinished, progress, filePath, finalFilename, mimeType, err := h.storageService.HandleChunkUpload(
		uploadID,
		filename,
		contentType,
		chunkData,
		chunkIndex,
		int32(totalChunks),
		totalSize,
		uploadPath,
	)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Upload failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if isFinished {
		// Completed upload: { "path": "uploads/<mime>/<date>/", "name": "<hash>.<ext>", "mime_type": "<mime>" }
		response := map[string]interface{}{
			"path":      filePath,      // e.g., "uploads/image-jpeg/2024-01-15/"
			"name":      finalFilename, // e.g., "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6.jpg"
			"mime_type": mimeType,      // e.g., "image/jpeg"
		}
		_ = json.NewEncoder(w).Encode(response)
	} else {
		// In-progress chunk: { "done": <float 0-100> }
		response := map[string]interface{}{
			"done": progress,
		}
		_ = json.NewEncoder(w).Encode(response)
	}
}

// firstFormValue returns the first non-empty form/query value among the given keys.
func firstFormValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if v := r.FormValue(key); v != "" {
			return v
		}
	}
	return ""
}

// parseChunkIndex reads resumable.js (1-based) or legacy (0-based) chunk index fields.
func parseChunkIndex(r *http.Request) (int32, bool) {
	if v := r.FormValue("resumableChunkNumber"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 {
			return 0, false
		}
		return int32(n - 1), true
	}

	if v := r.FormValue("chunk_index"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			return 0, false
		}
		return int32(n), true
	}

	return 0, false
}

// HandleHealthCheck handles health check endpoint
func (h *HTTPHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "storage-service",
		"version": "1.0.0",
	})
}

// sendError sends an error response
func (h *HTTPHandler) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// ServeUploads serves GET /uploads/{path} from the local upload directory.
// Example: /uploads/profile/abc.png -> {uploadRoot}/profile/abc.png
func (h *HTTPHandler) ServeUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/uploads")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}

	root := h.uploadRoot
	if root == "" {
		root = "uploads"
	}

	filePath := filepath.Join(root, filepath.FromSlash(rel))
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		legacy := filepath.Join(root, "uploads", filepath.FromSlash(rel))
		if info, err := os.Stat(legacy); err == nil && !info.IsDir() {
			filePath = legacy
		}
	}
	http.ServeFile(w, r, filePath)
}

// RegisterHTTPRoutes registers all HTTP routes
func (h *HTTPHandler) RegisterHTTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/upload", h.HandleChunkUpload)
	mux.HandleFunc("/health", h.HandleHealthCheck)
	mux.HandleFunc("/api/upload", h.HandleChunkUpload) // Also support /api/upload
	mux.HandleFunc("/uploads/", h.ServeUploads)
}

// StartHTTPServer starts the HTTP server
func StartHTTPServer(handler *HTTPHandler, port string) error {
	mux := http.NewServeMux()
	handler.RegisterHTTPRoutes(mux)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}

	return server.ListenAndServe()
}
