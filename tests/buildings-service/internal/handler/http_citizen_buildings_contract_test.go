package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/buildings-service/internal/handler"
	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
)

type mockCitizenAuthClient struct {
	authpb.CitizenServiceClient
	userInfo func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error)
}

func (m *mockCitizenAuthClient) GetCitizenUserInfo(ctx context.Context, req *authpb.GetCitizenUserInfoRequest, _ ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
	if m.userInfo != nil {
		return m.userInfo(ctx, req)
	}
	return &authpb.GetCitizenUserInfoResponse{UserId: 42}, nil
}

type mockCitizenBuildingsHTTPAPI struct {
	summary func(context.Context, *featurespb.GetCitizenBuildingSummaryRequest) (*featurespb.GetCitizenBuildingSummaryResponse, error)
	chart   func(context.Context, *featurespb.GetCitizenBuildingChartRequest) (*featurespb.GetCitizenBuildingChartResponse, error)
	list    func(context.Context, *featurespb.ListCitizenBuildingsRequest) (*featurespb.ListCitizenBuildingsResponse, error)
}

func (m *mockCitizenBuildingsHTTPAPI) GetCitizenBuildingSummary(ctx context.Context, req *featurespb.GetCitizenBuildingSummaryRequest) (*featurespb.GetCitizenBuildingSummaryResponse, error) {
	if m.summary != nil {
		return m.summary(ctx, req)
	}
	return &featurespb.GetCitizenBuildingSummaryResponse{}, nil
}
func (m *mockCitizenBuildingsHTTPAPI) GetCitizenBuildingChart(ctx context.Context, req *featurespb.GetCitizenBuildingChartRequest) (*featurespb.GetCitizenBuildingChartResponse, error) {
	if m.chart != nil {
		return m.chart(ctx, req)
	}
	return &featurespb.GetCitizenBuildingChartResponse{}, nil
}
func (m *mockCitizenBuildingsHTTPAPI) ListCitizenBuildings(ctx context.Context, req *featurespb.ListCitizenBuildingsRequest) (*featurespb.ListCitizenBuildingsResponse, error) {
	if m.list != nil {
		return m.list(ctx, req)
	}
	return &featurespb.ListCitizenBuildingsResponse{}, nil
}

func newCitizenBuildingsHTTP(t *testing.T, api *mockCitizenBuildingsHTTPAPI) *handler.HTTPCitizenBuildingsHandler {
	t.Helper()
	citizen := &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return &authpb.GetCitizenUserInfoResponse{UserId: 42, Privacy: map[string]int32{}}, nil
		},
	}
	if api == nil {
		api = &mockCitizenBuildingsHTTPAPI{}
	}
	return handler.NewHTTPCitizenBuildingsHandler(api, citizen)
}

func TestHTTPCitizenBuildingsSummary_ShapeAndKarbaris(t *testing.T) {
	var got []string
	api := &mockCitizenBuildingsHTTPAPI{
		summary: func(_ context.Context, req *featurespb.GetCitizenBuildingSummaryRequest) (*featurespb.GetCitizenBuildingSummaryResponse, error) {
			assert.Equal(t, uint64(42), req.UserId)
			got = req.AllowedKarbaris
			return &featurespb.GetCitizenBuildingSummaryResponse{
				Data: []*featurespb.CitizenBuildingSummaryItem{{Karbari: "m", Label: "مسکونی", Count: 5}},
			}, nil
		},
	}
	h := newCitizenBuildingsHTTP(t, api)
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings/summary?karbari=m", nil), "hm-1", []string{"summary"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"m"}, got)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasPeriod := body["period"]
	assert.False(t, hasPeriod)
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	assert.Equal(t, "m", item["karbari"])
	assert.Equal(t, "مسکونی", item["label"])
	assert.Equal(t, float64(5), item["count"])
}

func TestHTTPCitizenBuildingsChart_PeriodAndShape(t *testing.T) {
	for _, test := range []struct {
		name, query, wantPeriod string
	}{
		{"daily", "period=daily", "daily"},
		{"weekly", "period=weekly", "weekly"},
		{"monthly", "period=monthly", "monthly"},
		{"yearly", "period=yearly", "yearly"},
		{"missing defaults daily", "", "daily"},
		{"invalid defaults daily", "period=bogus", "daily"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotPeriod string
			api := &mockCitizenBuildingsHTTPAPI{
				chart: func(_ context.Context, req *featurespb.GetCitizenBuildingChartRequest) (*featurespb.GetCitizenBuildingChartResponse, error) {
					gotPeriod = req.Period
					return &featurespb.GetCitizenBuildingChartResponse{
						Data: &featurespb.CitizenBuildingChartData{
							Completed: []*featurespb.CitizenChartPoint{{Karbari: "t", Label: "bucket", Amount: 2}},
						},
						Period: req.Period,
					}, nil
				},
			}
			h := newCitizenBuildingsHTTP(t, api)
			target := "/api/citizen/hm-1/buildings/chart"
			if test.query != "" {
				target += "?" + test.query
			}
			w := httptest.NewRecorder()
			h.Handle(w, httptest.NewRequest(http.MethodGet, target, nil), "hm-1", []string{"chart"})
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.wantPeriod, gotPeriod)

			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			_, hasPeriod := body["period"]
			assert.False(t, hasPeriod)
			data := body["data"].([]interface{})
			require.Len(t, data, 1)
			point := data[0].(map[string]interface{})
			assert.Equal(t, "t", point["karbari"])
			assert.Equal(t, "bucket", point["label"])
			assert.Equal(t, float64(2), point["amount"])
		})
	}
}

func TestHTTPCitizenBuildingsList_PaginationPrivacyAndErrors(t *testing.T) {
	from, to := int32(11), int32(20)
	area := 85.0
	api := &mockCitizenBuildingsHTTPAPI{
		list: func(_ context.Context, req *featurespb.ListCitizenBuildingsRequest) (*featurespb.ListCitizenBuildingsResponse, error) {
			assert.Equal(t, []string{"m"}, req.AllowedKarbaris)
			assert.Equal(t, int32(2), req.Page)
			return &featurespb.ListCitizenBuildingsResponse{
				Data: []*featurespb.CitizenBuildingItem{{
					BuildingId: "sku-residential-001", Karbari: "m", Area: &area,
					Images: []*featurespb.Image{{Id: 11, Url: "https://cdn.example/a.jpg"}, nil},
				}},
				Meta: &featurespb.FeatureTradeHistoryPaginationMeta{
					CurrentPage: 2, LastPage: 3, PerPage: 10, Total: 25, From: &from, To: &to,
				},
			}, nil
		},
	}
	citizen := &mockCitizenAuthClient{
		userInfo: func(_ context.Context, req *authpb.GetCitizenUserInfoRequest, _ ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			assert.Equal(t, "hm-1", req.Code)
			return &authpb.GetCitizenUserInfoResponse{
				UserId: 42,
				Privacy: map[string]int32{
					"maskoni_features": 1,
					"tejari_features":  0,
				},
			}, nil
		},
	}
	h := handler.NewHTTPCitizenBuildingsHandler(api, citizen)
	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings?karbari=m&karbari=t&karbari=zz&page=2", nil)
	w := httptest.NewRecorder()
	h.Handle(w, req, "hm-1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	assert.Equal(t, "sku-residential-001", item["building_id"])
	assert.Equal(t, "m", item["karbari"])
	assert.Equal(t, 85.0, item["area"])
	images := item["images"].([]interface{})
	require.Len(t, images, 1)
	assert.Equal(t, "https://cdn.example/a.jpg", images[0].(map[string]interface{})["url"])
	links := body["links"].(map[string]interface{})
	assert.Contains(t, links["prev"].(string), "page=1")
	assert.Contains(t, links["next"].(string), "page=3")
	assert.Contains(t, links["next"].(string), "karbari=m")
	meta := body["meta"].(map[string]interface{})
	assert.Equal(t, float64(25), meta["total"])

	nilCitizen := handler.NewHTTPCitizenBuildingsHandler(api, nil)
	w = httptest.NewRecorder()
	nilCitizen.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings", nil), "hm-1", nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	notFound := handler.NewHTTPCitizenBuildingsHandler(api, &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return nil, status.Error(codes.NotFound, "missing")
		},
	})
	w = httptest.NewRecorder()
	notFound.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings", nil), "hm-1", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	otherErr := handler.NewHTTPCitizenBuildingsHandler(api, &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return nil, status.Error(codes.Unavailable, "down")
		},
	})
	w = httptest.NewRecorder()
	otherErr.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings", nil), "hm-1", nil)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHTTPCitizenBuildingsIndexedKarbariQuery(t *testing.T) {
	var got []string
	api := &mockCitizenBuildingsHTTPAPI{
		list: func(_ context.Context, req *featurespb.ListCitizenBuildingsRequest) (*featurespb.ListCitizenBuildingsResponse, error) {
			got = req.AllowedKarbaris
			return &featurespb.ListCitizenBuildingsResponse{}, nil
		},
	}
	h := newCitizenBuildingsHTTP(t, api)
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/buildings?karbari[0]=m&karbari[1]=a", nil), "hm-1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"m", "a"}, got)
}
