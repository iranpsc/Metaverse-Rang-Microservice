package handler_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/features-service/internal/handler"
	authpb "metarang/shared/pb/auth"
	pb "metarang/shared/pb/features"
)

type routeAuthClient struct{}

func (routeAuthClient) Register(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error) {
	return nil, nil
}
func (routeAuthClient) Redirect(context.Context, *authpb.RedirectRequest, ...grpc.CallOption) (*authpb.RedirectResponse, error) {
	return nil, nil
}
func (routeAuthClient) Callback(context.Context, *authpb.CallbackRequest, ...grpc.CallOption) (*authpb.CallbackResponse, error) {
	return nil, nil
}
func (routeAuthClient) GetMe(context.Context, *authpb.GetMeRequest, ...grpc.CallOption) (*authpb.UserResponse, error) {
	return nil, nil
}
func (routeAuthClient) Logout(context.Context, *authpb.LogoutRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (routeAuthClient) ValidateToken(context.Context, *authpb.ValidateTokenRequest, ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return &authpb.ValidateTokenResponse{Valid: true, UserId: 2}, nil
}
func (routeAuthClient) RequestAccountSecurity(context.Context, *authpb.RequestAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (routeAuthClient) VerifyAccountSecurity(context.Context, *authpb.VerifyAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (routeAuthClient) CheckAccountSecurity(context.Context, *authpb.CheckAccountSecurityRequest, ...grpc.CallOption) (*authpb.CheckAccountSecurityResponse, error) {
	return &authpb.CheckAccountSecurityResponse{Unlocked: true}, nil
}

type mockHTTPMapAPI struct{}

func (mockHTTPMapAPI) ListMaps(context.Context, *pb.ListMapsRequest) (*pb.ListMapsResponse, error) {
	return &pb.ListMapsResponse{Maps: []*pb.Map{{Id: 1, Name: "City", Color: "red", CentralPointCoordinates: `[1,2]`, SoldFeaturesPercentage: "10"}}}, nil
}
func (mockHTTPMapAPI) GetMap(context.Context, *pb.GetMapRequest) (*pb.GetMapResponse, error) {
	return &pb.GetMapResponse{Map: &pb.Map{Id: 1, Name: "City"}}, nil
}
func (mockHTTPMapAPI) GetMapBorder(context.Context, *pb.GetMapRequest) (*pb.GetMapBorderResponse, error) {
	return &pb.GetMapBorderResponse{Data: &pb.MapBorderData{BorderCoordinates: `[[0,0]]`}}, nil
}

type mockHTTPIsicAPI struct{}

func (mockHTTPIsicAPI) ListIsicCodes(context.Context, *pb.ListIsicCodesRequest) (*pb.ListIsicCodesResponse, error) {
	code := uint64(11)
	return &pb.ListIsicCodesResponse{
		Data:  []*pb.IsicCode{{Id: 1, Name: "n", Code: &code, Verified: true}},
		Links: &pb.PaginationLinks{},
		Meta:  &pb.FeatureTradeHistoryPaginationMeta{CurrentPage: 1, PerPage: 10},
	}, nil
}

func TestHTTPRoutesCoverage(t *testing.T) {
	h := handler.NewHTTPFeaturesHandler(&mockHTTPFeatureAPI{}, &mockHTTPMarketplaceAPI{}, &mockHTTPBuildingAPI{
		getBuildings: func(_ context.Context, _ *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
			return &pb.BuildingsResponse{Buildings: sampleHTTPFeature().BuildingModels}, nil
		},
	}, routeAuthClient{})

	withUserJSON := func(method, target, body string) *http.Request {
		req := requestWithUser(httptest.NewRequest(method, target, strings.NewReader(body)), 2)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer tok")
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		return req
	}
	serve := func(fn func(http.ResponseWriter, *http.Request), req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		fn(w, req)
		return w
	}

	assert.Equal(t, 200, serve(h.HandleFeaturesRoutes, withUserJSON(http.MethodGet, "/api/features/1", "")).Code)
	assert.Equal(t, 200, serve(h.HandleFeaturesRoutes, withUserJSON(http.MethodPost, "/api/features/buy/1", "")).Code)
	assert.Equal(t, 200, serve(h.HandleFeaturesRoutes, withUserJSON(http.MethodGet, "/api/features/1/build/package", "")).Code)
	assert.Equal(t, 200, serve(h.HandleFeaturesRoutes, withUserJSON(http.MethodPost, "/api/features/1/build/m1", `{"launched_satisfaction":"1","rotation":"0","position":"1,2"}`)).Code)
	assert.Equal(t, 200, serve(h.HandleFeaturesRoutes, withUserJSON(http.MethodGet, "/api/features/1/build/buildings", "")).Code)

	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodGet, "/api/buy-requests", "")).Code)
	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodGet, "/api/buy-requests/recieved", "")).Code)
	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodPost, "/api/buy-requests/store/1", `{"note":"n","price_psc":10,"price_irr":20}`)).Code)
	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodPost, "/api/buy-requests/accept/9", "")).Code)
	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodPost, "/api/buy-requests/reject/9", "")).Code)
	assert.Equal(t, 204, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodDelete, "/api/buy-requests/delete/9", "")).Code)
	assert.Equal(t, 200, serve(h.HandleBuyRequestsRoutes, withUserJSON(http.MethodPost, "/api/buy-requests/add-grace-period/9", `{"grace_period":7}`)).Code)

	assert.Equal(t, 200, serve(h.HandleSellRequestsRoutes, withUserJSON(http.MethodGet, "/api/sell-requests", "")).Code)
	assert.Equal(t, 201, serve(h.HandleSellRequestsRoutes, withUserJSON(http.MethodPost, "/api/sell-requests/store/1", `{"price_psc":"10","price_irr":"20","minimum_price_percentage":90}`)).Code)
	assert.Equal(t, 200, serve(h.HandleSellRequestsRoutes, withUserJSON(http.MethodDelete, "/api/sell-requests/8", "")).Code)

	assert.Equal(t, 200, serve(h.HandleMyFeaturesRoutes, withUserJSON(http.MethodGet, "/api/my-features/2/features/1", "")).Code)
	assert.Equal(t, 204, serve(h.HandleMyFeaturesRoutes, withUserJSON(http.MethodPost, "/api/my-features/2/features/1", `{"minimum_price_percentage":90}`)).Code)
	assert.Equal(t, 200, serve(h.HandleMyFeaturesRoutes, withUserJSON(http.MethodPost, "/api/my-features/2/remove-image/1/image/4", "")).Code)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="images"; filename="a.jpg"`)
	hdr.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = part.Write([]byte{1, 2, 3})
	require.NoError(t, mw.Close())
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/my-features/2/add-image/1", &buf), 2)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	h.HandleMyFeaturesRoutes(w, req)
	assert.Equal(t, 200, w.Code, w.Body.String())

	// Updating feature images via the feature update URL (multipart) must not return
	// "request body is required" — images are uploaded through storage-service.
	var updateBuf bytes.Buffer
	updateMW := multipart.NewWriter(&updateBuf)
	updateHdr := make(textproto.MIMEHeader)
	updateHdr.Set("Content-Disposition", `form-data; name="images[]"; filename="b.png"`)
	// Browsers often omit part Content-Type; filename extension must be enough.
	part2, err := updateMW.CreatePart(updateHdr)
	require.NoError(t, err)
	_, _ = part2.Write([]byte{4, 5, 6})
	require.NoError(t, updateMW.Close())
	updateReq := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/my-features/2/features/1", &updateBuf), 2)
	updateReq.Header.Set("Content-Type", updateMW.FormDataContentType())
	updateReq.Header.Set("Authorization", "Bearer tok")
	updateReq.ContentLength = -1 // chunked / unset body length (Bruno and similar clients)
	updateW := httptest.NewRecorder()
	h.HandleMyFeaturesRoutes(updateW, updateReq)
	assert.Equal(t, 200, updateW.Code, updateW.Body.String())
	assert.NotContains(t, updateW.Body.String(), "request body is required")

	profit := handler.NewHTTPProfitHandler(&mockHTTPProfitAPI{})
	assert.Equal(t, 200, serve(profit.Handle, withUserJSON(http.MethodGet, "/api/hourly-profits?per_page=5", "")).Code)
	assert.Equal(t, 200, serve(profit.Handle, withUserJSON(http.MethodPost, "/api/hourly-profits", `{"karbari":"m"}`)).Code)
	single := serve(profit.Handle, withUserJSON(http.MethodPost, "/api/hourly-profits/11", ""))
	assert.True(t, single.Code == 200 || single.Code == 500)

	maps := handler.NewHTTPMapsHandler(mockHTTPMapAPI{})
	assert.Equal(t, 200, serve(maps.Handle, httptest.NewRequest(http.MethodGet, "/api/maps", nil)).Code)
	assert.Equal(t, 200, serve(maps.Handle, httptest.NewRequest(http.MethodGet, "/api/maps/1", nil)).Code)
	assert.Equal(t, 200, serve(maps.Handle, httptest.NewRequest(http.MethodGet, "/api/maps/1/border", nil)).Code)

	isic := handler.NewHTTPIsicCodesHandler(mockHTTPIsicAPI{})
	assert.Equal(t, 200, serve(isic.List, httptest.NewRequest(http.MethodGet, "/api/isic-codes?page=1&search=a", nil)).Code)

	unauth := httptest.NewRecorder()
	profit.Handle(unauth, httptest.NewRequest(http.MethodGet, "/api/hourly-profits", nil))
	assert.Equal(t, 401, unauth.Code)
	assert.Equal(t, 404, serve(profit.Handle, withUserJSON(http.MethodPut, "/api/hourly-profits", "")).Code)
	assert.Equal(t, 422, serve(profit.Handle, withUserJSON(http.MethodPost, "/api/hourly-profits", `{}`)).Code)
	assert.Equal(t, 422, serve(profit.Handle, withUserJSON(http.MethodPost, "/api/hourly-profits", `{"karbari":"z"}`)).Code)
	assert.Equal(t, 400, serve(maps.Handle, httptest.NewRequest(http.MethodGet, "/api/maps/nope", nil)).Code)
}

func TestHTTPProfitSingleWithProfit(t *testing.T) {
	profit := handler.NewHTTPProfitHandler(&mockHTTPProfitAPI{
		single: func(_ context.Context, _ *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error) {
			return &pb.HourlyProfitResponse{Profit: &pb.HourlyProfit{Id: 11, UserId: 2, Amount: "1.5", Karbari: "m"}}, nil
		},
		list: func(_ context.Context, _ *pb.GetHourlyProfitsRequest) (*pb.HourlyProfitsResponse, error) {
			return &pb.HourlyProfitsResponse{
				Profits: []*pb.HourlyProfit{{Id: 11, UserId: 2, Amount: "1.5", Karbari: "m"}},
				HasMore: true,
			}, nil
		},
	})
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/hourly-profits?per_page=2", nil), 2)
	w := httptest.NewRecorder()
	profit.Handle(w, req)
	assert.Equal(t, 200, w.Code)

	req = requestWithUser(httptest.NewRequest(http.MethodPost, "/api/hourly-profits/11", nil), 2)
	w = httptest.NewRecorder()
	profit.Handle(w, req)
	assert.Equal(t, 200, w.Code)
}
