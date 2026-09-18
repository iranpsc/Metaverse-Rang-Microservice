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
	pb "metarang/shared/pb/features"
	authpkg "metarang/shared/pkg/auth"
)

type mockHTTPFeatureAPI struct {
	listFeatures   func(context.Context, *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error)
	listMyFeatures func(context.Context, *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error)
	tradeHistory   func(context.Context, *pb.GetFeatureTradeHistoryRequest) (*pb.GetFeatureTradeHistoryResponse, error)
	getFeature     func(context.Context, *pb.GetFeatureRequest) (*pb.FeatureResponse, error)
	getMyFeature   func(context.Context, *pb.GetMyFeatureRequest) (*pb.FeatureResponse, error)
	addImages      func(context.Context, *pb.AddMyFeatureImagesRequest) (*pb.FeatureResponse, error)
}

func (m *mockHTTPFeatureAPI) ListFeatures(ctx context.Context, req *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error) {
	if m.listFeatures != nil {
		return m.listFeatures(ctx, req)
	}
	return &pb.FeaturesResponse{}, nil
}
func (m *mockHTTPFeatureAPI) GetFeature(ctx context.Context, req *pb.GetFeatureRequest) (*pb.FeatureResponse, error) {
	if m.getFeature != nil {
		return m.getFeature(ctx, req)
	}
	return &pb.FeatureResponse{Feature: sampleHTTPFeature()}, nil
}
func (m *mockHTTPFeatureAPI) ListMyFeatures(ctx context.Context, req *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
	if m.listMyFeatures != nil {
		return m.listMyFeatures(ctx, req)
	}
	return &pb.ListMyFeaturesResponse{Links: &pb.PaginationLinks{}, Meta: &pb.SimplePaginationMeta{}}, nil
}
func (m *mockHTTPFeatureAPI) GetMyFeature(context.Context, *pb.GetMyFeatureRequest) (*pb.FeatureResponse, error) {
	if m.getMyFeature != nil {
		return m.getMyFeature(context.Background(), nil)
	}
	return &pb.FeatureResponse{Feature: sampleHTTPFeature()}, nil
}
func (m *mockHTTPFeatureAPI) AddMyFeatureImages(context.Context, *pb.AddMyFeatureImagesRequest) (*pb.FeatureResponse, error) {
	if m.addImages != nil {
		return m.addImages(context.Background(), nil)
	}
	return &pb.FeatureResponse{Feature: sampleHTTPFeature()}, nil
}
func (m *mockHTTPFeatureAPI) RemoveMyFeatureImage(context.Context, *pb.RemoveMyFeatureImageRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *mockHTTPFeatureAPI) UpdateMyFeature(context.Context, *pb.UpdateMyFeatureRequest) (*pb.UpdateMyFeatureResponse, error) {
	return &pb.UpdateMyFeatureResponse{PricePsc: "12.5", PriceIrr: "450"}, nil
}
func (m *mockHTTPFeatureAPI) GetFeatureTradeHistory(ctx context.Context, req *pb.GetFeatureTradeHistoryRequest) (*pb.GetFeatureTradeHistoryResponse, error) {
	if m.tradeHistory != nil {
		return m.tradeHistory(ctx, req)
	}
	return &pb.GetFeatureTradeHistoryResponse{}, nil
}

type mockHTTPMarketplaceAPI struct{}

func sampleHTTPFeature() *pb.Feature {
	return &pb.Feature{
		Id: 1, OwnerId: 2, IsHourlyProfitActive: true,
		Properties: &pb.FeatureProperties{Id: "p1", Address: "a", Karbari: "m", Rgb: "d", Owner: "o", Label: "l", Area: "10", Density: 1, Stability: "1", PricePsc: "1", PriceIrr: "2"},
		Images:     []*pb.Image{{Id: 4, Url: "https://img"}},
		Seller:     &pb.Seller{Id: 3, Name: "Ali", Code: "hm-3"},
		Geometry:   &pb.Geometry{Id: 5, Coordinates: []*pb.Coordinate{{Id: 6, X: "1", Y: "2"}}},
		BuildingModels: []*pb.Building{{
			ConstructionStartDate: "2026-01-01T00:00:00Z",
			ConstructionEndDate:   "2099-02-01T00:00:00Z",
			Rotation:              "0",
			Position:              "1,2",
			Model:                 &pb.BuildingModel{Id: 11, ModelId: "1001", Name: "Tower", File: `{"url":"m.glb"}`, Images: "[]"},
		}},
	}
}

func sampleBuyRequest() *pb.BuyRequestResponse {
	return &pb.BuyRequestResponse{
		Id: 9, FeatureId: 1, Status: 0, Note: "n", PricePsc: "10", PriceIrr: "20",
		Buyer:             &pb.BuyerInfo{Id: 2, Code: "hm-2", ProfilePhoto: "p"},
		Seller:            &pb.SellerInfo{Id: 3, Code: "hm-3"},
		FeatureProperties: &pb.FeatureProperties{Id: "p1", Karbari: "m"},
	}
}

func sampleSellRequest() *pb.SellRequestResponse {
	return &pb.SellRequestResponse{
		Id: 8, FeatureId: 1, SellerId: 3, PricePsc: "10", PriceIrr: "20", Status: 0,
		FeatureProperties: &pb.FeatureProperties{Id: "p1", Karbari: "m"},
	}
}

func (*mockHTTPMarketplaceAPI) BuyFeature(context.Context, *pb.BuyFeatureRequest) (*pb.BuyFeatureResponse, error) {
	return &pb.BuyFeatureResponse{Feature: sampleHTTPFeature()}, nil
}
func (*mockHTTPMarketplaceAPI) SendBuyRequest(context.Context, *pb.SendBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	return sampleBuyRequest(), nil
}
func (*mockHTTPMarketplaceAPI) AcceptBuyRequest(context.Context, *pb.AcceptBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	return sampleBuyRequest(), nil
}
func (*mockHTTPMarketplaceAPI) RejectBuyRequest(context.Context, *pb.RejectBuyRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) DeleteBuyRequest(context.Context, *pb.DeleteBuyRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) ListBuyRequests(context.Context, *pb.ListBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	return &pb.BuyRequestsResponse{BuyRequests: []*pb.BuyRequestResponse{sampleBuyRequest()}}, nil
}
func (*mockHTTPMarketplaceAPI) ListReceivedBuyRequests(context.Context, *pb.ListReceivedBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	return &pb.BuyRequestsResponse{BuyRequests: []*pb.BuyRequestResponse{sampleBuyRequest()}}, nil
}
func (*mockHTTPMarketplaceAPI) CreateSellRequest(context.Context, *pb.CreateSellRequestRequest) (*pb.SellRequestResponse, error) {
	return sampleSellRequest(), nil
}
func (*mockHTTPMarketplaceAPI) ListSellRequests(context.Context, *pb.ListSellRequestsRequest) (*pb.SellRequestsResponse, error) {
	return &pb.SellRequestsResponse{SellRequests: []*pb.SellRequestResponse{sampleSellRequest()}}, nil
}
func (*mockHTTPMarketplaceAPI) ListFeatureSellRequests(context.Context, *pb.ListFeatureSellRequestsRequest) (*pb.SellRequestsResponse, error) {
	return &pb.SellRequestsResponse{SellRequests: []*pb.SellRequestResponse{sampleSellRequest()}}, nil
}
func (*mockHTTPMarketplaceAPI) DeleteSellRequest(context.Context, *pb.DeleteSellRequestRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (*mockHTTPMarketplaceAPI) UpdateGracePeriod(context.Context, *pb.UpdateGracePeriodRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type mockHTTPBuildingAPI struct {
	getBuildings       func(context.Context, *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error)
	updateBuilding     func(context.Context, *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error)
	updateInformation  func(context.Context, *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error)
	destroyBuilding    func(context.Context, *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error)
	completedBuildings func(context.Context, *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error)
}

func (*mockHTTPBuildingAPI) GetBuildPackage(context.Context, *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error) {
	return &pb.BuildPackageResponse{
		Models:      []*pb.BuildingModel{{Id: 1, ModelId: "1", Name: "n", Sku: "s", Images: "[]", Attributes: "[]", File: "{}", RequiredSatisfaction: "1"}},
		Coordinates: []string{"1,2"},
	}, nil
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
	list   func(context.Context, *pb.GetHourlyProfitsRequest) (*pb.HourlyProfitsResponse, error)
}

func (m *mockHTTPProfitAPI) GetHourlyProfits(ctx context.Context, req *pb.GetHourlyProfitsRequest) (*pb.HourlyProfitsResponse, error) {
	if m.list != nil {
		return m.list(ctx, req)
	}
	return &pb.HourlyProfitsResponse{}, nil
}
func (*mockHTTPProfitAPI) GetProfitsByApplication(context.Context, *pb.GetProfitsByApplicationRequest) (*pb.ProfitsByApplicationResponse, error) {
	return &pb.ProfitsByApplicationResponse{}, nil
}
func (m *mockHTTPProfitAPI) GetSingleProfit(ctx context.Context, req *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error) {
	if m.single != nil {
		return m.single(ctx, req)
	}
	return &pb.HourlyProfitResponse{Profit: &pb.HourlyProfit{}}, nil
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

func TestHTTPListFeaturesBuildingModels(t *testing.T) {
	feature := &mockHTTPFeatureAPI{listFeatures: func(_ context.Context, req *pb.ListFeaturesRequest) (*pb.FeaturesResponse, error) {
		assert.True(t, req.LoadBuildings)
		return &pb.FeaturesResponse{
			Features: []*pb.Feature{{
				Id:      7,
				OwnerId: 3,
				BuildingModels: []*pb.Building{{
					ConstructionStartDate: "2026-01-01T00:00:00Z",
					ConstructionEndDate:   "2026-02-01T00:00:00Z",
					Rotation:              "45",
					Position:              `{"x":1,"y":2}`,
					Model: &pb.BuildingModel{
						Id:      11,
						ModelId: "1001",
						Name:    "Tower",
						File:    `{"url":"m.glb"}`,
					},
				}},
			}},
		}, nil
	}}

	w := httptest.NewRecorder()
	newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).ListFeatures(
		w,
		httptest.NewRequest(http.MethodGet, "/api/features?points[]=10,20&points[]=30,20&points[]=30,40&points[]=10,40&load_buildings=1", nil),
	)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	featureMap := data[0].(map[string]interface{})
	models := featureMap["building_models"].([]interface{})
	require.Len(t, models, 1)
	model := models[0].(map[string]interface{})
	assert.Equal(t, float64(11), model["id"])
	assert.Equal(t, float64(1001), model["model_id"]) // numeric catalog id (main-compatible)
	assert.NotContains(t, model, "name")
	assert.NotContains(t, model, "sku")
	file := model["file"].(map[string]interface{})
	assert.Equal(t, "m.glb", file["url"])
	building := model["building"].(map[string]interface{})
	assert.Equal(t, float64(7), building["feature_id"])
	assert.Equal(t, float64(11), building["model_id"]) // local building_models.id
	assert.Equal(t, "45", building["rotation"])
	pos := building["position"].(map[string]interface{})
	assert.Equal(t, float64(1), pos["x"])
	assert.NotContains(t, building, "launched_satisfaction")
	assert.NotContains(t, building, "bubble_diameter")
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
			Data:  []*pb.CompletedBuilding{{Id: 1, FeatureId: 10, FeaturePropertiesId: "QA-1"}},
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

func TestHTTPGetSingleProfit_ExpectedResourceShape(t *testing.T) {
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

func TestHTTPListMyFeatures_SearchFilterAddressAndPagination(t *testing.T) {
	feature := &mockHTTPFeatureAPI{listMyFeatures: func(_ context.Context, req *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
		assert.Equal(t, uint64(42), req.UserId)
		assert.Equal(t, int32(2), req.Page)
		assert.Equal(t, "TO111", req.Search)
		assert.Equal(t, "m", req.Filter)
		return &pb.ListMyFeaturesResponse{
			Data: []*pb.Feature{{
				Id: 9,
				Properties: &pb.FeatureProperties{
					Id:                     "TO111",
					Address:                "12 Main St",
					PricePsc:               "100",
					PriceIrr:               "200",
					Stability:              "10.00",
					MinimumPricePercentage: 100,
					Karbari:                "m",
					Rgb:                    "#fff",
				},
			}},
			Links: &pb.PaginationLinks{
				First: "/api/my-features?filter=m&page=1&search=TO111",
				Prev:  "/api/my-features?filter=m&page=1&search=TO111",
			},
			Meta: &pb.SimplePaginationMeta{CurrentPage: 2, Path: "/api/my-features", PerPage: 5},
		}, nil
	}}

	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/my-features?page=2&search=TO111&filter=m", nil), 42)
	newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).ListMyFeatures(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	assert.Equal(t, float64(9), item["id"])
	props := item["properties"].(map[string]interface{})
	assert.Equal(t, "TO111", props["id"])
	assert.Equal(t, "12 Main St", props["address"])
	assert.Equal(t, "m", props["karbari"])
	meta := body["meta"].(map[string]interface{})
	assert.Equal(t, float64(2), meta["current_page"])
	assert.Equal(t, float64(5), meta["per_page"])
	links := body["links"].(map[string]interface{})
	assert.Contains(t, links["first"], "page=1")
	assert.Contains(t, links["first"], "search=TO111")
	assert.Contains(t, links["first"], "filter=m")
}

func TestHTTPListMyFeatures_TrimsSearchAndFilter(t *testing.T) {
	var gotSearch, gotFilter string
	feature := &mockHTTPFeatureAPI{listMyFeatures: func(_ context.Context, req *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
		gotSearch = req.Search
		gotFilter = req.Filter
		return &pb.ListMyFeaturesResponse{
			Data:  []*pb.Feature{},
			Links: &pb.PaginationLinks{First: "/api/my-features?page=1"},
			Meta:  &pb.SimplePaginationMeta{CurrentPage: 1, Path: "/api/my-features", PerPage: 5},
		}, nil
	}}
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/my-features?search=%20block%20&filter=%20t%20", nil), 1)
	newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).ListMyFeatures(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "block", gotSearch)
	assert.Equal(t, "t", gotFilter)
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

func TestHTTPFeatureSellRequestsRoute(t *testing.T) {
	var gotFeatureID uint64
	market := &mockHTTPMarketplaceAPIWithFeatureSells{
		listFeatureSellRequests: func(_ context.Context, req *pb.ListFeatureSellRequestsRequest) (*pb.SellRequestsResponse, error) {
			gotFeatureID = req.FeatureId
			return &pb.SellRequestsResponse{SellRequests: []*pb.SellRequestResponse{
				{Id: 2, FeatureId: 99, SellerId: 3, PricePsc: "15", PriceIrr: "25", Status: 0, CreatedAt: "newer"},
				{Id: 1, FeatureId: 99, SellerId: 3, PricePsc: "10", PriceIrr: "20", Status: 0, CreatedAt: "older"},
			}}, nil
		},
	}
	h := handler.NewHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, market, &mockHTTPBuildingAPI{}, nil)

	w := httptest.NewRecorder()
	h.HandleFeaturesRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/99/sell-requests", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint64(99), gotFeatureID)

	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	require.Len(t, body.Data, 2)
	assert.Equal(t, float64(2), body.Data[0]["id"])
	assert.Equal(t, float64(1), body.Data[1]["id"])
}

func TestHTTPFeatureSellRequestsRoute_InvalidFeatureID(t *testing.T) {
	h := newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, &mockHTTPBuildingAPI{})
	w := httptest.NewRecorder()
	h.HandleFeaturesRoutes(w, httptest.NewRequest(http.MethodGet, "/api/features/abc/sell-requests", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTPGetFeature_IncludesLatestSellRequestWhenForSale(t *testing.T) {
	feature := sampleHTTPFeature()
	feature.IsForSale = 1
	feature.LatestSellRequest = &pb.SellRequestResponse{
		Id: 8, FeatureId: 1, SellerId: 2, PricePsc: "12.5", PriceIrr: "450", Status: 0, CreatedAt: "1404/01/01",
	}
	api := &mockHTTPFeatureAPI{getFeature: func(_ context.Context, req *pb.GetFeatureRequest) (*pb.FeatureResponse, error) {
		assert.Equal(t, uint64(1), req.FeatureId)
		return &pb.FeatureResponse{Feature: feature}, nil
	}}
	w := httptest.NewRecorder()
	newHTTPFeaturesHandler(api, &mockHTTPBuildingAPI{}).GetFeature(w, httptest.NewRequest(http.MethodGet, "/api/features/1", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, true, data["is_for_sale"])
	latest := data["latest_sell_request"].(map[string]interface{})
	assert.Equal(t, float64(8), latest["id"])
	assert.Equal(t, "12.5", latest["price_psc"])
	assert.Equal(t, "450", latest["price_irr"])
}

func TestHTTPGetFeature_OmitsLatestSellRequestWhenNotForSale(t *testing.T) {
	w := httptest.NewRecorder()
	newHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, &mockHTTPBuildingAPI{}).GetFeature(w, httptest.NewRequest(http.MethodGet, "/api/features/1", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, false, data["is_for_sale"])
	assert.NotContains(t, data, "latest_sell_request")
}

func TestHTTPListMyFeatures_IncludesLatestSellRequestWhenForSale(t *testing.T) {
	api := &mockHTTPFeatureAPI{listMyFeatures: func(_ context.Context, _ *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
		return &pb.ListMyFeaturesResponse{
			Data: []*pb.Feature{{
				Id:        9,
				IsForSale: 1,
				LatestSellRequest: &pb.SellRequestResponse{
					Id: 8, FeatureId: 9, SellerId: 42, PricePsc: "1.25", PriceIrr: "250.5", Status: 0,
				},
				Properties: &pb.FeatureProperties{Id: "TO111", PricePsc: "1.25", PriceIrr: "250.5"},
			}},
			Links: &pb.PaginationLinks{First: "/api/my-features?page=1"},
			Meta:  &pb.SimplePaginationMeta{CurrentPage: 1, Path: "/api/my-features", PerPage: 5},
		}, nil
	}}
	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/my-features", nil), 42)
	newHTTPFeaturesHandler(api, &mockHTTPBuildingAPI{}).ListMyFeatures(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	item := body["data"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, true, item["is_for_sale"])
	latest := item["latest_sell_request"].(map[string]interface{})
	assert.Equal(t, float64(8), latest["id"])
	assert.Equal(t, "1.25", latest["price_psc"])
	assert.Equal(t, "250.5", latest["price_irr"])
}

type mockHTTPMarketplaceAPIWithFeatureSells struct {
	mockHTTPMarketplaceAPI
	listFeatureSellRequests func(context.Context, *pb.ListFeatureSellRequestsRequest) (*pb.SellRequestsResponse, error)
}

func (m *mockHTTPMarketplaceAPIWithFeatureSells) ListFeatureSellRequests(ctx context.Context, req *pb.ListFeatureSellRequestsRequest) (*pb.SellRequestsResponse, error) {
	if m.listFeatureSellRequests != nil {
		return m.listFeatureSellRequests(ctx, req)
	}
	return m.mockHTTPMarketplaceAPI.ListFeatureSellRequests(ctx, req)
}

func TestHTTPListMyFeatures_IncludesIsForSaleOnEachItem(t *testing.T) {
	feature := &mockHTTPFeatureAPI{listMyFeatures: func(_ context.Context, req *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
		assert.Equal(t, uint64(42), req.UserId)
		return &pb.ListMyFeaturesResponse{
			Data: []*pb.Feature{
				{
					Id: 9, IsForSale: 1, Properties: &pb.FeatureProperties{Id: "TO111"},
					LatestSellRequest: &pb.SellRequestResponse{Id: 8, FeatureId: 9, SellerId: 42, Status: 0, PricePsc: "10", PriceIrr: "20"},
				},
				{Id: 10, IsForSale: 0, Properties: &pb.FeatureProperties{Id: "TO222"}},
			},
			Links: &pb.PaginationLinks{First: "/api/my-features?page=1"},
			Meta:  &pb.SimplePaginationMeta{CurrentPage: 1, Path: "/api/my-features", PerPage: 5},
		}, nil
	}}

	w := httptest.NewRecorder()
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/my-features?page=1", nil), 42)
	newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).ListMyFeatures(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	require.Len(t, data, 2)
	first := data[0].(map[string]interface{})
	second := data[1].(map[string]interface{})
	assert.Equal(t, float64(9), first["id"])
	assert.Equal(t, float64(1), first["is-for-sale"])
	latest := first["latest-sell-request"].(map[string]interface{})
	assert.Equal(t, float64(8), latest["id"])
	assert.Equal(t, float64(10), second["id"])
	assert.Equal(t, float64(0), second["is-for-sale"])
	assert.Nil(t, second["latest-sell-request"])
}

func TestHTTPGetFeature_IncludesIsForSaleWithoutLatestSellRequest(t *testing.T) {
	feature := &mockHTTPFeatureAPI{getFeature: func(_ context.Context, req *pb.GetFeatureRequest) (*pb.FeatureResponse, error) {
		return &pb.FeatureResponse{Feature: &pb.Feature{Id: 42, OwnerId: 2, IsForSale: 1}}, nil
	}}
	w := httptest.NewRecorder()
	newHTTPFeaturesHandler(feature, &mockHTTPBuildingAPI{}).HandleFeaturesRoutes(
		w,
		httptest.NewRequest(http.MethodGet, "/api/features/42", nil),
	)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, float64(42), data["id"])
	assert.Equal(t, true, data["is_for_sale"])
	assert.NotContains(t, data, "is-for-sale")
	assert.NotContains(t, data, "latest_sell_request")
	assert.NotContains(t, data, "latest-sell-request")
}
