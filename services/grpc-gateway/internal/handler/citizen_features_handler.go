package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
)

// CitizenFeaturesHandler serves public citizen feature asset HTTP endpoints.
type CitizenFeaturesHandler struct {
	citizenClient         authpb.CitizenServiceClient
	citizenFeaturesClient featurespb.CitizenFeaturesServiceClient
	locale                string
}

func NewCitizenFeaturesHandler(authConn, featuresConn *grpc.ClientConn, locale string) *CitizenFeaturesHandler {
	return &CitizenFeaturesHandler{
		citizenClient:         authpb.NewCitizenServiceClient(authConn),
		citizenFeaturesClient: featurespb.NewCitizenFeaturesServiceClient(featuresConn),
		locale:                locale,
	}
}

var citizenKarbariPrivacyMap = map[string]string{
	"a": "amoozeshi_features",
	"m": "maskoni_features",
	"t": "tejari_features",
	"g": "gardeshgari_features",
	"s": "fazasabz_features",
	"b": "behdashti_features",
	"e": "edari_features",
	"n": "nemayeshgah_features",
}

var citizenDisplayableKarbaris = []string{"a", "m", "t", "g", "s", "b", "e", "n"}

// Handle dispatches /api/citizen/{code}/features[/summary|/chart].
func (h *CitizenFeaturesHandler) Handle(w http.ResponseWriter, r *http.Request, code string, rest []string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if code == "" {
		writeError(w, http.StatusBadRequest, "citizen code is required")
		return
	}

	switch {
	case len(rest) == 0 || (len(rest) == 1 && rest[0] == ""):
		h.handleList(w, r, code)
	case len(rest) >= 1 && rest[0] == "summary":
		h.handleSummary(w, r, code)
	case len(rest) >= 1 && rest[0] == "chart":
		h.handleChart(w, r, code)
	default:
		writeError(w, http.StatusNotFound, "invalid citizen features endpoint")
	}
}

// HandleSummary handles GET /api/citizen/{code}/features/summary
// Path param: code (citizen code)
// Query params: period, karbari
func (h *CitizenFeaturesHandler) handleSummary(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	resp, err := h.citizenFeaturesClient.GetCitizenFeatureSummary(r.Context(), &featurespb.GetCitizenFeatureSummaryRequest{
		UserId:          userID,
		Period:          resolveCitizenFeaturesPeriod(r),
		AllowedKarbaris: allowed,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.locale)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		data = append(data, map[string]interface{}{
			"karbari":       item.Karbari,
			"label":         item.Label,
			"current_count": item.CurrentCount,
			"bought_count":  item.BoughtCount,
			"sold_count":    item.SoldCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   data,
		"period": resp.Period,
	})
}

// HandleChart handles GET /api/citizen/{code}/features/chart
// Path param: code (citizen code)
// Query params: period, karbari
func (h *CitizenFeaturesHandler) handleChart(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	resp, err := h.citizenFeaturesClient.GetCitizenFeatureChart(r.Context(), &featurespb.GetCitizenFeatureChartRequest{
		UserId:          userID,
		Period:          resolveCitizenFeaturesPeriod(r),
		AllowedKarbaris: allowed,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.locale)
		return
	}

	labels := []string{}
	bought := []int32{}
	sold := []int32{}
	if resp.Data != nil {
		labels = resp.Data.Labels
		bought = resp.Data.Bought
		sold = resp.Data.Sold
		if labels == nil {
			labels = []string{}
		}
		if bought == nil {
			bought = []int32{}
		}
		if sold == nil {
			sold = []int32{}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"labels": labels,
			"bought": bought,
			"sold":   sold,
		},
	})
}

// HandleList handles GET /api/citizen/{code}/features
// Path param: code (citizen code)
// Query params: karbari, search, page, per_page
func (h *CitizenFeaturesHandler) handleList(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}
	perPage := int32(15)
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if p, err := strconv.ParseInt(perPageStr, 10, 32); err == nil && p > 0 {
			perPage = int32(p)
		}
	}

	resp, err := h.citizenFeaturesClient.ListCitizenFeatures(r.Context(), &featurespb.ListCitizenFeaturesRequest{
		UserId:          userID,
		AllowedKarbaris: allowed,
		Search:          r.URL.Query().Get("search"),
		Page:            page,
		PerPage:         perPage,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.locale)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		row := map[string]interface{}{
			"id":         item.Id,
			"vod_id":     item.VodId,
			"address":    item.Address,
			"area":       item.Area,
			"density":    item.Density,
			"karbari":    item.Karbari,
			"owner_code": item.OwnerCode,
			"price_psc":  parseFlexibleNumber(item.PricePsc),
			"price_irr":  parseFlexibleNumber(item.PriceIrr),
			"label":      item.Label,
			"center":     citizenCenterJSON(item.Center),
			"images":     citizenImagesJSON(item.Images),
		}
		data = append(data, row)
	}

	markers := make([]map[string]interface{}, 0, len(resp.MapMarkers))
	for _, marker := range resp.MapMarkers {
		markers = append(markers, map[string]interface{}{
			"id":      marker.Id,
			"center":  citizenCenterJSON(marker.Center),
			"karbari": marker.Karbari,
		})
	}

	meta := map[string]interface{}{
		"current_page": int32(1),
		"from":         nil,
		"last_page":    int32(1),
		"path":         requestPath(r),
		"per_page":     perPage,
		"to":           nil,
		"total":        int32(0),
	}
	basePath := requestPath(r)
	links := map[string]interface{}{
		"first": basePath + "?page=1",
		"last":  basePath + "?page=1",
		"prev":  nil,
		"next":  nil,
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":        data,
		"links":       links,
		"meta":        meta,
		"map_markers": markers,
	})
}

func (h *CitizenFeaturesHandler) resolveCitizenAndKarbaris(
	w http.ResponseWriter,
	r *http.Request,
	code string,
) (uint64, []string, bool) {
	info, err := h.citizenClient.GetCitizenUserInfo(r.Context(), &authpb.GetCitizenUserInfoRequest{Code: code})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			writeError(w, http.StatusNotFound, "citizen not found")
			return 0, nil, false
		}
		writeGRPCErrorWithLocale(w, err, h.locale)
		return 0, nil, false
	}

	allowed := filterAllowedKarbaris(info.Privacy, parseKarbariQuery(r.URL.Query()))
	return info.UserId, allowed, true
}

func resolveCitizenFeaturesPeriod(r *http.Request) string {
	period := r.URL.Query().Get("period")
	switch period {
	case "daily", "weekly", "monthly", "yearly":
		return period
	default:
		return "daily"
	}
}

func parseKarbariQuery(query url.Values) []string {
	if indexed := parseIndexedQueryArray(query, "karbari"); len(indexed) > 0 {
		return indexed
	}
	if vals, ok := query["karbari[]"]; ok && len(vals) > 0 {
		return vals
	}
	if vals, ok := query["karbari"]; ok && len(vals) > 0 {
		return vals
	}
	return nil
}

func filterAllowedKarbaris(privacy map[string]int32, requested []string) []string {
	candidates := requested
	if len(candidates) == 0 {
		candidates = append([]string{}, citizenDisplayableKarbaris...)
	}

	out := make([]string, 0, len(candidates))
	for _, karbari := range candidates {
		key, mapped := citizenKarbariPrivacyMap[karbari]
		if !mapped {
			// Only karbaris with an explicit privacy key are allow-listed.
			continue
		}
		val, ok := privacy[key]
		if !ok || val == 1 {
			out = append(out, karbari)
		}
	}
	return out
}

func citizenCenterJSON(center *featurespb.CitizenFeatureCenter) interface{} {
	if center == nil {
		return nil
	}
	return map[string]interface{}{
		"x": center.X,
		"y": center.Y,
	}
}

func citizenImagesJSON(images []*featurespb.Image) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(images))
	for _, img := range images {
		if img == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":  img.Id,
			"url": img.Url,
		})
	}
	return out
}

func parseFlexibleNumber(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	return raw
}
