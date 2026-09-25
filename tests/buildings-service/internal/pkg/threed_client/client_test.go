package threed_client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/buildings-service/pkg/threed_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := threed_client.New("http://example.test")
	require.NotNil(t, c)
}

func TestGetBuildPackage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/build-package", r.URL.Path)
		assert.Equal(t, "9", r.URL.Query().Get("feature_id"))
		assert.Equal(t, "100", r.URL.Query().Get("area"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(threed_client.BuildPackageResponse{
			Data: []threed_client.BuildingModelData{{ID: 1, Name: "Tower", SKU: "sku-1"}},
		})
	}))
	defer srv.Close()

	c := threed_client.New(srv.URL)
	resp, err := c.GetBuildPackage(context.Background(), threed_client.BuildPackageRequest{
		FeatureID: 9, Area: "100", Density: "2", Karbari: "m", Page: 1,
	})
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "Tower", resp.Data[0].Name)
}

func TestGetBuildPackage_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := threed_client.New(srv.URL)
	_, err := c.GetBuildPackage(context.Background(), threed_client.BuildPackageRequest{FeatureID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3D Meta API returned error")
}

func TestGetBuildPackage_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer srv.Close()

	c := threed_client.New(srv.URL)
	_, err := c.GetBuildPackage(context.Background(), threed_client.BuildPackageRequest{FeatureID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestGetBuildPackage_ConnectionError(t *testing.T) {
	c := threed_client.New("http://127.0.0.1:1")
	_, err := c.GetBuildPackage(context.Background(), threed_client.BuildPackageRequest{FeatureID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to call 3D Meta API")
}
