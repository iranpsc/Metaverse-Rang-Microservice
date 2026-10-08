package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/buildings-service/internal/handler"
	featurespb "metarang/shared/pb/features"
)

type muxBuildingAPI struct{}

func (muxBuildingAPI) GetBuildPackage(context.Context, *featurespb.GetBuildPackageRequest) (*featurespb.BuildPackageResponse, error) {
	return &featurespb.BuildPackageResponse{}, nil
}
func (muxBuildingAPI) BuildFeature(context.Context, *featurespb.BuildFeatureRequest) (*featurespb.BuildFeatureResponse, error) {
	return &featurespb.BuildFeatureResponse{}, nil
}
func (muxBuildingAPI) GetBuildings(context.Context, *featurespb.GetBuildingsRequest) (*featurespb.BuildingsResponse, error) {
	return &featurespb.BuildingsResponse{}, nil
}
func (muxBuildingAPI) UpdateBuilding(context.Context, *featurespb.UpdateBuildingRequest) (*featurespb.BuildingResponse, error) {
	return &featurespb.BuildingResponse{}, nil
}
func (muxBuildingAPI) UpdateBuildingInformation(context.Context, *featurespb.UpdateBuildingInformationRequest) (*featurespb.UpdateBuildingInformationResponse, error) {
	return &featurespb.UpdateBuildingInformationResponse{}, nil
}
func (muxBuildingAPI) DestroyBuilding(context.Context, *featurespb.DestroyBuildingRequest) (*featurespb.BuildingResponse, error) {
	return &featurespb.BuildingResponse{}, nil
}
func (muxBuildingAPI) ListCompletedBuildings(context.Context, *featurespb.ListCompletedBuildingsRequest) (*featurespb.ListCompletedBuildingsResponse, error) {
	return &featurespb.ListCompletedBuildingsResponse{
		Links: &featurespb.PaginationLinks{},
		Meta:  &featurespb.FeatureTradeHistoryPaginationMeta{},
	}, nil
}

func testPublicMux() http.Handler {
	return handler.NewPublicHTTPHandler(handler.HTTPServerHandlers{
		Buildings:        handler.NewHTTPBuildingsHandler(muxBuildingAPI{}),
		CitizenBuildings: handler.NewHTTPCitizenBuildingsHandler(nil, nil),
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
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/7/features", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("citizen features status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/7/buildings", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("citizen buildings nil auth status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewPublicHTTPHandler_CompletedAndBuildRoutes(t *testing.T) {
	h := testPublicMux()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("completed status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/features/42/build/buildings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get buildings status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/features/42", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("feature lookup status=%d", rec.Code)
	}
}

func TestNewPublicHTTPHandler_NilCitizenHandler(t *testing.T) {
	h := handler.NewPublicHTTPHandler(handler.HTTPServerHandlers{
		Buildings: handler.NewHTTPBuildingsHandler(muxBuildingAPI{}),
	}, nil, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/citizen/7/buildings", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nil citizen handler status=%d", rec.Code)
	}
}
