package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"metarang/sitemap-generator-service/internal/service"
)

type testGenerator struct{ err error }

func (g testGenerator) GenerateAll(context.Context) error { return g.err }

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHTTPHandler(testGenerator{}).Health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGenerateRejectsInvalidRequests(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "secret")
	h := NewHTTPHandler(testGenerator{})
	for _, tc := range []struct {
		name   string
		method string
		token  string
		want   int
	}{
		{"method", http.MethodGet, "secret", http.StatusMethodNotAllowed},
		{"missing token", http.MethodPost, "", http.StatusUnauthorized},
		{"wrong token", http.MethodPost, "wrong", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/generate", nil)
			req.Header.Set("X-Service-Token", tc.token)
			rec := httptest.NewRecorder()
			h.Generate(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d", rec.Code, tc.want)
			}
		})
	}

	t.Setenv("INTERNAL_SERVICE_SECRET", "")
	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Header.Set("X-Service-Token", "secret")
	rec := httptest.NewRecorder()
	h.Generate(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty secret: got %d", rec.Code)
	}
}

func TestGenerateResultErrorsAndSuccess(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "secret")
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"busy", service.ErrGenerationInProgress, http.StatusConflict},
		{"failure", errors.New("database failure"), http.StatusInternalServerError},
		{"success", nil, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/generate", nil)
			req.Header.Set("X-Service-Token", "secret")
			rec := httptest.NewRecorder()
			NewHTTPHandler(testGenerator{err: tc.err}).Generate(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d", rec.Code, tc.want)
			}
		})
	}
}
