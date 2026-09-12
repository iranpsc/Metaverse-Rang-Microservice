package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testPublicMux() http.Handler {
	return newPublicHTTPHandler(HTTPServerHandlers{
		Features:        &HTTPFeaturesHandler{},
		Profit:          &HTTPProfitHandler{},
		Maps:            &HTTPMapsHandler{},
		Isic:            &HTTPIsicCodesHandler{},
		CitizenFeatures: &HTTPCitizenFeaturesHandler{},
	}, nil, nil, nil)
}

func TestNewPublicHTTPHandler_HealthAndCitizen(t *testing.T) {
	h := testPublicMux()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health status=%d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body=%v", body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/health", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("options status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("citizen empty status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/7/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("citizen unknown status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/7/buildings", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("citizen buildings nil handler status=%d", rec.Code)
	}
}
