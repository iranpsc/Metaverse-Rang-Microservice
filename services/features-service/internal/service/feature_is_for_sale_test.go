package service

import (
	"testing"

	"metarang/features-service/internal/models"
)

func TestIsForSaleFromLatestSellRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *models.SellFeatureRequest
		want int32
	}{
		{name: "no sell request is not for sale", req: nil, want: 0},
		{name: "open sell request status 0 is for sale", req: &models.SellFeatureRequest{Status: 0}, want: 1},
		{name: "completed sell request status 1 is not for sale", req: &models.SellFeatureRequest{Status: 1}, want: 0},
		{name: "unknown sell request status is not for sale", req: &models.SellFeatureRequest{Status: 2}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isForSaleFromLatestSellRequest(tt.req); got != tt.want {
				t.Fatalf("isForSaleFromLatestSellRequest() = %d, want %d", got, tt.want)
			}
		})
	}
}
