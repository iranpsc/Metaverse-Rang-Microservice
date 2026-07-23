package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/features-service/internal/handler"
	authpkg "metarang/shared/pkg/auth"
	pb "metarang/shared/pb/features"
)

type mockHTTPFeatureAPI struct {
	listFeatures     func(context.Context, *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error)
	tradeHistory     func(context.Context, *pb.GetFeatureTradeHistoryRequest) (*pb.GetFeatureTradeHistoryResponse, error)
}

func (m *mockHTTPFeatureAPI) ListFeatures(ctx context.Context, req *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error) {
	if m.listFeatures != nil {
		return m.listFeatures(ctx, req)
	}
	return &pb.FeaturesResponse{}, nil
}
func (m *mockHTTPFeatureAPI) GetFeature(context.Context, *pb.GetFeatureRequest) (*pb.FeatureResponse, error) {
	return &pb.FeatureResponse{}, nil
}
func (m *mockHTTPFeatureAPI) ListMyFeatures(context.Context, *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
	return &pb.ListMyFeaturesResponse{}, nil
}
func (m *mockHTTPFeatureAPI) GetMyFeature(context.Context, *pb.GetMyFeatureRequest) (*pb.FeatureResponse, error) {
	return &pb.FeatureResponse{}, nil
}
func (m *mockHTTPFeatureAPI) AddMyFeatureImages(context.Context, *pb.AddMyFeatureImagesRequest) (*pb.FeatureResponse, error) {
	return &pb.FeatureResponse{}, nil
}
func (m *mockHTTPFeatureAPI) RemoveMyFeatureImage(context.Context, *pb.RemoveMyFeatureImageRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *mockHTTPFeatureAPI) UpdateMyFeature(context.Context, *pb.UpdateMyFeatureRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *mockHTTPFeatureAPI) GetFeatureTradeHistory(ctx context.Context, req *pb.GetFeatureTradeHistoryRequest) (*pb.GetFeatureTradeHistoryResponse, error) {
	if m.tradeHistory != nil {
		return m.tradeHistory(ctx, req)
	}
	return &pb.GetFeatureTradeHistoryResponse{}, nil
}

type mockHTTPMarketplaceAPI struct{}

func (*mockHTTPMarketplaceAPI) BuyFeature(context.Context, *pb.BuyFeatureRequest) (*pb.BuyFeatureResponse, error) {
	return &pb.BuyFeatureResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) SendBuyRequest(context.Context, *pb.SendBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	return &pb.BuyRequestResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) AcceptBuyRequest(context.Context, *pb.AcceptBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	return &pb.BuyRequestResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) RejectBuyRequest(context.Context, *pb.RejectBuyRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) DeleteBuyRequest(context.Context, *pb.DeleteBuyRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) ListBuyRequests(context.Context, *pb.ListBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	return &pb.BuyRequestsResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) ListReceivedBuyRequests(context.Context, *pb.ListReceivedBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	return &pb.BuyRequestsResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) CreateSellRequest(context.Context, *pb.CreateSellRequestRequest) (*pb.SellRequestResponse, error) {
	return &pb.SellRequestResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) ListSellRequests(context.Context, *pb.ListSellRequestsRequest) (*pb.SellRequestsResponse, error) {
	return &pb.SellRequestsResponse{}, nil
}
func (*mockHTTPMarketplaceAPI) DeleteSellRequest(context.Context, *pb.DeleteSellRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) UpdateGracePeriod(context.Context, *pb.UpdateGracePeriodRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type mockHTTPBuildingAPI struct {
	getBuildings        func(context.Context, *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error)
	updateBuilding      func(context.Context, *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error)
	updateInformation   func(context.Context, *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error)
	destroyBuilding     func(context.Context, *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error)
	completedBuildings  func(context.Context, *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error)
}

func (*mockHTTPBuildingAPI) GetBuildPackage(context.Context, *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error) {
	return &pb.BuildPackageResponse{}, nil
}
func (*mockHTTPBuildingAPI) BuildFeature(context.Context, *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error) {
	return &pb.BuildFeatureResponse{}, nil
}
func (m *mockHTTPBuildingAPI) GetBuildings(ctx context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
	if m.getBuildings != nil {
		return m.getBuildings(ctx, req)
	}
	return &pb.BuildingsResponse{}, nil
}
func (m *mockHTTPBuildingAPI) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
	if m.updateBuilding != nil {
		return m.updateBuilding(ctx, req)
	}
	return &pb.BuildingResponse{}, nil
}
func (m *mockHTTPBuildingAPI) UpdateBuildingInformation(ctx context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error) {
	if m.updateInformation != nil {
		return m.updateInformation(ctx, req)
	}
	return &pb.UpdateBuildingInformationResponse{}, nil
}
func (m *mockHTTPBuildingAPI) DestroyBuilding(ctx context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
	if m.destroyBuilding != nil {
		return m.destroyBuilding(ctx, req)
	}
	return &pb.BuildingResponse{}, nil
}
func (m *mockHTTPBuildingAPI) ListCompletedBuildings(ctx context.Context, req *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error) {
	if m.completedBuildings != nil {
		return m.completedBuildings(ctx, req)
	}
	return &pb.ListCompletedBuildingsResponse{}, nil
}

type mockHTTPProfitAPI struct {
	single func(context.Context, *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error)
}

func (*mockHTTPProfitAPI) GetHourlyProfits(context.Context, *pb.GetHourlyProfitsRequest) (*pb.HourlyProfitsResponse, error) {
	return &pb.HourlyProfitsResponse{}, nil
}
func (*mockHTTPProfitAPI) GetProfitsByApplication(context.Context, *pb.GetProfitsByApplicationRequest) (*pb.ProfitsByApplicationResponse, error) {
	return &pb.ProfitsByApplicationResponse{}, nil
}
func (m *mockHTTPProfitAPI) GetSingleProfit(ctx context.Context, req *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error) {
	if m.single != nil {
		return m.single(ctx, req)
	}
	return &pb.HourlyProfitResponse{}, nil
}

func newHTTPFeaturesHandler(feature *mockHTTPFeatureAPI, building *mockHTTPBuildingAPI) *handler.HTTPFeaturesHandler {
	return handler.NewHTTPFeaturesHandler(feature, &mockHTTPMarketplaceAPI{}, building, nil)
}

func requestWithUser(req *http.Request, userID uint64) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: userID}))
}

func TestHTTPListFeaturesPointsContract(t *testing.T) {
	for _, test := range []struct {
		name, target string
		want         []string
	}{
		{"indexed points", "/api/features?points[0]=10,20&points[1]=30,20&points[2]=30,40&points[3]=10,40", []string{"10,20", "30,20", "30,40", "10,40"}},
		{"repeated points", "/api/features?points[]=10,20&points[]=30,20&points[]=30,40&points[]=10,40", []string{"10,20", "30,20", "30,40", "10,40"}},
		{"JSON points", `/api/features?points=["10,20","30,20","30,40","10,40"]`, []string{"10,20", "30,20", "30,40", "10,40"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			feature := &mockHTTPFeatureAPI{listFeatures: func(_ context.Context, req *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error) {
				assert.Equal(t, test.want, req.Points)
				return &pb.FeaturesResponse{}, nil
			}}
			w := httptest.NewRecorder()
			newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).ListFeatures(w, httptest.NewRequest(http.MethodGet, test.target, nil))
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}

	t.Run("missing points returns validation response", func(t *testing.T) {
		w := httptest.NewRecorder()
		newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, &mockHTTPBuildingAPI{}).ListFeatures(w, httptest.NewRequest(http.MethodGet, "/api/features", nil))
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Contains(t, body, "errors")
	})
}

func TestHTTPBuildingMutationRoutes(t *testing.T) {
	var updated, patched, destroyed *pb.BuildingInformation
	building := &mockHTTPBuildingAPI{
		updateBuilding: func(_ context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			updated = &pb.BuildingInformation{}
			return &pb.BuildingResponse{}, nil
		},
		updateInformation: func(_ context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			patched = req.Information
			return &pb.UpdateBuildingInformationResponse{Information: req.Information}, nil
		},
		destroyBuilding: func(_ context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			assert.Equal(t, "1001", req.BuildingModelId)
			destroyed = &pb.BuildingInformation{}
			return &pb.BuildingResponse{}, nil
		},
	}
	h := newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, building)
	for _, test := range []struct {
		name, method, target, body string
	}{
		{"PUT", http.MethodPut, "/api/features/42/build/buildings/1001", `{"launched_satisfaction":"50"}`},
		{"POST method PUT", http.MethodPost, "/api/features/42/build/buildings/1001?_method=put", `{"launched_satisfaction":"50"}`},
		{"PATCH", http.MethodPatch, "/api/features/42/build/buildings/1001", `{"information":{"name":"Updated Store"}}`},
		{"POST method PATCH", http.MethodPost, "/api/features/42/build/buildings/1001?_method=patch", `{"information":{"name":"Updated Store"}}`},
		{"DELETE", http.MethodDelete, "/api/features/42/build/buildings/1001", ""},
		{"POST method DELETE", http.MethodPost, "/api/features/42/build/buildings/1001?_method=delete", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := requestWithUser(httptest.NewRequest(test.method, test.target, bytes.NewBufferString(test.body)), 7)
			req.Header.Set("Content-Type", "application/json")
			h.HandleFeaturesRoutes(w, req)
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		})
	}
	assert.NotNil(t, updated)
	assert.Equal(t, "Updated Store", patched.Name)
	assert.NotNil(t, destroyed)
}

func TestHTTPCompletedBuildingsRoutes(t *testing.T) {
	building := &mockHTTPBuildingAPI{completedBuildings: func(_ context.Context, _ *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error) {
		return &pb.ListCompletedBuildingsResponse{
			Data: []*pb.CompletedBuilding{{Id: 1, FeatureId: 10, FeaturePropertiesId: "QA-1"}},
			Links: &pb.PaginationLinks{}, Meta: &pb.FeatureTradeHistoryPaginationMeta{},
		}, nil
	}}
	h := newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, building)

	t.Run("takes precedence over feature lookup", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.HandleFeaturesRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "invalid feature ID")
	})
	t.Run("specific mux registration takes precedence", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle("GET /api/features/buildings/completed", http.HandlerFunc(h.ListCompletedBuildings))
		mux.Handle("/api/features/", http.HandlerFunc(h.HandleFeaturesRoutes))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/features/buildings/completed", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("building list still routes correctly", func(t *testing.T) {
		building.getBuildings = func(_ context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
			assert.Equal(t, uint64(42), req.FeatureId)
			return &pb.BuildingsResponse{}, nil
		}
		w := httptest.NewRecorder()
		h.HandleFeaturesRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/42/build/buildings", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHTTPGetSingleProfitLaravelResourceShape(t *testing.T) {
	profit := &mockHTTPProfitAPI{single: func(_ context.Context, req *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error) {
		assert.Equal(t, uint64(1), req.ProfitId)
		assert.Equal(t, uint64(42), req.UserId)
		return &pb.HourlyProfitResponse{Profit: &pb.HourlyProfit{
			Id: 1, FeatureId: 999, UserId: 42, FeatureDbId: 100, PropertiesId: "abc123", Karbari: "m",
			Amount: "12.345", DeadLine: "1403/01/15", IsActive: true,
		}}, nil
	}}
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/hourly-profits/1", nil), 42)
	handler.NewHTTPProfitHandler(profit).Handle(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "abc123", data["feature_id"])
	assert.Equal(t, float64(100), data["feature_db_id"])
	assert.Equal(t, "m", data["karbari"])
	assert.Equal(t, float64(42), data["user_id"])
}

func TestHTTPTradeHistoryRoutesExtractFeatureID(t *testing.T) {
	feature := &mockHTTPFeatureAPI{tradeHistory: func(_ context.Context, req *pb.GetFeatureTradeHistoryRequest) (*pb.GetFeatureTradeHistoryResponse, error) {
		assert.Equal(t, uint64(99), req.FeatureId)
		return &pb.GetFeatureTradeHistoryResponse{Links: &pb.PaginationLinks{}, Meta: &pb.FeatureTradeHistoryPaginationMeta{}}, nil
	}}
	h := newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{})

	for _, target := range []string{"/api/features/99/trade-history", "/api/features/99/trade-history."} {
		t.Run(target, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.HandleFeaturesRoutes(w, httptest.NewRequest(http.MethodGet, target, nil))
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
