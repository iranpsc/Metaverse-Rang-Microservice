package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	featurespb "metargb/shared/pb/features"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newFeaturesHandler(t *testing.T, feature *testutil.MockFeatureService) *handler.FeaturesHandler {
	t.Helper()
	conn, cleanup := testutil.DialFeaturesConn(feature, nil)
	t.Cleanup(cleanup)
	return handler.NewFeaturesHandler(conn, conn, "en")
}

func TestListFeatures_ParsePointsFromQuery(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   []string
	}{
		{
			name:   "Laravel indexed points[0]..points[3]",
			target: "/api/features?points[0]=10,20&points[1]=30,20&points[2]=30,40&points[3]=10,40",
			want:   []string{"10,20", "30,20", "30,40", "10,40"},
		},
		{
			name:   "points[] repeated values",
			target: "/api/features?points[]=10,20&points[]=30,20&points[]=30,40&points[]=10,40",
			want:   []string{"10,20", "30,20", "30,40", "10,40"},
		},
		{
			name:   "JSON array in points",
			target: `/api/features?points=["10,20","30,20","30,40","10,40"]`,
			want:   []string{"10,20", "30,20", "30,40", "10,40"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			feature := &testutil.MockFeatureService{}
			feature.ListFeaturesFunc = func(_ context.Context, req *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error) {
				got = req.Points
				return &featurespb.FeaturesResponse{}, nil
			}
			h := newFeaturesHandler(t, feature)

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			w := httptest.NewRecorder()
			h.ListFeatures(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListFeatures_MissingPoints(t *testing.T) {
	h := newFeaturesHandler(t, &testutil.MockFeatureService{})

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	h.ListFeatures(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasErrors := body["errors"]
	assert.True(t, hasErrors)
}

func TestListFeatures_DoesNotSplitSinglePointsByComma(t *testing.T) {
	called := false
	feature := &testutil.MockFeatureService{}
	feature.ListFeaturesFunc = func(_ context.Context, _ *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error) {
		called = true
		return &featurespb.FeaturesResponse{}, nil
	}
	h := newFeaturesHandler(t, feature)

	req := httptest.NewRequest(http.MethodGet, "/api/features?points=10,20,30,20,30,40,10,40", nil)
	w := httptest.NewRecorder()
	h.ListFeatures(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.False(t, called)
}
