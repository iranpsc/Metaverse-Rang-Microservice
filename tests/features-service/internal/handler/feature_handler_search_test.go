package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/features-service/internal/handler"
	pb "metarang/shared/pb/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFeatureHandler_SearchFeatures(t *testing.T) {
	t.Run("empty term skips the service", func(t *testing.T) {
		called := false
		h := handler.NewFeatureHandler(&mockFeaturePort{
			searchFeatures: func(context.Context, string) ([]*pb.SearchFeatureResult, error) {
				called = true
				return nil, errors.New("should not be called")
			},
		}, nil)

		resp, err := h.SearchFeatures(context.Background(), &pb.SearchFeaturesRequest{SearchTerm: "  "})
		require.NoError(t, err)
		assert.Empty(t, resp.Data)
		assert.False(t, called)
	})

	t.Run("maps service results", func(t *testing.T) {
		h := handler.NewFeatureHandler(&mockFeaturePort{
			searchFeatures: func(_ context.Context, term string) ([]*pb.SearchFeatureResult, error) {
				assert.Equal(t, "TEH-", term)
				return []*pb.SearchFeatureResult{{
					Id:                  7,
					FeaturePropertiesId: "TEH-1",
					Address:             "Tehran",
					Karbari:             "مسکونی",
					PricePsc:            "2.5",
					PriceIrr:            "3500",
					OwnerCode:           "HM-1",
					Coordinates:         []*pb.SearchFeatureCoordinate{{Id: 11, X: 51.1, Y: 35.6}},
					LatestSellRequest:   &pb.SellRequestResponse{Id: 8, FeatureId: 7, SellerId: 2, Status: 0, PricePsc: "10.5000000000", PriceIrr: "20.0000000000"},
				}}, nil
			},
		}, nil)

		resp, err := h.SearchFeatures(context.Background(), &pb.SearchFeaturesRequest{SearchTerm: "TEH-"})
		require.NoError(t, err)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, uint64(8), resp.Data[0].LatestSellRequest.Id)
		assert.Equal(t, int32(0), resp.Data[0].LatestSellRequest.Status)
	})

	t.Run("service error", func(t *testing.T) {
		h := handler.NewFeatureHandler(&mockFeaturePort{
			searchFeatures: func(context.Context, string) ([]*pb.SearchFeatureResult, error) {
				return nil, errors.New("db down")
			},
		}, nil)
		resp, err := h.SearchFeatures(context.Background(), &pb.SearchFeaturesRequest{SearchTerm: "x"})
		require.Error(t, err)
		assert.Nil(t, resp)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestHTTPFeaturesHandler_SearchFeatures(t *testing.T) {
	t.Run("json body includes latest pending sell request", func(t *testing.T) {
		api := &mockHTTPFeatureAPI{
			searchFeatures: func(_ context.Context, req *pb.SearchFeaturesRequest) (*pb.SearchFeaturesResponse, error) {
				assert.Equal(t, "TEH-", req.SearchTerm)
				return &pb.SearchFeaturesResponse{Data: []*pb.SearchFeatureResult{{
					Id:                  7,
					FeaturePropertiesId: "TEH-1",
					Address:             "Tehran",
					Karbari:             "مسکونی",
					PricePsc:            "2.5",
					PriceIrr:            "3500",
					OwnerCode:           "HM-1",
					Coordinates:         []*pb.SearchFeatureCoordinate{{Id: 11, X: 51.1, Y: 35.6}},
					LatestSellRequest: &pb.SellRequestResponse{
						Id: 8, FeatureId: 7, SellerId: 2, Status: 0,
						PricePsc: "10.5000000000", PriceIrr: "20.0000000000", CreatedAt: "1405/01/01",
					},
				}}}, nil
			},
		}
		body := bytes.NewBufferString(`{"searchTerm":"TEH-"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/search/features", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		newHTTPFeaturesHandler(api).SearchFeatures(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		var payload struct {
			Data []map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
		require.Len(t, payload.Data, 1)
		item := payload.Data[0]
		assert.Equal(t, "TEH-1", item["feature_properties_id"])
		assert.Equal(t, "مسکونی", item["karbari"])
		assert.Equal(t, "HM-1", item["owner_code"])
		coords, ok := item["coordinates"].([]interface{})
		require.True(t, ok)
		require.Len(t, coords, 1)
		sell, ok := item["latest_sell_request"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(8), sell["id"])
		assert.Equal(t, float64(7), sell["feature_id"])
		assert.Equal(t, float64(2), sell["seller_id"])
		assert.Equal(t, float64(0), sell["status"])
		assert.Equal(t, "10.5000000000", sell["price_psc"])
		assert.Equal(t, "1405/01/01", sell["created_at"])
	})

	t.Run("latest sell request is null when none is pending", func(t *testing.T) {
		api := &mockHTTPFeatureAPI{
			searchFeatures: func(context.Context, *pb.SearchFeaturesRequest) (*pb.SearchFeaturesResponse, error) {
				return &pb.SearchFeaturesResponse{Data: []*pb.SearchFeatureResult{{
					Id: 1, FeaturePropertiesId: "A", Coordinates: []*pb.SearchFeatureCoordinate{},
				}}}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/api/search/features?searchTerm=A", nil)
		rr := httptest.NewRecorder()
		newHTTPFeaturesHandler(api).SearchFeatures(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"latest_sell_request":null`)
	})

	t.Run("invalid json body", func(t *testing.T) {
		body := bytes.NewBufferString(`{`)
		req := httptest.NewRequest(http.MethodPost, "/api/search/features", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}).SearchFeatures(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
