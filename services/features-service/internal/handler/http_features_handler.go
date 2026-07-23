package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"metarang/features-service/internal/middleware"
	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"

	"google.golang.org/protobuf/types/known/emptypb"
)

type featureHTTPAPI interface {
	ListFeatures(context.Context, *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error)
	GetFeature(context.Context, *featurespb.GetFeatureRequest) (*featurespb.FeatureResponse, error)
	ListMyFeatures(context.Context, *featurespb.ListMyFeaturesRequest) (*featurespb.ListMyFeaturesResponse, error)
	GetMyFeature(context.Context, *featurespb.GetMyFeatureRequest) (*featurespb.FeatureResponse, error)
	AddMyFeatureImages(context.Context, *featurespb.AddMyFeatureImagesRequest) (*featurespb.FeatureResponse, error)
	RemoveMyFeatureImage(context.Context, *featurespb.RemoveMyFeatureImageRequest) (*emptypb.Empty, error)
	UpdateMyFeature(context.Context, *featurespb.UpdateMyFeatureRequest) (*emptypb.Empty, error)
	GetFeatureTradeHistory(context.Context, *featurespb.GetFeatureTradeHistoryRequest) (*featurespb.GetFeatureTradeHistoryResponse, error)
}
type marketplaceHTTPAPI interface {
	BuyFeature(context.Context, *featurespb.BuyFeatureRequest) (*featurespb.BuyFeatureResponse, error)
	SendBuyRequest(context.Context, *featurespb.SendBuyRequestRequest) (*featurespb.BuyRequestResponse, error)
	AcceptBuyRequest(context.Context, *featurespb.AcceptBuyRequestRequest) (*featurespb.BuyRequestResponse, error)
	RejectBuyRequest(context.Context, *featurespb.RejectBuyRequestRequest) (*emptypb.Empty, error)
	DeleteBuyRequest(context.Context, *featurespb.DeleteBuyRequestRequest) (*emptypb.Empty, error)
	ListBuyRequests(context.Context, *featurespb.ListBuyRequestsRequest) (*featurespb.BuyRequestsResponse, error)
	ListReceivedBuyRequests(context.Context, *featurespb.ListReceivedBuyRequestsRequest) (*featurespb.BuyRequestsResponse, error)
	CreateSellRequest(context.Context, *featurespb.CreateSellRequestRequest) (*featurespb.SellRequestResponse, error)
	ListSellRequests(context.Context, *featurespb.ListSellRequestsRequest) (*featurespb.SellRequestsResponse, error)
	DeleteSellRequest(context.Context, *featurespb.DeleteSellRequestRequest) (*emptypb.Empty, error)
	UpdateGracePeriod(context.Context, *featurespb.UpdateGracePeriodRequest) (*emptypb.Empty, error)
}
type buildingHTTPAPI interface {
	GetBuildPackage(context.Context, *featurespb.GetBuildPackageRequest) (*featurespb.BuildPackageResponse, error)
	BuildFeature(context.Context, *featurespb.BuildFeatureRequest) (*featurespb.BuildFeatureResponse, error)
	GetBuildings(context.Context, *featurespb.GetBuildingsRequest) (*featurespb.BuildingsResponse, error)
	UpdateBuilding(context.Context, *featurespb.UpdateBuildingRequest) (*featurespb.BuildingResponse, error)
	UpdateBuildingInformation(context.Context, *featurespb.UpdateBuildingInformationRequest) (*featurespb.UpdateBuildingInformationResponse, error)
	DestroyBuilding(context.Context, *featurespb.DestroyBuildingRequest) (*featurespb.BuildingResponse, error)
	ListCompletedBuildings(context.Context, *featurespb.ListCompletedBuildingsRequest) (*featurespb.ListCompletedBuildingsResponse, error)
}

// HTTPFeaturesHandler exposes the existing feature RPC handlers without a loopback dial.
type HTTPFeaturesHandler struct {
	feature  featureHTTPAPI
	market   marketplaceHTTPAPI
	building buildingHTTPAPI
	auth     authpb.AuthServiceClient
}

func NewHTTPFeaturesHandler(feature featureHTTPAPI, market marketplaceHTTPAPI, building buildingHTTPAPI, auth authpb.AuthServiceClient) *HTTPFeaturesHandler {
	return &HTTPFeaturesHandler{feature: feature, market: market, building: building, auth: auth}
}

func (h *HTTPFeaturesHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	points, ok := parsePoints(r.URL.Query())
	if !ok {
		writeValidationError(w, "points parameter is required")
		return
	}
	if len(points) < 4 {
		writeValidationError(w, "points array must have at least 4 elements")
		return
	}
	userID := uint64(0)
	if user, err := middleware.GetUserFromRequest(r); err == nil {
		userID = user.UserID
	}
	resp, err := h.feature.ListFeatures(r.Context(), &featurespb.ListFeaturesRequest{Points: points, LoadBuildings: r.URL.Query().Get("load_buildings") == "true" || r.URL.Query().Get("load_buildings") == "1", UserFeaturesLocation: r.URL.Query().Get("user_features_location") == "true" || r.URL.Query().Get("user_features_location") == "1"})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := make([]map[string]interface{}, 0, len(resp.Features))
	for _, feature := range resp.Features {
		data = append(data, listFeatureMap(feature, userID > 0, r.URL.Query().Get("load_buildings") == "true" || r.URL.Query().Get("load_buildings") == "1"))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}
func (h *HTTPFeaturesHandler) GetFeature(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	resp, err := h.feature.GetFeature(contextWithClientIP(r), &featurespb.GetFeatureRequest{FeatureId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": featureMap(resp.Feature)})
}
func (h *HTTPFeaturesHandler) HandleFeaturesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/")
	if strings.HasPrefix(path, "buy/") {
		h.BuyFeature(w, r)
		return
	}
	if path == "buildings/completed" {
		h.ListCompletedBuildings(w, r)
		return
	}
	if isFeatureTradeHistoryPath(path) {
		h.TradeHistory(w, r)
		return
	}
	if strings.Contains(path, "/build/package") {
		h.BuildPackage(w, r)
		return
	}
	if strings.Contains(path, "/build/buildings/") {
		h.BuildingMutation(w, r)
		return
	}
	if strings.Contains(path, "/build/buildings") {
		h.GetBuildings(w, r)
		return
	}
	if strings.Contains(path, "/build/") {
		h.BuildFeature(w, r)
		return
	}
	h.GetFeature(w, r)
}
func (h *HTTPFeaturesHandler) BuyFeature(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/features/buy/")
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	resp, err := h.market.BuyFeature(contextWithClientIP(r), &featurespb.BuyFeatureRequest{FeatureId: id, BuyerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": featureMap(resp.Feature)})
}
func (h *HTTPFeaturesHandler) BuildPackage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.user(w, r); !ok {
		return
	}
	id, err := featureID(r)
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	page := pageQuery(r, 1)
	resp, err := h.building.GetBuildPackage(r.Context(), &featurespb.GetBuildPackageRequest{FeatureId: id, Page: page})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, m := range resp.Models {
		data = append(data, map[string]interface{}{"id": m.Id, "model_id": m.ModelId, "name": m.Name, "sku": m.Sku, "images": parseJSONString(m.Images), "attributes": parseJSONString(m.Attributes), "file": parseJSONString(m.File), "required_satisfaction": m.RequiredSatisfaction})
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "feature": map[string]interface{}{"coordinates": resp.Coordinates}})
}
func (h *HTTPFeaturesHandler) BuildFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.user(w, r); !ok {
		return
	}
	id, err := featureID(r)
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/"), "/")
	if len(parts) < 3 {
		writeError(w, 400, "feature ID and building model ID are required")
		return
	}
	body := map[string]interface{}{}
	if err = decodeBody(r, &body); err != nil {
		writeValidationError(w, "request body is required")
		return
	}
	req := buildingRequest(id, parts[2], body)
	if _, err = h.building.BuildFeature(r.Context(), req); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{})
}
func (h *HTTPFeaturesHandler) GetBuildings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	id, err := featureID(r)
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	resp, err := h.building.GetBuildings(r.Context(), &featurespb.GetBuildingsRequest{FeatureId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := []map[string]interface{}{}
	for _, b := range resp.Buildings {
		if b.Model != nil {
			out = append(out, map[string]interface{}{"id": b.Model.Id, "model_id": b.Model.ModelId, "name": b.Model.Name, "sku": b.Model.Sku, "images": parseJSONString(b.Model.Images), "attributes": parseJSONString(b.Model.Attributes), "file": parseJSONString(b.Model.File), "required_satisfaction": b.Model.RequiredSatisfaction, "building": map[string]interface{}{"model_id": b.Model.ModelId, "feature_id": id, "construction_start_date": b.ConstructionStartDate, "construction_end_date": b.ConstructionEndDate, "launched_satisfaction": b.LaunchedSatisfaction, "information": parseJSONString(b.Information), "rotation": b.Rotation, "position": b.Position, "bubble_diameter": b.BubbleDiameter}})
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *HTTPFeaturesHandler) BuildingMutation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.user(w, r); !ok {
		return
	}
	id, err := featureID(r)
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/"), "/")
	if len(parts) < 4 {
		writeError(w, 400, "feature ID and building model ID are required")
		return
	}
	model := parts[3]
	switch effectiveHTTPMethod(r) {
	case http.MethodDelete:
		_, err = h.building.DestroyBuilding(r.Context(), &featurespb.DestroyBuildingRequest{FeatureId: id, BuildingModelId: model})
		if err == nil {
			writeJSON(w, 200, map[string]interface{}{})
		}
	case http.MethodPut:
		body := map[string]interface{}{}
		if err = decodeBody(r, &body); err == nil {
			_, err = h.building.UpdateBuilding(r.Context(), updateBuildingRequest(id, model, body))
		}
		if err == nil {
			writeJSON(w, 200, map[string]interface{}{})
		}
	case http.MethodPatch:
		body := map[string]interface{}{}
		if err = decodeBody(r, &body); err == nil {
			info := parseBuildingInformation(body)
			if info == nil {
				writeValidationError(w, "information is required")
				return
			}
			var resp *featurespb.UpdateBuildingInformationResponse
			resp, err = h.building.UpdateBuildingInformation(r.Context(), &featurespb.UpdateBuildingInformationRequest{FeatureId: id, BuildingModelId: model, Information: info})
			if err == nil {
				writeJSON(w, 200, map[string]interface{}{"information": buildingInformationMap(resp.Information)}, true)
			}
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeGRPCError(w, err)
	}
}
func (h *HTTPFeaturesHandler) ListCompletedBuildings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	resp, err := h.building.ListCompletedBuildings(r.Context(), &featurespb.ListCompletedBuildingsRequest{Page: pageQuery(r, 1)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, x := range resp.Data {
		data = append(data, map[string]interface{}{"id": x.Id, "feature_id": x.FeatureId, "feature_properties_id": x.FeaturePropertiesId, "length": optionalString(x.Length), "width": optionalString(x.Width), "density": optionalString(x.Density), "karbari": x.Karbari})
	}
	writeJSON(w, 200, paginated(data, resp.Links, resp.Meta))
}
func (h *HTTPFeaturesHandler) TradeHistory(w http.ResponseWriter, r *http.Request) {
	id, err := featureIDFromTradeHistoryRequest(r)
	if err != nil || id == 0 {
		writeError(w, 400, "invalid feature ID")
		return
	}
	resp, err := h.feature.GetFeatureTradeHistory(r.Context(), &featurespb.GetFeatureTradeHistoryRequest{FeatureId: id, Page: pageQuery(r, 1)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, x := range resp.Data {
		row := map[string]interface{}{"id": optionalUint64(x.Id), "type": x.Type, "participant_code": optionalString(x.ParticipantCode), "participant_label": x.ParticipantLabel, "date_time": nil, "price": nil}
		if x.DateTime != nil {
			row["date_time"] = map[string]interface{}{"date": x.DateTime.Date, "month_name": x.DateTime.MonthName, "year": x.DateTime.Year, "time": x.DateTime.Time, "formatted": x.DateTime.Formatted}
		}
		if x.Price != nil {
			row["price"] = map[string]interface{}{"type": x.Price.Type, "price_psc": optionalInt64(x.Price.PricePsc), "price_irr": optionalInt64(x.Price.PriceIrr), "color": optionalString(x.Price.Color), "color_name": optionalString(x.Price.ColorName), "color_amount": optionalInt64(x.Price.ColorAmount)}
		}
		data = append(data, row)
	}
	writeJSON(w, 200, paginated(data, resp.Links, resp.Meta))
}

func (h *HTTPFeaturesHandler) HandleBuyRequestsRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/buy-requests"), "/")
	switch {
	case path == "":
		if r.Method == http.MethodGet {
			h.ListBuyRequests(w, r)
			return
		}
	case path == "recieved" && r.Method == http.MethodGet:
		h.ListReceivedBuyRequests(w, r)
		return
	case strings.HasPrefix(path, "store/") && r.Method == http.MethodPost:
		h.SendBuyRequest(w, r)
		return
	case strings.HasPrefix(path, "accept/") && r.Method == http.MethodPost:
		h.AcceptBuyRequest(w, r)
		return
	case strings.HasPrefix(path, "reject/") && r.Method == http.MethodPost:
		h.RejectBuyRequest(w, r)
		return
	case strings.HasPrefix(path, "delete/") && r.Method == http.MethodDelete:
		h.DeleteBuyRequest(w, r)
		return
	case strings.HasPrefix(path, "add-grace-period/") && r.Method == http.MethodPost:
		h.UpdateGracePeriod(w, r)
		return
	}
	http.NotFound(w, r)
}
func (h *HTTPFeaturesHandler) ListBuyRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	resp, err := h.market.ListBuyRequests(r.Context(), &featurespb.ListBuyRequestsRequest{BuyerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := []map[string]interface{}{}
	for _, x := range resp.BuyRequests {
		out = append(out, buyRequestMap(x))
	}
	writeJSON(w, 200, out)
}
func (h *HTTPFeaturesHandler) ListReceivedBuyRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	resp, err := h.market.ListReceivedBuyRequests(r.Context(), &featurespb.ListReceivedBuyRequestsRequest{SellerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := []map[string]interface{}{}
	for _, x := range resp.BuyRequests {
		out = append(out, buyRequestMap(x))
	}
	writeJSON(w, 200, out)
}
func (h *HTTPFeaturesHandler) SendBuyRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/buy-requests/store/")
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	body := map[string]interface{}{}
	if err = decodeBody(r, &body); err != nil {
		writeValidationError(w, "request body is required")
		return
	}
	req := &featurespb.SendBuyRequestRequest{FeatureId: id, BuyerId: user.UserID, Note: stringValue(body["note"]), PricePsc: numberString(body["price_psc"]), PriceIrr: numberString(body["price_irr"])}
	resp, err := h.market.SendBuyRequest(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, buyRequestMap(resp))
}
func (h *HTTPFeaturesHandler) AcceptBuyRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/buy-requests/accept/")
	if err != nil {
		writeError(w, 400, "invalid buy request ID")
		return
	}
	resp, err := h.market.AcceptBuyRequest(r.Context(), &featurespb.AcceptBuyRequestRequest{RequestId: id, SellerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, buyRequestMap(resp))
}
func (h *HTTPFeaturesHandler) RejectBuyRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/buy-requests/reject/")
	if err != nil {
		writeError(w, 400, "invalid buy request ID")
		return
	}
	_, err = h.market.RejectBuyRequest(r.Context(), &featurespb.RejectBuyRequestRequest{RequestId: id, SellerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(200)
}
func (h *HTTPFeaturesHandler) DeleteBuyRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/buy-requests/delete/")
	if err != nil {
		writeError(w, 400, "invalid buy request ID")
		return
	}
	_, err = h.market.DeleteBuyRequest(r.Context(), &featurespb.DeleteBuyRequestRequest{RequestId: id, BuyerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *HTTPFeaturesHandler) UpdateGracePeriod(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/buy-requests/add-grace-period/")
	if err != nil {
		writeError(w, 400, "invalid buy request ID")
		return
	}
	body := map[string]interface{}{}
	if err = decodeBody(r, &body); err != nil {
		writeValidationError(w, "request body is required")
		return
	}
	days, ok := int32Value(body["grace_period"])
	if !ok || days < 1 || days > 30 {
		writeValidationError(w, "grace_period must be between 1 and 30")
		return
	}
	_, err = h.market.UpdateGracePeriod(r.Context(), &featurespb.UpdateGracePeriodRequest{RequestId: id, SellerId: user.UserID, GracePeriodDays: days})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{})
}
func (h *HTTPFeaturesHandler) HandleSellRequestsRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sell-requests"), "/")
	if path == "" && r.Method == http.MethodGet {
		h.ListSellRequests(w, r)
		return
	}
	if strings.HasPrefix(path, "store/") && r.Method == http.MethodPost {
		h.CreateSellRequest(w, r)
		return
	}
	if path != "" && r.Method == http.MethodDelete {
		h.DeleteSellRequest(w, r)
		return
	}
	http.NotFound(w, r)
}
func (h *HTTPFeaturesHandler) ListSellRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	resp, err := h.market.ListSellRequests(r.Context(), &featurespb.ListSellRequestsRequest{SellerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := []map[string]interface{}{}
	for _, x := range resp.SellRequests {
		out = append(out, sellRequestMap(x))
	}
	writeJSON(w, 200, out)
}
func (h *HTTPFeaturesHandler) CreateSellRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/sell-requests/store/")
	if err != nil {
		writeError(w, 400, "invalid feature ID")
		return
	}
	body := map[string]interface{}{}
	if err = decodeBody(r, &body); err != nil {
		writeValidationError(w, "request body is required")
		return
	}
	req := &featurespb.CreateSellRequestRequest{FeatureId: id, SellerId: user.UserID, PricePsc: numberString(body["price_psc"]), PriceIrr: numberString(body["price_irr"])}
	if v, ok := int32Value(body["minimum_price_percentage"]); ok {
		req.MinimumPricePercentage = v
	}
	resp, err := h.market.CreateSellRequest(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 201, sellRequestMap(resp))
}
func (h *HTTPFeaturesHandler) DeleteSellRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	id, err := idAfter(r, "/api/sell-requests/")
	if err != nil {
		writeError(w, 400, "invalid sell request ID")
		return
	}
	_, err = h.market.DeleteSellRequest(r.Context(), &featurespb.DeleteSellRequestRequest{SellRequestId: id, SellerId: user.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (h *HTTPFeaturesHandler) ListMyFeatures(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	resp, err := h.feature.ListMyFeatures(r.Context(), &featurespb.ListMyFeaturesRequest{UserId: user.UserID, Page: pageQuery(r, 1)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, x := range resp.Data {
		row := map[string]interface{}{"id": x.Id, "images": []interface{}{}}
		if x.Properties != nil {
			row["properties"] = map[string]interface{}{"id": x.Properties.Id, "price_psc": x.Properties.PricePsc, "price_irr": x.Properties.PriceIrr, "stability": x.Properties.Stability, "minimum_price_percentage": x.Properties.MinimumPricePercentage, "karbari": x.Properties.Karbari, "rgb": x.Properties.Rgb}
		}
		data = append(data, row)
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "links": map[string]interface{}{"first": resp.Links.First, "last": resp.Links.Last, "prev": resp.Links.Prev, "next": resp.Links.Next}, "meta": map[string]interface{}{"current_page": resp.Meta.CurrentPage, "path": resp.Meta.Path, "per_page": resp.Meta.PerPage}})
}
func (h *HTTPFeaturesHandler) HandleMyFeaturesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/my-features/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[1] == "add-image" && r.Method == http.MethodPost {
		h.AddMyFeatureImages(w, r)
		return
	}
	if len(parts) >= 5 && parts[1] == "remove-image" && parts[3] == "image" && r.Method == http.MethodPost {
		h.RemoveMyFeatureImage(w, r)
		return
	}
	if len(parts) >= 3 && parts[1] == "features" {
		if r.Method == http.MethodGet {
			h.GetMyFeature(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.UpdateMyFeature(w, r)
			return
		}
	}
	http.NotFound(w, r)
}
func (h *HTTPFeaturesHandler) validatedToken(w http.ResponseWriter, r *http.Request) bool {
	token := middleware.ExtractToken(r)
	if token == "" || h.auth == nil {
		writeError(w, 401, "authentication required")
		return false
	}
	resp, err := h.auth.ValidateToken(r.Context(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil || !resp.Valid {
		writeError(w, 401, "invalid or expired token")
		return false
	}
	return true
}
func (h *HTTPFeaturesHandler) myPath(w http.ResponseWriter, r *http.Request, expected string, indexes ...int) ([]uint64, bool) {
	if !h.validatedToken(w, r) {
		return nil, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/my-features/"), "/"), "/")
	if len(parts) < len(indexes)*2-1 || (expected != "" && !strings.Contains(strings.Join(parts, "/"), expected)) {
		writeError(w, 400, "invalid path format")
		return nil, false
	}
	out := make([]uint64, len(indexes))
	for i, index := range indexes {
		v, err := strconv.ParseUint(parts[index], 10, 64)
		if err != nil {
			writeError(w, 400, "invalid ID")
			return nil, false
		}
		out[i] = v
	}
	return out, true
}
func (h *HTTPFeaturesHandler) GetMyFeature(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.myPath(w, r, "/features/", 0, 2)
	if !ok {
		return
	}
	resp, err := h.feature.GetMyFeature(r.Context(), &featurespb.GetMyFeatureRequest{UserId: ids[0], FeatureId: ids[1]})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": featureMap(resp.Feature)})
}
func (h *HTTPFeaturesHandler) AddMyFeatureImages(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.myPath(w, r, "/add-image/", 0, 2)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, 400, "failed to parse multipart form")
		return
	}
	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		writeValidationError(w, "images field is required")
		return
	}
	req := &featurespb.AddMyFeatureImagesRequest{UserId: ids[0], FeatureId: ids[1]}
	for _, file := range files {
		ct := file.Header.Get("Content-Type")
		if ct != "image/png" && ct != "image/jpeg" && ct != "image/bmp" {
			writeValidationError(w, "invalid image type: must be PNG, JPG, or BMP")
			return
		}
		if file.Size > 1024*1024 {
			writeValidationError(w, "image size exceeds 1024 KB limit")
			return
		}
		f, err := file.Open()
		if err != nil {
			writeError(w, 400, "failed to read file")
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(w, 400, "failed to read file data")
			return
		}
		req.ImageData = append(req.ImageData, data)
		req.Filenames = append(req.Filenames, file.Filename)
		req.ContentTypes = append(req.ContentTypes, ct)
	}
	resp, err := h.feature.AddMyFeatureImages(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	images := []map[string]interface{}{}
	for _, img := range resp.Feature.Images {
		images = append(images, map[string]interface{}{"id": img.Id, "url": img.Url})
	}
	writeJSON(w, 200, map[string]interface{}{"data": images})
}
func (h *HTTPFeaturesHandler) RemoveMyFeatureImage(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.myPath(w, r, "/remove-image/", 0, 2, 4)
	if !ok {
		return
	}
	_, err := h.feature.RemoveMyFeatureImage(r.Context(), &featurespb.RemoveMyFeatureImageRequest{UserId: ids[0], FeatureId: ids[1], ImageId: ids[2]})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(200)
}
func (h *HTTPFeaturesHandler) UpdateMyFeature(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.myPath(w, r, "/features/", 0, 2)
	if !ok {
		return
	}
	var body struct {
		Minimum int32 `json:"minimum_price_percentage"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, 400, "request body is required")
		return
	}
	if body.Minimum < 80 {
		writeValidationError(w, "minimum_price_percentage must be at least 80")
		return
	}
	_, err := h.feature.UpdateMyFeature(r.Context(), &featurespb.UpdateMyFeatureRequest{UserId: ids[0], FeatureId: ids[1], MinimumPricePercentage: body.Minimum})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *HTTPFeaturesHandler) user(w http.ResponseWriter, r *http.Request) (*middlewareUser, bool) {
	user, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return nil, false
	}
	return &middlewareUser{UserID: user.UserID}, true
}

type middlewareUser struct{ UserID uint64 }

func idAfter(r *http.Request, prefix string) (uint64, error) {
	return strconv.ParseUint(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), 10, 64)
}
func featureID(r *http.Request) (uint64, error) {
	return strconv.ParseUint(strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/"), "/")[0], 10, 64)
}

func isFeatureTradeHistoryPath(path string) bool {
	path = strings.Trim(path, "/")
	if i := strings.Index(path, "/"); i >= 0 {
		rest := path[i+1:]
		return rest == "trade-history" || rest == "trade-history."
	}
	return false
}

func featureIDFromTradeHistoryRequest(r *http.Request) (uint64, error) {
	if v := r.PathValue("feature"); v != "" {
		return strconv.ParseUint(v, 10, 64)
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/")
	idPart := strings.Split(path, "/")[0]
	return strconv.ParseUint(idPart, 10, 64)
}
func pageQuery(r *http.Request, fallback int32) int32 {
	if value, err := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32); err == nil && value > 0 {
		return int32(value)
	}
	return fallback
}
func numberString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}
func stringValue(value interface{}) string { v, _ := value.(string); return v }
func int32Value(value interface{}) (int32, bool) {
	switch v := value.(type) {
	case float64:
		return int32(v), true
	case int32:
		return v, true
	case string:
		x, e := strconv.ParseInt(v, 10, 32)
		return int32(x), e == nil
	}
	return 0, false
}
func buildingRequest(id uint64, model string, body map[string]interface{}) *featurespb.BuildFeatureRequest {
	return &featurespb.BuildFeatureRequest{FeatureId: id, BuildingModelId: model, LaunchedSatisfaction: numberString(body["launched_satisfaction"]), Rotation: numberString(body["rotation"]), Position: stringValue(body["position"]), Information: parseBuildingInformation(body)}
}
func updateBuildingRequest(id uint64, model string, body map[string]interface{}) *featurespb.UpdateBuildingRequest {
	return &featurespb.UpdateBuildingRequest{FeatureId: id, BuildingModelId: model, LaunchedSatisfaction: numberString(body["launched_satisfaction"]), Rotation: numberString(body["rotation"]), Position: stringValue(body["position"]), Information: parseBuildingInformation(body)}
}
func featureMap(f *featurespb.Feature) map[string]interface{} {
	if f == nil {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{"id": f.Id, "owner_id": f.OwnerId, "is_hourly_profit_active": f.IsHourlyProfitActive}
	if f.Properties != nil {
		out["properties"] = propertyMap(f.Properties)
	}
	if len(f.Images) > 0 {
		images := []map[string]interface{}{}
		for _, x := range f.Images {
			images = append(images, map[string]interface{}{"id": x.Id, "url": x.Url})
		}
		out["images"] = images
	}
	if f.Seller != nil {
		out["seller"] = map[string]interface{}{"id": f.Seller.Id, "name": f.Seller.Name, "code": f.Seller.Code}
	}
	if f.Geometry != nil {
		coords := []map[string]interface{}{}
		for _, x := range f.Geometry.Coordinates {
			coords = append(coords, map[string]interface{}{"id": x.Id, "geometry_id": f.Geometry.Id, "x": x.X, "y": x.Y})
		}
		out["geometry"] = map[string]interface{}{"coordinates": coords}
	}
	if len(f.BuildingModels) > 0 {
		items := []map[string]interface{}{}
		for _, b := range f.BuildingModels {
			if b.Model == nil {
				continue
			}
			state := "completed"
			if t, e := time.Parse(time.RFC3339, b.ConstructionEndDate); e == nil && t.After(time.Now()) {
				state = "in_progress"
			}
			items = append(items, map[string]interface{}{"model_id": b.Model.Id, "name": b.Model.Name, "file": b.Model.File, "images": b.Model.Images, "construction_start_date": b.ConstructionStartDate, "construction_end_date": b.ConstructionEndDate, "rotation": b.Rotation, "position": b.Position, "status": state})
		}
		out["construction_status"] = items
	}
	return out
}
func listFeatureMap(f *featurespb.Feature, owned, buildings bool) map[string]interface{} {
	out := map[string]interface{}{"id": f.Id, "owner": f.OwnerId}
	if f.Properties != nil {
		out["properties"] = map[string]interface{}{"id": f.Properties.Id, "feature_id": f.Id, "rgb": f.Properties.Rgb}
	}
	if f.Geometry != nil {
		coords := []map[string]interface{}{}
		for _, x := range f.Geometry.Coordinates {
			coords = append(coords, map[string]interface{}{"id": x.Id, "geometry_id": f.Geometry.Id, "x": x.X, "y": x.Y})
		}
		out["geometry"] = map[string]interface{}{"feature_id": f.Id, "coordinates": coords}
	}
	if owned {
		out["is_owned_by_auth_user"] = f.IsOwnedByAuthUser
	}
	return out
}
func propertyMap(p *featurespb.FeatureProperties) map[string]interface{} {
	return map[string]interface{}{"id": p.Id, "address": p.Address, "density": p.Density, "stability": p.Stability, "price_psc": p.PricePsc, "price_irr": p.PriceIrr, "minimum_price_percentage": p.MinimumPricePercentage, "rgb": p.Rgb, "karbari": p.Karbari, "owner": p.Owner, "label": p.Label, "area": p.Area}
}
func buyRequestMap(x *featurespb.BuyRequestResponse) map[string]interface{} {
	out := map[string]interface{}{"id": x.Id, "feature_id": x.FeatureId, "status": x.Status, "note": x.Note, "price_psc": x.PricePsc, "price_irr": x.PriceIrr, "created_at": x.CreatedAt}
	if x.Buyer != nil {
		out["buyer"] = map[string]interface{}{"id": x.Buyer.Id, "code": x.Buyer.Code, "profile_photo": x.Buyer.ProfilePhoto}
	}
	if x.Seller != nil {
		out["seller"] = map[string]interface{}{"id": x.Seller.Id, "code": x.Seller.Code}
	}
	if x.FeatureProperties != nil {
		out["feature_properties"] = propertyMap(x.FeatureProperties)
	}
	return out
}
func sellRequestMap(x *featurespb.SellRequestResponse) map[string]interface{} {
	out := map[string]interface{}{"id": x.Id, "feature_id": x.FeatureId, "seller_id": x.SellerId, "price_psc": x.PricePsc, "price_irr": x.PriceIrr, "status": x.Status, "created_at": x.CreatedAt}
	if x.FeatureProperties != nil {
		out["feature_properties"] = propertyMap(x.FeatureProperties)
	}
	return out
}
func paginated(data interface{}, links *featurespb.PaginationLinks, meta *featurespb.FeatureTradeHistoryPaginationMeta) map[string]interface{} {
	result := map[string]interface{}{"data": data, "links": map[string]interface{}{"first": nil, "last": nil, "prev": nil, "next": nil}, "meta": map[string]interface{}{"current_page": int32(1), "from": nil, "last_page": int32(1), "path": "", "per_page": int32(10), "to": nil, "total": int32(0)}}
	if links != nil {
		result["links"] = map[string]interface{}{"first": emptyToNil(links.First), "last": emptyToNil(links.Last), "prev": emptyToNil(links.Prev), "next": emptyToNil(links.Next)}
	}
	if meta != nil {
		m := result["meta"].(map[string]interface{})
		m["current_page"] = meta.CurrentPage
		m["last_page"] = meta.LastPage
		m["path"] = meta.Path
		m["per_page"] = meta.PerPage
		m["total"] = meta.Total
		if meta.From != nil {
			m["from"] = *meta.From
		}
		if meta.To != nil {
			m["to"] = *meta.To
		}
	}
	return result
}
