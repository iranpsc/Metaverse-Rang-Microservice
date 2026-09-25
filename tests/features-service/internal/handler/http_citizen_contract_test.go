package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/features-service/internal/handler"
	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
	"metarang/shared/pkg/period"
)

var (
	// 2026-05-15 → Jalali 1405; 2024-03-01 (before Nowruz) → Jalali 1402
	citizenContractRef          = time.Date(2026, 5, 15, 12, 0, 0, 0, time.Local)
	citizenContractRegisteredAt = time.Date(2024, 3, 1, 0, 0, 0, 0, time.Local)
)

type mockCitizenAuthClient struct {
	authpb.CitizenServiceClient
	userInfo func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error)
}

func (m *mockCitizenAuthClient) GetCitizenProfile(context.Context, *authpb.GetCitizenProfileRequest, ...grpc.CallOption) (*authpb.CitizenProfileResponse, error) {
	return nil, nil
}
func (m *mockCitizenAuthClient) GetCitizenReferrals(context.Context, *authpb.GetCitizenReferralsRequest, ...grpc.CallOption) (*authpb.CitizenReferralsResponse, error) {
	return nil, nil
}
func (m *mockCitizenAuthClient) GetCitizenReferralChart(context.Context, *authpb.GetCitizenReferralChartRequest, ...grpc.CallOption) (*authpb.CitizenReferralChartResponse, error) {
	return nil, nil
}
func (m *mockCitizenAuthClient) GetCitizenUserInfo(ctx context.Context, req *authpb.GetCitizenUserInfoRequest, _ ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
	if m.userInfo != nil {
		return m.userInfo(ctx, req)
	}
	return &authpb.GetCitizenUserInfoResponse{UserId: 42}, nil
}

type mockCitizenFeaturesHTTPAPI struct {
	summary func(context.Context, *featurespb.GetCitizenFeatureSummaryRequest) (*featurespb.GetCitizenFeatureSummaryResponse, error)
	chart   func(context.Context, *featurespb.GetCitizenFeatureChartRequest) (*featurespb.GetCitizenFeatureChartResponse, error)
	list    func(context.Context, *featurespb.ListCitizenFeaturesRequest) (*featurespb.ListCitizenFeaturesResponse, error)
}

func (m *mockCitizenFeaturesHTTPAPI) GetCitizenFeatureSummary(ctx context.Context, req *featurespb.GetCitizenFeatureSummaryRequest) (*featurespb.GetCitizenFeatureSummaryResponse, error) {
	if m.summary != nil {
		return m.summary(ctx, req)
	}
	return &featurespb.GetCitizenFeatureSummaryResponse{}, nil
}
func (m *mockCitizenFeaturesHTTPAPI) GetCitizenFeatureChart(ctx context.Context, req *featurespb.GetCitizenFeatureChartRequest) (*featurespb.GetCitizenFeatureChartResponse, error) {
	if m.chart != nil {
		return m.chart(ctx, req)
	}
	return &featurespb.GetCitizenFeatureChartResponse{}, nil
}
func (m *mockCitizenFeaturesHTTPAPI) ListCitizenFeatures(ctx context.Context, req *featurespb.ListCitizenFeaturesRequest) (*featurespb.ListCitizenFeaturesResponse, error) {
	if m.list != nil {
		return m.list(ctx, req)
	}
	return &featurespb.ListCitizenFeaturesResponse{}, nil
}

func featureChartResponseForPeriod(periodValue string) *featurespb.GetCitizenFeatureChartResponse {
	window, err := period.ResolvePeriod(periodValue, citizenContractRef, citizenContractRegisteredAt)
	if err != nil {
		panic(err)
	}
	bought := make([]*featurespb.CitizenChartPoint, len(window.Buckets))
	sold := make([]*featurespb.CitizenChartPoint, len(window.Buckets))
	for i, bucket := range window.Buckets {
		bought[i] = &featurespb.CitizenChartPoint{Karbari: "t", Label: bucket.Label}
		sold[i] = &featurespb.CitizenChartPoint{Karbari: "t", Label: bucket.Label}
	}
	return &featurespb.GetCitizenFeatureChartResponse{
		Data:   &featurespb.CitizenFeatureChartData{Bought: bought, Sold: sold},
		Period: periodValue,
	}
}

func newCitizenHTTPHandlers(t *testing.T, features *mockCitizenFeaturesHTTPAPI) *handler.HTTPCitizenFeaturesHandler {
	t.Helper()
	citizen := &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return &authpb.GetCitizenUserInfoResponse{UserId: 42, Privacy: map[string]int32{}}, nil
		},
	}
	if features == nil {
		features = &mockCitizenFeaturesHTTPAPI{}
	}
	return handler.NewHTTPCitizenFeaturesHandler(features, citizen)
}

func expectedYearlyBucketCount(t *testing.T) int {
	t.Helper()
	window, err := period.ResolvePeriod("yearly", citizenContractRef, citizenContractRegisteredAt)
	require.NoError(t, err)
	require.Equal(t, []string{"1402", "1403", "1404", "1405"}, yearlyLabels(window))
	return len(window.Buckets)
}

func yearlyLabels(window *period.PeriodWindow) []string {
	labels := make([]string, len(window.Buckets))
	for i, bucket := range window.Buckets {
		labels[i] = bucket.Label
	}
	return labels
}

func TestHTTPCitizenFeaturesSummary_PeriodInResponse(t *testing.T) {
	for _, test := range []struct {
		name, query, wantPeriod string
	}{
		{"weekly", "period=weekly", "weekly"},
		{"monthly", "period=monthly", "monthly"},
		{"yearly", "period=yearly", "yearly"},
		{"missing defaults daily", "", "daily"},
		{"invalid defaults daily", "period=bogus", "daily"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotPeriod string
			features := &mockCitizenFeaturesHTTPAPI{
				summary: func(_ context.Context, req *featurespb.GetCitizenFeatureSummaryRequest) (*featurespb.GetCitizenFeatureSummaryResponse, error) {
					gotPeriod = req.Period
					return &featurespb.GetCitizenFeatureSummaryResponse{
						Data:   []*featurespb.CitizenFeatureSummaryItem{{Karbari: "t", Label: "تجاری"}},
						Period: req.Period,
					}, nil
				},
			}
			featuresHandler := newCitizenHTTPHandlers(t, features)

			target := "/api/citizen/hm-1/features/summary"
			if test.query != "" {
				target += "?" + test.query
			}
			w := httptest.NewRecorder()
			featuresHandler.Handle(w, httptest.NewRequest(http.MethodGet, target, nil), "hm-1", []string{"summary"})

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.wantPeriod, gotPeriod)

			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, test.wantPeriod, body["period"])
			_, hasData := body["data"]
			assert.True(t, hasData)
		})
	}
}

func TestHTTPCitizenFeaturesChart_PeriodAndBucketCounts(t *testing.T) {
	yearlyBuckets := expectedYearlyBucketCount(t)

	for _, test := range []struct {
		name, query, wantPeriod string
		wantBuckets             int
	}{
		{"daily", "period=daily", "daily", 24},
		{"weekly", "period=weekly", "weekly", 4},
		{"monthly", "period=monthly", "monthly", 12},
		{"yearly", "period=yearly", "yearly", yearlyBuckets},
		{"missing defaults daily", "", "daily", 24},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotPeriod string
			features := &mockCitizenFeaturesHTTPAPI{
				chart: func(_ context.Context, req *featurespb.GetCitizenFeatureChartRequest) (*featurespb.GetCitizenFeatureChartResponse, error) {
					gotPeriod = req.Period
					return featureChartResponseForPeriod(req.Period), nil
				},
			}
			featuresHandler := newCitizenHTTPHandlers(t, features)

			target := "/api/citizen/hm-1/features/chart"
			if test.query != "" {
				target += "?" + test.query
			}
			w := httptest.NewRecorder()
			featuresHandler.Handle(w, httptest.NewRequest(http.MethodGet, target, nil), "hm-1", []string{"chart"})

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.wantPeriod, gotPeriod)

			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			_, hasPeriod := body["period"]
			assert.False(t, hasPeriod)

			data := body["data"].(map[string]interface{})
			bought := data["bought"].([]interface{})
			sold := data["sold"].([]interface{})
			assert.Len(t, bought, test.wantBuckets)
			assert.Len(t, sold, test.wantBuckets)
			firstPoint := bought[0].(map[string]interface{})
			_, hasKarbari := firstPoint["karbari"]
			_, hasLabel := firstPoint["label"]
			_, hasAmount := firstPoint["amount"]
			assert.True(t, hasKarbari)
			assert.True(t, hasLabel)
			assert.True(t, hasAmount)
		})
	}
}




func TestHTTPCitizenFeaturesList_PaginationPrivacyAndErrors(t *testing.T) {
	from, to := int32(16), int32(30)
	features := &mockCitizenFeaturesHTTPAPI{
		list: func(_ context.Context, req *featurespb.ListCitizenFeaturesRequest) (*featurespb.ListCitizenFeaturesResponse, error) {
			assert.Equal(t, []string{"m"}, req.AllowedKarbaris)
			assert.Equal(t, int32(2), req.Page)
			assert.Equal(t, int32(15), req.PerPage)
			return &featurespb.ListCitizenFeaturesResponse{
				Data: []*featurespb.CitizenFeatureItem{{
					Id: 9, VodId: "v", Address: "a", Area: 10, Density: 1, Karbari: "m",
					OwnerCode: "hm-1", PricePsc: "12.5", PriceIrr: "3", Label: "l",
					Center: &featurespb.CitizenFeatureCenter{X: 1, Y: 2},
					Images: []*featurespb.Image{{Id: 4, Url: "https://cdn.example/f.jpg"}, nil},
				}},
				MapMarkers: []*featurespb.CitizenFeatureMapMarker{
					{Id: 9, Karbari: "m", Center: &featurespb.CitizenFeatureCenter{X: 1, Y: 2}},
					{Id: 8, Karbari: "m"},
				},
				Meta: &featurespb.FeatureTradeHistoryPaginationMeta{
					CurrentPage: 2, LastPage: 3, PerPage: 15, Total: 40, From: &from, To: &to,
				},
			}, nil
		},
	}
	citizen := &mockCitizenAuthClient{
		userInfo: func(_ context.Context, _ *authpb.GetCitizenUserInfoRequest, _ ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return &authpb.GetCitizenUserInfoResponse{
				UserId: 42,
				Privacy: map[string]int32{
					"maskoni_features": 1,
					"tejari_features":  0,
				},
			}, nil
		},
	}
	h := handler.NewHTTPCitizenFeaturesHandler(features, citizen)
	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/features?karbari=m&karbari=t&karbari=zz&page=2&per_page=15&search=q", nil)
	w := httptest.NewRecorder()
	h.Handle(w, req, "hm-1", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body["data"])
	assert.NotEmpty(t, body["map_markers"])
	links := body["links"].(map[string]interface{})
	assert.Contains(t, links["prev"].(string), "page=1")
	assert.Contains(t, links["next"].(string), "page=3")

	nilCitizen := handler.NewHTTPCitizenFeaturesHandler(features, nil)
	w = httptest.NewRecorder()
	nilCitizen.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/features", nil), "hm-1", nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	notFound := handler.NewHTTPCitizenFeaturesHandler(features, &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return nil, status.Error(codes.NotFound, "missing")
		},
	})
	w = httptest.NewRecorder()
	notFound.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/features", nil), "hm-1", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	otherErr := handler.NewHTTPCitizenFeaturesHandler(features, &mockCitizenAuthClient{
		userInfo: func(context.Context, *authpb.GetCitizenUserInfoRequest, ...grpc.CallOption) (*authpb.GetCitizenUserInfoResponse, error) {
			return nil, status.Error(codes.Unavailable, "down")
		},
	})
	w = httptest.NewRecorder()
	otherErr.Handle(w, httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/features", nil), "hm-1", nil)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

