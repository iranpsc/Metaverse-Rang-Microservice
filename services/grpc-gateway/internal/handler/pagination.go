package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

func buildSimplePaginationLinks(r *http.Request, currentPage int32, hasMore bool) map[string]interface{} {
	baseURL := requestBaseURL(r)
	query := r.URL.Query()

	links := map[string]interface{}{}

	query.Set("page", "1")
	links["first"] = baseURL + "?" + query.Encode()
	links["last"] = nil

	if currentPage > 1 {
		query.Set("page", strconv.FormatInt(int64(currentPage-1), 10))
		links["prev"] = baseURL + "?" + query.Encode()
	} else {
		links["prev"] = nil
	}

	if hasMore {
		query.Set("page", strconv.FormatInt(int64(currentPage+1), 10))
		links["next"] = baseURL + "?" + query.Encode()
	} else {
		links["next"] = nil
	}

	return links
}

// publicBaseURL returns the Kong/public gateway base (scheme+host) when APP_URL
// is set. Kong sets preserve_host=false, so r.Host is the upstream grpc-gateway
// host and must not be used in client-facing pagination links.
func publicBaseURL(r *http.Request) string {
	if appURL := strings.TrimSuffix(os.Getenv("APP_URL"), "/"); appURL != "" {
		return appURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}

func requestBaseURL(r *http.Request) string {
	return publicBaseURL(r) + r.URL.Path
}

func requestPath(r *http.Request) string {
	return publicBaseURL(r) + r.URL.Path
}
