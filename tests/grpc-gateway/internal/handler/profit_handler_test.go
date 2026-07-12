package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	featurespb "metargb/shared/pb/features"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func TestGetSingleProfit_LaravelResourceShape(t *testing.T) {
	profit := &testutil.MockFeatureProfitService{}
	profit.GetSingleProfitFunc = func(_ context.Context, _ *featurespb.GetSingleProfitRequest) (*featurespb.HourlyProfitResponse, error) {
		return &featurespb.HourlyProfitResponse{
			Profit: &featurespb.HourlyProfit{
				Id:           1,
				FeatureId:    999,
				UserId:       42,
				FeatureDbId:  100,
				PropertiesId: "abc123",
				Karbari:      "m",
				Amount:       "12.345",
				DeadLine:     "1403/01/15",
				IsActive:     true,
			},
		}, nil
	}
	conn, cleanup := testutil.DialFeaturesConn(nil, profit)
	defer cleanup()
	h := handler.NewProfitHandler(conn, conn)

	req := httptest.NewRequest(http.MethodPost, "/api/hourly-profits/1", nil)
	req = testutil.RequestWithUser(req, 42)
	w := httptest.NewRecorder()
	h.GetSingleProfit(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := body["data"].(map[string]interface{})

	if got["feature_id"] != "abc123" {
		t.Errorf("feature_id should be properties id string, got %v", got["feature_id"])
	}
	if got["feature_db_id"] != float64(100) {
		t.Errorf("feature_db_id = %v, want 100", got["feature_db_id"])
	}
	if got["karbari"] != "m" {
		t.Errorf("karbari = %v, want m", got["karbari"])
	}
	if got["user_id"] != float64(42) {
		t.Errorf("user_id = %v, want 42", got["user_id"])
	}
}
