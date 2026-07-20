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

	"metarang/grpc-gateway/internal/handler"
	"metarang/grpc-gateway/internal/testutil"
	featurespb "metarang/shared/pb/features"
)

func TestHandleFeaturesRoutes_UpdateBuildingMethods(t *testing.T) {
	var gotFeatureID uint64
	var gotModelID string
	building := &testutil.MockBuildingService{
		UpdateBuildingFunc: func(ctx context.Context, req *featurespb.UpdateBuildingRequest) (*featurespb.BuildingResponse, error) {
			gotFeatureID = req.FeatureId
			gotModelID = req.BuildingModelId
			return &featurespb.BuildingResponse{Success: true}, nil
		},
	}
	conn, cleanup := testutil.DialFeaturesConnWithBuilding(nil, nil, building)
	t.Cleanup(cleanup)
	h := handler.NewFeaturesHandler(conn, conn, "en")

	body := []byte(`{"launched_satisfaction":"50","rotation":"0","position":"1,2"}`)

	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{
			name:   "PUT reaches UpdateBuilding",
			method: http.MethodPut,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusOK,
		},
		{
			name:   "POST with _method=put reaches UpdateBuilding",
			method: http.MethodPost,
			url:    "/api/features/42/build/buildings/1001?_method=put",
			want:   http.StatusOK,
		},
		{
			name:   "PATCH does not reach UpdateBuilding",
			method: http.MethodPatch,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusUnprocessableEntity,
		},
		{
			name:   "plain POST still 404",
			method: http.MethodPost,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusNotFound,
		},
		{
			name:   "GET still 404",
			method: http.MethodGet,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFeatureID, gotModelID = 0, ""
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = testutil.RequestWithUser(req, 7)
			w := httptest.NewRecorder()
			h.HandleFeaturesRoutes(w, req)

			require.Equal(t, tt.want, w.Code, "body=%s", w.Body.String())
			if tt.want == http.StatusOK {
				assert.Equal(t, uint64(42), gotFeatureID)
				assert.Equal(t, "1001", gotModelID)
			}
			if tt.method == http.MethodPatch {
				assert.Equal(t, uint64(0), gotFeatureID)
			}
		})
	}
}

func TestHandleFeaturesRoutes_PatchBuildingInformation(t *testing.T) {
	var gotFeatureID uint64
	var gotModelID string
	var gotInfo *featurespb.BuildingInformation
	building := &testutil.MockBuildingService{
		UpdateBuildingInformationFunc: func(ctx context.Context, req *featurespb.UpdateBuildingInformationRequest) (*featurespb.UpdateBuildingInformationResponse, error) {
			gotFeatureID = req.FeatureId
			gotModelID = req.BuildingModelId
			gotInfo = req.Information
			return &featurespb.UpdateBuildingInformationResponse{
				Information: &featurespb.BuildingInformation{
					ActivityLine: "Retail",
					Name:         "Updated Store",
					Address:      "123 Main St",
				},
			}, nil
		},
	}
	conn, cleanup := testutil.DialFeaturesConnWithBuilding(nil, nil, building)
	t.Cleanup(cleanup)
	h := handler.NewFeaturesHandler(conn, conn, "en")

	body := []byte(`{"information":{"activity_line":"Retail","name":"Updated Store","address":"123 Main St"}}`)

	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{
			name:   "PATCH reaches UpdateBuildingInformation",
			method: http.MethodPatch,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusOK,
		},
		{
			name:   "POST with _method=patch reaches UpdateBuildingInformation",
			method: http.MethodPost,
			url:    "/api/features/42/build/buildings/1001?_method=patch",
			want:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFeatureID, gotModelID, gotInfo = 0, "", nil
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = testutil.RequestWithUser(req, 7)
			w := httptest.NewRecorder()
			h.HandleFeaturesRoutes(w, req)

			require.Equal(t, tt.want, w.Code, "body=%s", w.Body.String())
			assert.Equal(t, uint64(42), gotFeatureID)
			assert.Equal(t, "1001", gotModelID)
			require.NotNil(t, gotInfo)
			assert.Equal(t, "Retail", gotInfo.ActivityLine)
			assert.Equal(t, "Updated Store", gotInfo.Name)

			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			info, ok := resp["information"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "Updated Store", info["name"])
		})
	}
}

func TestHandleFeaturesRoutes_DestroyBuildingMethods(t *testing.T) {
	var gotFeatureID uint64
	var gotModelID string
	building := &testutil.MockBuildingService{
		DestroyBuildingFunc: func(ctx context.Context, req *featurespb.DestroyBuildingRequest) (*featurespb.BuildingResponse, error) {
			gotFeatureID = req.FeatureId
			gotModelID = req.BuildingModelId
			return &featurespb.BuildingResponse{Success: true}, nil
		},
	}
	conn, cleanup := testutil.DialFeaturesConnWithBuilding(nil, nil, building)
	t.Cleanup(cleanup)
	h := handler.NewFeaturesHandler(conn, conn, "en")

	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{
			name:   "DELETE reaches DestroyBuilding",
			method: http.MethodDelete,
			url:    "/api/features/42/build/buildings/1001",
			want:   http.StatusOK,
		},
		{
			name:   "POST with _method=delete reaches DestroyBuilding",
			method: http.MethodPost,
			url:    "/api/features/42/build/buildings/1001?_method=delete",
			want:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFeatureID, gotModelID = 0, ""
			req := testutil.RequestWithUser(httptest.NewRequest(tt.method, tt.url, nil), 7)
			w := httptest.NewRecorder()
			h.HandleFeaturesRoutes(w, req)

			require.Equal(t, tt.want, w.Code, "body=%s", w.Body.String())
			assert.Equal(t, uint64(42), gotFeatureID)
			assert.Equal(t, "1001", gotModelID)
		})
	}
}
