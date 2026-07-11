package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"metargb/shared/pkg/sentry"
	"metargb/storage-service/internal/service"
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

// ChunkUploadRequest represents the HTTP request for chunk upload
type ChunkUploadHTTPRequest struct {
	UploadID    string `json:"upload_id"`
	ChunkIndex  int32  `json:"chunk_index"`
	TotalChunks int32  `json:"total_chunks"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	TotalSize   int64  `json:"total_size"`
	UploadPath  string `json:"upload_path,omitempty"`
}

// ChunkUploadResponse represents the HTTP response for chunk upload
type ChunkUploadHTTPResponse struct {
	Success        bool    `json:"success"`
	Message        string  `json:"message"`
	PercentageDone float64 `json:"done"` // Match Laravel's response key
	IsFinished     bool    `json:"is_finished,omitempty"`
	FileURL        string  `json:"path,omitempty"`      // Match Laravel's response key
	FileName       string  `json:"name,omitempty"`      // Match Laravel's response key
	MimeType       string  `json:"mime_type,omitempty"` // Match Laravel's response key
}

// HandleChunkUpload handles the chunk upload HTTP endpoint
// POST /upload
func (h *HTTPHandler) HandleChunkUpload(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

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
	defer file.Close()

	// Read chunk data
	chunkData, err := io.ReadAll(file)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read chunk: %v", err))
		return
	}

	// Parse metadata from form values
	uploadID := r.FormValue("upload_id")
	if uploadID == "" {
		// Generate upload ID if not provided
		uploadID = fmt.Sprintf("upload_%d", fileHeader.Size)
	}

	chunkIndex, err := strconv.ParseInt(r.FormValue("chunk_index"), 10, 32)
	if err != nil {
		chunkIndex = 0 // Default to 0 if not provided (single chunk)
	}

	totalChunks, err := strconv.ParseInt(r.FormValue("total_chunks"), 10, 32)
	if err != nil {
		totalChunks = 1 // Default to 1 if not provided (single chunk)
	}

	totalSize, err := strconv.ParseInt(r.FormValue("total_size"), 10, 64)
	if err != nil {
		totalSize = fileHeader.Size // Use current chunk size if not provided
	}

	filename := r.FormValue("filename")
	if filename == "" {
		filename = fileHeader.Filename
	}

	contentType := r.FormValue("content_type")
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
		int32(chunkIndex),
		int32(totalChunks),
		totalSize,
		uploadPath,
	)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Upload failed: %v", err))
		return
	}

	// Build response matching Laravel format exactly
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if isFinished {
		// Completed upload: { "path": "uploads/<mime>/<date>/", "name": "<hash>.<ext>", "mime_type": "<mime>" }
		response := map[string]interface{}{
			"path":      filePath,      // e.g., "uploads/image-jpeg/2024-01-15/"
			"name":      finalFilename, // e.g., "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6.jpg"
			"mime_type": mimeType,      // e.g., "image/jpeg"
		}
		json.NewEncoder(w).Encode(response)
	} else {
		// In-progress chunk: { "done": <float 0-100> }
		response := map[string]interface{}{
			"done": progress,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// HandleHealthCheck handles health check endpoint
func (h *HTTPHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "storage-service",
		"version": "1.0.0",
	})
}

// sendError sends an error response
func (h *HTTPHandler) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
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
