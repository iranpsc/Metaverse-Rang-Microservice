package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "metargb/shared/pb/auth"
)

func TestBuildCitizenReferralsHTTPResponse(t *testing.T) {
	t.Run("single data wrapper with Laravel pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/citizen/hm-2000001/referrals?page=1", nil)
		req.Host = "example.test"

		resp := &pb.CitizenReferralsResponse{
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
		}

		out := buildCitizenReferralsHTTPResponse(req, resp)

		data, ok := out["data"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, data, 1)
		assert.Equal(t, uint64(1), data[0]["id"])
		assert.Equal(t, "hm-2", data[0]["code"])
		orders, ok := data[0]["referrerOrders"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, orders, 1)

		links, ok := out["links"].(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, links["first"])
		assert.Nil(t, links["last"])
		assert.Nil(t, links["prev"])
		assert.NotNil(t, links["next"])

		meta, ok := out["meta"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, int32(1), meta["current_page"])
		assert.Equal(t, int32(10), meta["per_page"])
		assert.Equal(t, 1, meta["from"])
		assert.Equal(t, 1, meta["to"])
		assert.Contains(t, meta["path"], "/api/citizen/hm-2000001/referrals")

		_, hasNestedData := data[0]["data"]
		assert.False(t, hasNestedData)
	})
}

func TestBuildCitizenReferralChartHTTPResponse(t *testing.T) {
	t.Run("single data wrapper", func(t *testing.T) {
		resp := &pb.CitizenReferralChartResponse{
			Data: &pb.ReferralChartData{
				TotalReferralsCount:       "3",
				TotalReferralOrdersAmount: "1500",
				ChartData: []*pb.ChartDataPoint{
					{Label: "10:00", Count: 1, TotalAmount: 500},
				},
			},
		}

		out := buildCitizenReferralChartHTTPResponse(resp)

		payload, ok := out["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "3", payload["total_referrals_count"])
		assert.Equal(t, "1500", payload["total_referral_orders_amount"])

		chartData, ok := payload["chart_data"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, chartData, 1)
		assert.Equal(t, "10:00", chartData[0]["label"])

		_, hasNestedData := payload["data"]
		assert.False(t, hasNestedData)
	})
}

func TestWriteJSONSkipsDoubleWrapForCitizenReferrals(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/citizen/hm-2000001/referrals", nil)
	req.Host = "example.test"

	resp := &pb.CitizenReferralsResponse{
		Data: []*pb.CitizenReferral{{Id: 1, Code: "hm-2", Name: "User"}},
		Meta: &pb.PaginationMeta{CurrentPage: 1},
	}

	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, buildCitizenReferralsHTTPResponse(req, resp))

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
