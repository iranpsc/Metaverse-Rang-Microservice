package handler

import (
	"testing"

	featurespb "metargb/shared/pb/features"
)

func TestFormatHourlyProfitResource(t *testing.T) {
	profit := &featurespb.HourlyProfit{
		Id:           1,
		FeatureId:    999,
		UserId:       42,
		FeatureDbId:  100,
		PropertiesId: "abc123",
		Karbari:      "m",
		Amount:       "12.345",
		DeadLine:     "1403/01/15",
		IsActive:     true,
	}

	got := formatHourlyProfitResource(profit)

	if got["feature_id"] != "abc123" {
		t.Errorf("feature_id should be properties id string, got %v", got["feature_id"])
	}
	if got["feature_db_id"] != uint64(100) {
		t.Errorf("feature_db_id = %v, want 100", got["feature_db_id"])
	}
	if got["karbari"] != "m" {
		t.Errorf("karbari = %v, want m", got["karbari"])
	}
	if got["user_id"] != uint64(42) {
		t.Errorf("user_id = %v, want 42", got["user_id"])
	}
}
