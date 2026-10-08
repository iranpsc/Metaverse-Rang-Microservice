package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/buildings-service/internal/handler"
)

func TestCorsPreflightMiddlewareAllowsPATCH(t *testing.T) {
	nextCalled := false
	h := handler.CorsPreflightMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/features/923/build/buildings/23", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected OPTIONS to be answered without calling next")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	allowMethods := rec.Header().Get("Access-Control-Allow-Methods")
	if allowMethods == "" || !containsCSVToken(allowMethods, "PATCH") {
		t.Fatalf("Allow-Methods %q does not include PATCH", allowMethods)
	}
}

func containsCSVToken(header, token string) bool {
	for _, part := range splitCSV(header) {
		if part == token {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	parts := make([]string, 0, 8)
	start := 0
	for i := 0; i <= len(value); i++ {
		if i == len(value) || value[i] == ',' {
			part := value[start:i]
			for len(part) > 0 && (part[0] == ' ' || part[0] == '\t') {
				part = part[1:]
			}
			for len(part) > 0 && (part[len(part)-1] == ' ' || part[len(part)-1] == '\t') {
				part = part[:len(part)-1]
			}
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	return parts
}
