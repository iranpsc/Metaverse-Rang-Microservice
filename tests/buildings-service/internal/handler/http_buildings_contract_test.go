package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/buildings-service/internal/handler"
	pb "metarang/shared/pb/features"
	authpkg "metarang/shared/pkg/auth"
)

type mockHTTPBuildingAPI struct {
	getBuildPackage    func(context.Context, *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error)
	buildFeature       func(context.Context, *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error)
	getBuildings       func(context.Context, *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error)
	updateBuilding     func(context.Context, *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error)
	updateInformation  func(context.Context, *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error)
	destroyBuilding    func(context.Context, *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error)
	completedBuildings func(context.Context, *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error)
}

func (m *mockHTTPBuildingAPI) GetBuildPackage(ctx context.Context, req *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error) {
	if m.getBuildPackage != nil {
		return m.getBuildPackage(ctx, req)
	}
	return &pb.BuildPackageResponse{
		Models:      []*pb.BuildingModel{{Id: 1, ModelId: "1", Name: "n", Sku: "s", Images: "[]", Attributes: "[]", File: "{}", RequiredSatisfaction: "1"}},
		Coordinates: []string{"1,2"},
	}, nil
}
func (m *mockHTTPBuildingAPI) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error) {
	if m.buildFeature != nil {
		return m.buildFeature(ctx, req)
	}
	return &pb.BuildFeatureResponse{}, nil
}
func (m *mockHTTPBuildingAPI) GetBuildings(ctx context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
	if m.getBuildings != nil {
		return m.getBuildings(ctx, req)
	}
	return &pb.BuildingsResponse{}, nil
}
func (m *mockHTTPBuildingAPI) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
	if m.updateBuilding != nil {
		return m.updateBuilding(ctx, req)
	}
	return &pb.BuildingResponse{}, nil
}
func (m *mockHTTPBuildingAPI) UpdateBuildingInformation(ctx context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error) {
	if m.updateInformation != nil {
		return m.updateInformation(ctx, req)
	}
	return &pb.UpdateBuildingInformationResponse{}, nil
}
func (m *mockHTTPBuildingAPI) DestroyBuilding(ctx context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
	if m.destroyBuilding != nil {
		return m.destroyBuilding(ctx, req)
	}
	return &pb.BuildingResponse{}, nil
}
func (m *mockHTTPBuildingAPI) ListCompletedBuildings(ctx context.Context, req *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error) {
	if m.completedBuildings != nil {
		return m.completedBuildings(ctx, req)
	}
	return &pb.ListCompletedBuildingsResponse{}, nil
}

func requestWithUser(req *http.Request, userID uint64) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: userID}))
}

func TestHTTPBuildingMutationRoutes(t *testing.T) {
	var updated, patched, destroyed *pb.BuildingInformation
	building := &mockHTTPBuildingAPI{
		updateBuilding: func(_ context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			updated = &pb.BuildingInformation{}
			return &pb.BuildingResponse{}, nil
		},
		updateInformation: func(_ context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			patched = req.Information
			return &pb.UpdateBuildingInformationResponse{Information: req.Information}, nil
		},
		destroyBuilding: func(_ context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			destroyed = &pb.BuildingInformation{}
			return &pb.BuildingResponse{}, nil
		},
	}
	h := handler.NewHTTPBuildingsHandler(building)
	for _, test := range []struct {
		name, method, target, body string
	}{
		{"PUT", http.MethodPut, "/api/features/42/build/buildings/1001", `{"launched_satisfaction":"50"}`},
		{"POST method PUT", http.MethodPost, "/api/features/42/build/buildings/1001?_method=put", `{"launched_satisfaction":"50"}`},
		{"PATCH", http.MethodPatch, "/api/features/42/build/buildings/1001", `{"information":{"name":"Updated Store"}}`},
		{"POST method PATCH", http.MethodPost, "/api/features/42/build/buildings/1001?_method=patch", `{"information":{"name":"Updated Store"}}`},
		{"DELETE", http.MethodDelete, "/api/features/42/build/buildings/1001", ""},
		{"POST method DELETE", http.MethodPost, "/api/features/42/build/buildings/1001?_method=delete", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := requestWithUser(httptest.NewRequest(test.method, test.target, bytes.NewBufferString(test.body)), 7)
			req.Header.Set("Content-Type", "application/json")
			h.HandleFeaturesBuildRoutes(w, req)
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		})
	}
	assert.NotNil(t, updated)
	assert.Equal(t, "Updated Store", patched.Name)
	assert.NotNil(t, destroyed)
}

func TestHTTPCompletedBuildingsRoutes(t *testing.T) {
	building := &mockHTTPBuildingAPI{completedBuildings: func(_ context.Context, _ *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error) {
		return &pb.ListCompletedBuildingsResponse{
			Data:  []*pb.CompletedBuilding{{Id: 1, FeatureId: 10, FeaturePropertiesId: "QA-1"}},
			Links: &pb.PaginationLinks{}, Meta: &pb.FeatureTradeHistoryPaginationMeta{},
		}, nil
	}}
	h := handler.NewHTTPBuildingsHandler(building)

	t.Run("takes precedence over feature lookup", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "invalid feature ID")
	})
	t.Run("specific mux registration takes precedence", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle("GET /api/features/buildings/completed", http.HandlerFunc(h.ListCompletedBuildings))
		mux.Handle("/api/features/", http.HandlerFunc(h.HandleFeaturesBuildRoutes))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("building list still routes correctly", func(t *testing.T) {
		building.getBuildings = func(_ context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			return &pb.BuildingsResponse{}, nil
		}
		w := httptest.NewRecorder()
		h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42/build/buildings", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHTTPBuildPackageShape(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/features/1/build/package", nil), 7)
	h.BuildPackage(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)
	model := data[0].(map[string]interface{})
	assert.Equal(t, float64(1), model["id"])
	assert.Equal(t, "1", model["model_id"])
	assert.Equal(t, "n", model["name"])
	assert.Equal(t, "s", model["sku"])
	assert.Equal(t, "1", model["required_satisfaction"])
	feature, ok := body["feature"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{"1,2"}, feature["coordinates"])
}

func TestHTTPBuildPackageRequiresAuth(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	h.BuildPackage(w, httptest.NewRequest(http.MethodGet, "/api/features/1/build/package", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "authentication required", body["error"])
}

func TestHTTPBuildFeatureRoute(t *testing.T) {
	var got *pb.BuildFeatureRequest
	building := &mockHTTPBuildingAPI{buildFeature: func(_ context.Context, req *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error) {
		got = req
		return &pb.BuildFeatureResponse{}, nil
	}}
	h := handler.NewHTTPBuildingsHandler(building)
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/features/42/build/1001", bytes.NewBufferString(`{"launched_satisfaction":"25","rotation":"0","position":"1,2"}`)), 7)
	req.Header.Set("Content-Type", "application/json")
	h.HandleFeaturesBuildRoutes(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, got)
	assert.Equal(t, uint64(42), got.FeatureId)
	assert.Equal(t, "1001", got.BuildingModelId)
	assert.Equal(t, "25", got.LaunchedSatisfaction)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasData := body["data"]
	assert.True(t, hasData)
}

func TestHTTPBuildFeatureRequiresAuthAndPost(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})

	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42/build/1001", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/features/42/build/1001", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	h.HandleFeaturesBuildRoutes(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTPGetBuildingsShape(t *testing.T) {
	building := &mockHTTPBuildingAPI{getBuildings: func(_ context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
		assert.Equal(t, uint64(42), req.FeatureId)
		return &pb.BuildingsResponse{Buildings: []*pb.Building{{
			ConstructionStartDate: "2026-01-01T00:00:00Z",
			ConstructionEndDate:   "2026-02-01T00:00:00Z",
			LaunchedSatisfaction:  "50",
			Rotation:              "45",
			Position:              "1,2",
			BubbleDiameter:        "3.5",
			Information:           `{"name":"Store"}`,
			Model: &pb.BuildingModel{
				Id: 11, ModelId: "1001", Name: "Tower", Sku: "sku-1",
				Images: "[]", Attributes: "[]", File: `{"url":"m.glb"}`, RequiredSatisfaction: "1",
			},
		}}}, nil
	}}
	h := handler.NewHTTPBuildingsHandler(building)
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42/build/buildings", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	model := data[0].(map[string]interface{})
	assert.Equal(t, float64(11), model["id"])
	assert.Equal(t, "1001", model["model_id"])
	assert.Equal(t, "Tower", model["name"])
	assert.Equal(t, "sku-1", model["sku"])
	file := model["file"].(map[string]interface{})
	assert.Equal(t, "m.glb", file["url"])
	nested := model["building"].(map[string]interface{})
	assert.Equal(t, "1001", nested["model_id"])
	assert.Equal(t, float64(42), nested["feature_id"])
	assert.Equal(t, "45", nested["rotation"])
	assert.Equal(t, "50", nested["launched_satisfaction"])
	assert.Equal(t, "3.5", nested["bubble_diameter"])
	info := nested["information"].(map[string]interface{})
	assert.Equal(t, "Store", info["name"])
}

func TestHTTPCompletedBuildingsShape(t *testing.T) {
	length, width, density := "30", "50", "3"
	from, to := int32(1), int32(1)
	building := &mockHTTPBuildingAPI{completedBuildings: func(_ context.Context, req *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error) {
		assert.Equal(t, int32(2), req.Page)
		return &pb.ListCompletedBuildingsResponse{
			Data: []*pb.CompletedBuilding{{
				Id: 42, FeatureId: 7, FeaturePropertiesId: "QA-1",
				Length: &length, Width: &width, Density: &density, Karbari: "m",
			}},
			Links: &pb.PaginationLinks{First: "/api/features/buildings/completed?page=1", Last: "/api/features/buildings/completed?page=2"},
			Meta: &pb.FeatureTradeHistoryPaginationMeta{
				CurrentPage: 2, LastPage: 2, Path: "/api/features/buildings/completed",
				PerPage: 10, Total: 11, From: &from, To: &to,
			},
		}, nil
	}}
	h := handler.NewHTTPBuildingsHandler(building)
	w := httptest.NewRecorder()
	h.ListCompletedBuildings(w, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed?page=2", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	assert.Equal(t, float64(42), item["id"])
	assert.Equal(t, float64(7), item["feature_id"])
	assert.Equal(t, "QA-1", item["feature_properties_id"])
	assert.Equal(t, "30", item["length"])
	assert.Equal(t, "50", item["width"])
	assert.Equal(t, "3", item["density"])
	assert.Equal(t, "m", item["karbari"])
	links := body["links"].(map[string]interface{})
	assert.Contains(t, links["first"].(string), "page=1")
	meta := body["meta"].(map[string]interface{})
	assert.Equal(t, float64(2), meta["current_page"])
	assert.Equal(t, float64(11), meta["total"])
}

func TestHTTPInvalidFeatureID(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/abc/build/buildings", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "invalid feature ID", body["error"])
}

func TestHTTPUnknownBuildPath(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPPatchInformationRequired(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodPatch, "/api/features/42/build/buildings/1001", bytes.NewBufferString(`{"information":{}}`)), 7)
	req.Header.Set("Content-Type", "application/json")
	h.HandleFeaturesBuildRoutes(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "errors")
	assert.Contains(t, body, "message")
}

func TestHTTPMutationRequiresAuth(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodDelete, "/api/features/42/build/buildings/1001", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTPCookieMutationRequiresCSRFHeader(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	req := requestWithUser(httptest.NewRequest(http.MethodDelete, "/api/features/42/build/buildings/1001", nil), 7)
	req.AddCookie(&http.Cookie{Name: "token", Value: "session"})
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	req = requestWithUser(httptest.NewRequest(http.MethodDelete, "/api/features/42/build/buildings/1001", nil), 7)
	req.AddCookie(&http.Cookie{Name: "token", Value: "session"})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPGRPCErrorMapping(t *testing.T) {
	building := &mockHTTPBuildingAPI{getBuildings: func(context.Context, *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
		return nil, status.Error(codes.NotFound, "building not found")
	}}
	h := handler.NewHTTPBuildingsHandler(building)
	w := httptest.NewRecorder()
	h.HandleFeaturesBuildRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42/build/buildings", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "building not found", body["error"])
}
