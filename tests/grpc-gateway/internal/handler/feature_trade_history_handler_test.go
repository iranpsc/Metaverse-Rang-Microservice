package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/grpc-gateway/internal/handler"
)

func TestFeatureIDFromTradeHistoryPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"123/trade-history", "123"},
		{"123/trade-history/", "123"},
		{"123/trade-history.", "123"},
		{"42/trade-history.", "42"},
		{"trade-history", ""},
		{"123/build/package", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := handler.FeatureIDFromTradeHistoryPath(tt.path); got != tt.want {
			t.Fatalf("FeatureIDFromTradeHistoryPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsFeatureTradeHistoryPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"123/trade-history", true},
		{"123/trade-history.", true},
		{"123/build/package", false},
		{"trade-history", false},
	}

	for _, tt := range tests {
		if got := handler.IsFeatureTradeHistoryPath(tt.path); got != tt.want {
			t.Fatalf("IsFeatureTradeHistoryPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExtractFeatureIDFromTradeHistoryPath_PathValue(t *testing.T) {
	mux := http.NewServeMux()
	var got string
	mux.HandleFunc("GET /api/features/{feature}/trade-history", func(w http.ResponseWriter, r *http.Request) {
		got = handler.ExtractFeatureIDFromTradeHistoryPath(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/features/99/trade-history", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	if got != "99" {
		t.Fatalf("PathValue feature ID = %q, want 99", got)
	}
}

func TestExtractFeatureIDFromTradeHistoryPath_URLFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/features/77/trade-history", nil)
	if got := handler.ExtractFeatureIDFromTradeHistoryPath(req); got != "77" {
		t.Fatalf("fallback feature ID = %q, want 77", got)
	}
}
