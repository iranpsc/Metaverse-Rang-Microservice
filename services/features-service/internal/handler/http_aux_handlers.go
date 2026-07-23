package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/features-service/internal/middleware"
	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
)

type profitHTTPAPI interface {
	GetHourlyProfits(context.Context, *featurespb.GetHourlyProfitsRequest) (*featurespb.HourlyProfitsResponse, error)
	GetProfitsByApplication(context.Context, *featurespb.GetProfitsByApplicationRequest) (*featurespb.ProfitsByApplicationResponse, error)
	GetSingleProfit(context.Context, *featurespb.GetSingleProfitRequest) (*featurespb.HourlyProfitResponse, error)
}
type mapHTTPAPI interface {
	ListMaps(context.Context, *featurespb.ListMapsRequest) (*featurespb.ListMapsResponse, error)
	GetMap(context.Context, *featurespb.GetMapRequest) (*featurespb.GetMapResponse, error)
	GetMapBorder(context.Context, *featurespb.GetMapRequest) (*featurespb.GetMapBorderResponse, error)
}
type isicHTTPAPI interface {
	ListIsicCodes(context.Context, *featurespb.ListIsicCodesRequest) (*featurespb.ListIsicCodesResponse, error)
}
type citizenFeaturesHTTPAPI interface {
	GetCitizenFeatureSummary(context.Context, *featurespb.GetCitizenFeatureSummaryRequest) (*featurespb.GetCitizenFeatureSummaryResponse, error)
	GetCitizenFeatureChart(context.Context, *featurespb.GetCitizenFeatureChartRequest) (*featurespb.GetCitizenFeatureChartResponse, error)
	ListCitizenFeatures(context.Context, *featurespb.ListCitizenFeaturesRequest) (*featurespb.ListCitizenFeaturesResponse, error)
}
type citizenBuildingsHTTPAPI interface {
	GetCitizenBuildingSummary(context.Context, *featurespb.GetCitizenBuildingSummaryRequest) (*featurespb.GetCitizenBuildingSummaryResponse, error)
	GetCitizenBuildingChart(context.Context, *featurespb.GetCitizenBuildingChartRequest) (*featurespb.GetCitizenBuildingChartResponse, error)
	ListCitizenBuildings(context.Context, *featurespb.ListCitizenBuildingsRequest) (*featurespb.ListCitizenBuildingsResponse, error)
}

type HTTPProfitHandler struct{ api profitHTTPAPI }

func NewHTTPProfitHandler(api profitHTTPAPI) *HTTPProfitHandler { return &HTTPProfitHandler{api} }
func (h *HTTPProfitHandler) Handle(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/hourly-profits"), "/")
	if path == "" && r.Method == http.MethodGet {
		h.list(w, r, user.UserID)
		return
	}
	if path == "" && r.Method == http.MethodPost {
		h.byApplication(w, r, user.UserID)
		return
	}
	if path != "" && r.Method == http.MethodPost {
		h.single(w, r, user.UserID)
		return
	}
	http.NotFound(w, r)
}
func (h *HTTPProfitHandler) list(w http.ResponseWriter, r *http.Request, id uint64) {
	page := pageQuery(r, 1)
	per := int32(10)
	if v := r.URL.Query().Get("per_page"); v != "" {
		if x, e := strconv.ParseInt(v, 10, 32); e == nil && x > 0 {
			per = int32(x)
		}
	}
	resp, err := h.api.GetHourlyProfits(r.Context(), &featurespb.GetHourlyProfitsRequest{UserId: id, Page: page, PageSize: per})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, x := range resp.Profits {
		data = append(data, profitMap(x))
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "links": buildSimplePaginationLinks(r, page, resp.HasMore), "additional": map[string]interface{}{"total_maskoni_profit": resp.TotalMaskoniProfit, "total_tejari_profit": resp.TotalTejariProfit, "total_amozeshi_profit": resp.TotalAmozeshiProfit}})
}
func (h *HTTPProfitHandler) byApplication(w http.ResponseWriter, r *http.Request, id uint64) {
	var body struct {
		Karbari string `json:"karbari"`
	}
	if err := decodeBody(r, &body); err != nil || body.Karbari == "" {
		writeValidationError(w, "karbari field is required")
		return
	}
	if body.Karbari != "m" && body.Karbari != "t" && body.Karbari != "a" {
		writeValidationError(w, "karbari must be one of: m, t, a")
		return
	}
	_, err := h.api.GetProfitsByApplication(r.Context(), &featurespb.GetProfitsByApplicationRequest{UserId: id, Karbari: body.Karbari})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write([]byte("[]"))
}
func (h *HTTPProfitHandler) single(w http.ResponseWriter, r *http.Request, id uint64) {
	profit, err := idAfter(r, "/api/hourly-profits/")
	if err != nil {
		writeError(w, 400, "invalid profit ID")
		return
	}
	resp, err := h.api.GetSingleProfit(r.Context(), &featurespb.GetSingleProfitRequest{ProfitId: profit, UserId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": profitMap(resp.Profit)})
}
func profitMap(x *featurespb.HourlyProfit) map[string]interface{} {
	return map[string]interface{}{"id": x.Id, "user_id": x.UserId, "feature_db_id": x.FeatureDbId, "feature_id": x.PropertiesId, "is_active": x.IsActive, "amount": x.Amount, "karbari": x.Karbari, "dead_line": x.DeadLine}
}

type HTTPMapsHandler struct{ api mapHTTPAPI }

func NewHTTPMapsHandler(api mapHTTPAPI) *HTTPMapsHandler { return &HTTPMapsHandler{api} }
func (h *HTTPMapsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/maps"), "/")
	if path == "" {
		resp, err := h.api.ListMaps(r.Context(), &featurespb.ListMapsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		data := []map[string]interface{}{}
		for _, m := range resp.Maps {
			data = append(data, mapMap(m))
		}
		writeJSON(w, 200, data)
		return
	}
	parts := strings.Split(path, "/")
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid map ID")
		return
	}
	if len(parts) > 1 && parts[1] == "border" {
		resp, err := h.api.GetMapBorder(r.Context(), &featurespb.GetMapRequest{MapId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"border_coordinates": parseJSONString(resp.Data.BorderCoordinates)}})
		return
	}
	resp, err := h.api.GetMap(r.Context(), &featurespb.GetMapRequest{MapId: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, mapMap(resp.Map))
}
func mapMap(m *featurespb.Map) map[string]interface{} {
	return map[string]interface{}{"id": m.Id, "name": m.Name, "color": m.Color, "central_point_coordinates": parseJSONString(m.CentralPointCoordinates), "sold_features_percentage": m.SoldFeaturesPercentage}
}

type HTTPIsicCodesHandler struct{ api isicHTTPAPI }

func NewHTTPIsicCodesHandler(api isicHTTPAPI) *HTTPIsicCodesHandler {
	return &HTTPIsicCodesHandler{api}
}
func (h *HTTPIsicCodesHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListIsicCodes(r.Context(), &featurespb.ListIsicCodesRequest{Page: pageQuery(r, 1), Search: r.URL.Query().Get("search")})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := []map[string]interface{}{}
	for _, x := range resp.Data {
		data = append(data, map[string]interface{}{"id": x.Id, "name": x.Name, "code": optionalUint64(x.Code), "verified": x.Verified})
	}
	writeJSON(w, 200, paginated(data, resp.Links, resp.Meta))
}

type HTTPCitizenFeaturesHandler struct {
	api     citizenFeaturesHTTPAPI
	citizen authpb.CitizenServiceClient
}
type HTTPCitizenBuildingsHandler struct {
	api      citizenBuildingsHTTPAPI
	features *HTTPCitizenFeaturesHandler
}

func NewHTTPCitizenFeaturesHandler(api citizenFeaturesHTTPAPI, c authpb.CitizenServiceClient) *HTTPCitizenFeaturesHandler {
	return &HTTPCitizenFeaturesHandler{api, c}
}
func NewHTTPCitizenBuildingsHandler(api citizenBuildingsHTTPAPI, f *HTTPCitizenFeaturesHandler) *HTTPCitizenBuildingsHandler {
	return &HTTPCitizenBuildingsHandler{api, f}
}

var privacyKeys = map[string]string{"a": "amoozeshi_features", "m": "maskoni_features", "t": "tejari_features", "g": "gardeshgari_features", "s": "fazasabz_features", "b": "behdashti_features", "e": "edari_features", "n": "nemayeshgah_features"}

func (h *HTTPCitizenFeaturesHandler) resolve(w http.ResponseWriter, r *http.Request, code string) (uint64, []string, bool) {
	if h.citizen == nil {
		writeError(w, 503, "service temporarily unavailable")
		return 0, nil, false
	}
	info, err := h.citizen.GetCitizenUserInfo(r.Context(), &authpb.GetCitizenUserInfoRequest{Code: code})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			writeError(w, http.StatusNotFound, "citizen not found")
			return 0, nil, false
		}
		writeGRPCError(w, err)
		return 0, nil, false
	}
	requested := parseIndexedArray(r.URL.Query(), "karbari")
	if len(requested) == 0 {
		requested = r.URL.Query()["karbari[]"]
	}
	if len(requested) == 0 {
		requested = r.URL.Query()["karbari"]
	}
	if len(requested) == 0 {
		requested = []string{"a", "m", "t", "g", "s", "b", "e", "n"}
	}
	allowed := filterAllowedKarbaris(info.Privacy, requested)
	return info.UserId, allowed, true
}

func filterAllowedKarbaris(privacy map[string]int32, requested []string) []string {
	if len(requested) == 0 {
		requested = []string{"a", "m", "t", "g", "s", "b", "e", "n"}
	}
	allowed := make([]string, 0, len(requested))
	for _, k := range requested {
		key, ok := privacyKeys[k]
		if !ok {
			continue
		}
		if privacy == nil {
			allowed = append(allowed, k)
			continue
		}
		if value, exists := privacy[key]; !exists || value == 1 {
			allowed = append(allowed, k)
		}
	}
	return allowed
}

func (h *HTTPCitizenFeaturesHandler) Handle(w http.ResponseWriter, r *http.Request, code string, rest []string) {
	id, allowed, ok := h.resolve(w, r, code)
	if !ok {
		return
	}
	period := r.URL.Query().Get("period")
	if period != "weekly" && period != "monthly" && period != "yearly" {
		period = "daily"
	}
	if len(rest) > 0 && rest[0] == "summary" {
		resp, err := h.api.GetCitizenFeatureSummary(r.Context(), &featurespb.GetCitizenFeatureSummaryRequest{UserId: id, Period: period, AllowedKarbaris: allowed})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		data := []map[string]interface{}{}
		for _, x := range resp.Data {
			data = append(data, map[string]interface{}{"karbari": x.Karbari, "label": x.Label, "current_count": x.CurrentCount, "bought_count": x.BoughtCount, "sold_count": x.SoldCount})
		}
		writeJSON(w, 200, map[string]interface{}{"data": data, "period": resp.Period})
		return
	}
	if len(rest) > 0 && rest[0] == "chart" {
		resp, err := h.api.GetCitizenFeatureChart(r.Context(), &featurespb.GetCitizenFeatureChartRequest{UserId: id, Period: period, AllowedKarbaris: allowed})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"labels": resp.Data.Labels, "bought": resp.Data.Bought, "sold": resp.Data.Sold}})
		return
	}

	perPage := int32(15)
	if v := r.URL.Query().Get("per_page"); v != "" {
		if x, e := strconv.ParseInt(v, 10, 32); e == nil && x > 0 {
			perPage = int32(x)
		}
	}
	page := pageQuery(r, 1)
	resp, err := h.api.ListCitizenFeatures(r.Context(), &featurespb.ListCitizenFeaturesRequest{
		UserId: id, AllowedKarbaris: allowed, Search: r.URL.Query().Get("search"), Page: page, PerPage: perPage,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, x := range resp.Data {
		data = append(data, map[string]interface{}{
			"id": x.Id, "vod_id": x.VodId, "address": x.Address, "area": x.Area, "density": x.Density,
			"karbari": x.Karbari, "owner_code": x.OwnerCode,
			"price_psc": parseFlexibleNumber(x.PricePsc), "price_irr": parseFlexibleNumber(x.PriceIrr),
			"label": x.Label, "center": citizenCenterJSON(x.Center), "images": citizenImagesJSON(x.Images),
		})
	}
	markers := make([]map[string]interface{}, 0, len(resp.MapMarkers))
	for _, marker := range resp.MapMarkers {
		markers = append(markers, map[string]interface{}{
			"id": marker.Id, "center": citizenCenterJSON(marker.Center), "karbari": marker.Karbari,
		})
	}
	basePath := requestPath(r)
	meta := map[string]interface{}{
		"current_page": int32(1), "from": nil, "last_page": int32(1), "path": basePath,
		"per_page": perPage, "to": nil, "total": int32(0),
	}
	links := map[string]interface{}{
		"first": basePath + "?page=1", "last": basePath + "?page=1", "prev": nil, "next": nil,
	}
	if resp.Meta != nil {
		meta["current_page"] = resp.Meta.CurrentPage
		meta["last_page"] = resp.Meta.LastPage
		meta["per_page"] = resp.Meta.PerPage
		meta["total"] = resp.Meta.Total
		meta["path"] = basePath
		if resp.Meta.From != nil {
			meta["from"] = *resp.Meta.From
		}
		if resp.Meta.To != nil {
			meta["to"] = *resp.Meta.To
		}
		links["first"] = basePath + "?page=1"
		links["last"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.LastPage)
		if resp.Meta.CurrentPage > 1 {
			links["prev"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.CurrentPage-1)
		}
		if resp.Meta.CurrentPage < resp.Meta.LastPage {
			links["next"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.CurrentPage+1)
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "links": links, "meta": meta, "map_markers": markers})
}
func (h *HTTPCitizenBuildingsHandler) Handle(w http.ResponseWriter, r *http.Request, code string, rest []string) {
	id, allowed, ok := h.features.resolve(w, r, code)
	if !ok {
		return
	}
	period := r.URL.Query().Get("period")
	if period != "weekly" && period != "monthly" && period != "yearly" {
		period = "daily"
	}
	if len(rest) > 0 && rest[0] == "summary" {
		resp, err := h.api.GetCitizenBuildingSummary(r.Context(), &featurespb.GetCitizenBuildingSummaryRequest{UserId: id, AllowedKarbaris: allowed})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		data := []map[string]interface{}{}
		for _, x := range resp.Data {
			data = append(data, map[string]interface{}{"karbari": x.Karbari, "label": x.Label, "count": x.Count})
		}
		writeJSON(w, 200, map[string]interface{}{"data": data})
		return
	}
	if len(rest) > 0 && rest[0] == "chart" {
		resp, err := h.api.GetCitizenBuildingChart(r.Context(), &featurespb.GetCitizenBuildingChartRequest{UserId: id, Period: period, AllowedKarbaris: allowed})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"labels": resp.Data.Labels, "completed": resp.Data.Completed}, "period": resp.Period})
		return
	}
	resp, err := h.api.ListCitizenBuildings(r.Context(), &featurespb.ListCitizenBuildingsRequest{UserId: id, AllowedKarbaris: allowed, Page: pageQuery(r, 1)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, x := range resp.Data {
		data = append(data, map[string]interface{}{
			"feature_properties_id": x.FeaturePropertiesId,
			"karbari":               x.Karbari,
			"area":                  optionalFloat64(x.Area),
			"visitors":              optionalFloat64(x.Visitors),
			"empty_units":           optionalFloat64(x.EmptyUnits),
			"floors":                optionalFloat64(x.Floors),
			"construction_end_date": optionalString(x.ConstructionEndDate),
		})
	}
	basePath := requestPath(r)
	meta := map[string]interface{}{
		"current_page": int32(1), "from": nil, "last_page": int32(1), "path": basePath,
		"per_page": int32(10), "to": nil, "total": int32(0),
	}
	links := map[string]interface{}{
		"first": basePath + "?page=1", "last": basePath + "?page=1", "prev": nil, "next": nil,
	}
	if resp.Meta != nil {
		meta["current_page"] = resp.Meta.CurrentPage
		meta["last_page"] = resp.Meta.LastPage
		meta["per_page"] = resp.Meta.PerPage
		meta["total"] = resp.Meta.Total
		meta["path"] = basePath
		if resp.Meta.From != nil {
			meta["from"] = *resp.Meta.From
		}
		if resp.Meta.To != nil {
			meta["to"] = *resp.Meta.To
		}
		links["first"] = basePath + "?page=1"
		links["last"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.LastPage)
		if resp.Meta.CurrentPage > 1 {
			links["prev"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.CurrentPage-1)
		}
		if resp.Meta.CurrentPage < resp.Meta.LastPage {
			links["next"] = fmt.Sprintf("%s?page=%d", basePath, resp.Meta.CurrentPage+1)
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "links": links, "meta": meta})
}
