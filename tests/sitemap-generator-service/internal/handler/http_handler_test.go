package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"metarang/sitemap-generator-service/internal/handler"
	"metarang/sitemap-generator-service/internal/service"
)

type stubGenerator struct {
	err error
}

func (s *stubGenerator) GenerateAll(ctx context.Context) error {
	return s.err
}

func TestGenerate_RequiresServiceToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")
	h := handler.NewHTTPHandler(&stubGenerator{})

	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	rec := httptest.NewRecorder()
	h.Generate(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", rec.Code)
	}
}

func TestGenerate_SuccessWithToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")
	h := handler.NewHTTPHandler(&stubGenerator{})

	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Header.Set("X-Service-Token", "test-secret")
	rec := httptest.NewRecorder()
	h.Generate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestGenerate_GenericError(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")
	h := handler.NewHTTPHandler(&stubGenerator{err: errors.New("db exploded: password=secret")})

	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Header.Set("X-Service-Token", "test-secret")
	rec := httptest.NewRecorder()
	h.Generate(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "generation failed" {
		t.Fatalf("leaked error: %v", body)
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "db exploded") {
		t.Fatalf("internal error leaked: %s", rec.Body.String())
	}
}

func TestGenerate_ConflictWhenBusy(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")
	h := handler.NewHTTPHandler(&stubGenerator{err: service.ErrGenerationInProgress})

	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Header.Set("X-Service-Token", "test-secret")
	rec := httptest.NewRecorder()
	h.Generate(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d want 409", rec.Code)
	}
}
