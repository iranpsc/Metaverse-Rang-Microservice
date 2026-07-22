package middleware

import (
	"log"
	"net/http"
)

// LoggingMiddleware creates an HTTP middleware that logs incoming requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📥 Incoming request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
