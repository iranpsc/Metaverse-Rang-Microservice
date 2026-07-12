package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type StorageHandler struct {
	proxy *httputil.ReverseProxy
}

func NewStorageHandler(storageServiceAddr string) *StorageHandler {
	// Parse the storage service URL
	targetURL, err := url.Parse("http://" + storageServiceAddr)
	if err != nil {
		// If parsing fails, log error but continue (will fail on first request)
		// In production, you might want to handle this differently
		return &StorageHandler{
			proxy: nil,
		}
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the proxy director to preserve the original request path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// The path should already be set correctly by the reverse proxy
		// We just need to ensure the host is correct
		req.Host = targetURL.Host
	}

	return &StorageHandler{
		proxy: proxy,
	}
}

// HandleUpload handles POST /api/upload
// This proxies the request to the storage service's HTTP endpoint
func (h *StorageHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if h.proxy == nil {
		writeError(w, http.StatusServiceUnavailable, "storage service not available")
		return
	}

	// Modify the request path to forward to /api/upload on storage service
	// The storage service handles both /upload and /api/upload
	r.URL.Path = "/api/upload"

	// Forward the request to storage service
	h.proxy.ServeHTTP(w, r)
}
