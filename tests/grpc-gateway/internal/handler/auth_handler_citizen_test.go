package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "metargb/shared/pb/auth"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newCitizenAuthHandler(t *testing.T, citizen *testutil.MockCitizenService) *handler.AuthHandler {
	t.Helper()
	conn, cleanup := testutil.DialAuthConn(&testutil.AuthMocks{Citizen: citizen})
	t.Cleanup(cleanup)
	return handler.NewAuthHandler(conn, nil, "en")
}

func TestGetCitizenReferrals_LaravelPagination(t *testing.T) {
	citizen := &testutil.MockCitizenService{}
	citizen.GetCitizenReferralsFunc = func(_ context.Context, req *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error) {
		return &pb.CitizenReferralsResponse{
			Data: []*pb.CitizenReferral{
				{
					Id:   1,
					Code: "hm-2",
					Name: "Test User",
					ReferrerOrders: []*pb.ReferrerOrder{
						{Id: 10, Amount: 500, CreatedAt: "1403-01-01 12:00:00"},
					},
				},
			},
			Meta: &pb.PaginationMeta{
				CurrentPage: 1,
				NextPageUrl: "?page=2",
			},
		}, nil
	}
	h := newCitizenAuthHandler(t, citizen)

	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-2000001/referrals?page=1", nil)
	req.Host = "example.test"
	w := httptest.NewRecorder()
	h.HandleCitizenRoutes(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)

	item := data[0].(map[string]interface{})
	assert.EqualValues(t, 1, item["id"])
	assert.Equal(t, "hm-2", item["code"])
	orders, ok := item["referrerOrders"].([]interface{})
	require.True(t, ok, "referrerOrders must always be present")
	require.Len(t, orders, 1)

	links, ok := body["links"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, links["first"])
	assert.Nil(t, links["last"])
	assert.Nil(t, links["prev"])
	assert.NotNil(t, links["next"])

	meta, ok := body["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.EqualValues(t, 1, meta["current_page"])
	assert.EqualValues(t, 10, meta["per_page"])
	assert.EqualValues(t, 1, meta["from"])
	assert.EqualValues(t, 1, meta["to"])
	assert.Contains(t, meta["path"], "/api/citizen/hm-2000001/referrals")
}

func TestGetCitizenReferralChart_SingleDataWrapper(t *testing.T) {
	citizen := &testutil.MockCitizenService{}
	citizen.GetCitizenReferralChartFunc = func(_ context.Context, _ *pb.GetCitizenReferralChartRequest) (*pb.CitizenReferralChartResponse, error) {
		return &pb.CitizenReferralChartResponse{
			Data: &pb.ReferralChartData{
				TotalReferralsCount:       "3",
				TotalReferralOrdersAmount: "1500",
				ChartData: []*pb.ChartDataPoint{
					{Label: "10:00", Count: 1, TotalAmount: 500},
				},
			},
		}, nil
	}
	h := newCitizenAuthHandler(t, citizen)

	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/referrals/chart", nil)
	w := httptest.NewRecorder()
	h.HandleCitizenRoutes(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	payload, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "3", payload["total_referrals_count"])
	assert.Equal(t, "1500", payload["total_referral_orders_amount"])

	chartData, ok := payload["chart_data"].([]interface{})
	require.True(t, ok)
	require.Len(t, chartData, 1)
	assert.Equal(t, "10:00", chartData[0].(map[string]interface{})["label"])
}

func TestGetCitizenReferrals_EmptyReferrerOrders(t *testing.T) {
	citizen := &testutil.MockCitizenService{}
	citizen.GetCitizenReferralsFunc = func(_ context.Context, _ *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error) {
		return &pb.CitizenReferralsResponse{
			Data: []*pb.CitizenReferral{
				{Id: 1, Code: "hm-2", Name: "User", ReferrerOrders: []*pb.ReferrerOrder{}},
			},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}
	h := newCitizenAuthHandler(t, citizen)

	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-2000001/referrals?page=1", nil)
	w := httptest.NewRecorder()
	h.HandleCitizenRoutes(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	orders, ok := data[0].(map[string]interface{})["referrerOrders"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, orders)
}

func TestGetCitizenReferrals_NoDoubleWrap(t *testing.T) {
	citizen := &testutil.MockCitizenService{}
	citizen.GetCitizenReferralsFunc = func(_ context.Context, _ *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error) {
		return &pb.CitizenReferralsResponse{
			Data: []*pb.CitizenReferral{{Id: 1, Code: "hm-2", Name: "User"}},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}
	h := newCitizenAuthHandler(t, citizen)

	req := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-2000001/referrals", nil)
	req.Host = "example.test"
	w := httptest.NewRecorder()
	h.HandleCitizenRoutes(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	_, hasTopLevelData := body["data"]
	assert.True(t, hasTopLevelData)
	_, hasLinks := body["links"]
	assert.True(t, hasLinks)
	_, hasMeta := body["meta"]
	assert.True(t, hasMeta)

	if inner, ok := body["data"].(map[string]interface{}); ok {
		_, hasDoubleData := inner["data"]
		assert.False(t, hasDoubleData, "response must not double-wrap data")
	}
}
