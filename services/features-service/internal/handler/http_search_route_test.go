package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	featurespb "metarang/shared/pb/features"

	"google.golang.org/protobuf/types/known/emptypb"
)

type searchRouteAPI struct{}

func (searchRouteAPI) ListFeatures(context.Context, *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error) {
	return &featurespb.FeaturesResponse{}, nil
}
func (searchRouteAPI) GetFeature(context.Context, *featurespb.GetFeatureRequest) (*featurespb.FeatureResponse, error) {
	return &featurespb.FeatureResponse{}, nil
}
func (searchRouteAPI) ListMyFeatures(context.Context, *featurespb.ListMyFeaturesRequest) (*featurespb.ListMyFeaturesResponse, error) {
	return &featurespb.ListMyFeaturesResponse{}, nil
}
func (searchRouteAPI) GetMyFeature(context.Context, *featurespb.GetMyFeatureRequest) (*featurespb.FeatureResponse, error) {
	return &featurespb.FeatureResponse{}, nil
}
func (searchRouteAPI) AddMyFeatureImages(context.Context, *featurespb.AddMyFeatureImagesRequest) (*featurespb.FeatureResponse, error) {
	return &featurespb.FeatureResponse{}, nil
}
func (searchRouteAPI) RemoveMyFeatureImage(context.Context, *featurespb.RemoveMyFeatureImageRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (searchRouteAPI) UpdateMyFeature(context.Context, *featurespb.UpdateMyFeatureRequest) (*featurespb.UpdateMyFeatureResponse, error) {
	return &featurespb.UpdateMyFeatureResponse{}, nil
}
func (searchRouteAPI) GetFeatureTradeHistory(context.Context, *featurespb.GetFeatureTradeHistoryRequest) (*featurespb.GetFeatureTradeHistoryResponse, error) {
	return &featurespb.GetFeatureTradeHistoryResponse{}, nil
}
func (searchRouteAPI) SearchFeatures(_ context.Context, req *featurespb.SearchFeaturesRequest) (*featurespb.SearchFeaturesResponse, error) {
	return &featurespb.SearchFeaturesResponse{Data: []*featurespb.SearchFeatureResult{{
		Id: 1, FeaturePropertiesId: req.SearchTerm,
	}}}, nil
}

func TestPublicMux_SearchFeatures(t *testing.T) {
	h := newPublicHTTPHandler(HTTPServerHandlers{
		Features: NewHTTPFeaturesHandler(searchRouteAPI{}, nil, nil),
	}, nil, nil, nil)

	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"post json", http.MethodPost, "/api/search/features", `{"searchTerm":"teh"}`},
		{"get query", http.MethodGet, "/api/search/features?searchTerm=teh", ""},
		{"post slash", http.MethodPost, "/api/search/features/", `{"searchTerm":"teh"}`},
		{"get slash", http.MethodGet, "/api/search/features/", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body *strings.Reader
			if tc.body == "" {
				body = strings.NewReader("")
			} else {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			t.Logf("status=%d body=%s", rr.Code, rr.Body.String())
			if rr.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}
