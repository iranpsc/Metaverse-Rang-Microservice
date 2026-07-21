package handler

import (
	"fmt"
	"net/http"
	"strconv"

	featurespb "metarang/shared/pb/features"
)

// CitizenBuildingsHandler serves public citizen building HTTP endpoints.
type CitizenBuildingsHandler struct {
	citizenBuildingsClient featurespb.CitizenBuildingsServiceClient
	citizenFeaturesHandler *CitizenFeaturesHandler
}

func NewCitizenBuildingsHandler(
	citizenFeaturesHandler *CitizenFeaturesHandler,
	citizenBuildingsClient featurespb.CitizenBuildingsServiceClient,
) *CitizenBuildingsHandler {
	return &CitizenBuildingsHandler{
		citizenFeaturesHandler: citizenFeaturesHandler,
		citizenBuildingsClient: citizenBuildingsClient,
	}
}

// Handle dispatches /api/citizen/{code}/buildings[/summary|/chart].
func (h *CitizenBuildingsHandler) Handle(w http.ResponseWriter, r *http.Request, code string, rest []string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if code == "" {
		writeError(w, http.StatusBadRequest, "citizen code is required")
		return
	}

	switch {
	case len(rest) >= 1 && rest[0] == "summary":
		h.handleSummary(w, r, code)
	case len(rest) >= 1 && rest[0] == "chart":
		h.handleChart(w, r, code)
	case len(rest) == 0 || (len(rest) == 1 && rest[0] == ""):
		h.handleList(w, r, code)
	default:
		writeError(w, http.StatusNotFound, "invalid citizen buildings endpoint")
	}
}

func (h *CitizenBuildingsHandler) handleSummary(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.citizenFeaturesHandler.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	resp, err := h.citizenBuildingsClient.GetCitizenBuildingSummary(r.Context(), &featurespb.GetCitizenBuildingSummaryRequest{
		UserId:          userID,
		AllowedKarbaris: allowed,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.citizenFeaturesHandler.locale)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		data = append(data, map[string]interface{}{
			"karbari": item.Karbari,
			"label":   item.Label,
			"count":   item.Count,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

func (h *CitizenBuildingsHandler) handleChart(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.citizenFeaturesHandler.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	resp, err := h.citizenBuildingsClient.GetCitizenBuildingChart(r.Context(), &featurespb.GetCitizenBuildingChartRequest{
		UserId:          userID,
		Period:          resolveCitizenFeaturesPeriod(r),
		AllowedKarbaris: allowed,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.citizenFeaturesHandler.locale)
		return
	}

	labels := []string{}
	completed := []int32{}
	if resp.Data != nil {
		labels = resp.Data.Labels
		completed = resp.Data.Completed
		if labels == nil {
			labels = []string{}
		}
		if completed == nil {
			completed = []int32{}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"labels":    labels,
			"completed": completed,
		},
		"period": resp.Period,
	})
}

func (h *CitizenBuildingsHandler) handleList(w http.ResponseWriter, r *http.Request, code string) {
	userID, allowed, ok := h.citizenFeaturesHandler.resolveCitizenAndKarbaris(w, r, code)
	if !ok {
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	resp, err := h.citizenBuildingsClient.ListCitizenBuildings(r.Context(), &featurespb.ListCitizenBuildingsRequest{
		UserId:          userID,
		AllowedKarbaris: allowed,
		Page:            page,
	})
	if err != nil {
		writeGRPCErrorWithLocale(w, err, h.citizenFeaturesHandler.locale)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		row := map[string]interface{}{
			"feature_properties_id": item.FeaturePropertiesId,
			"karbari":               item.Karbari,
			"area":                  citizenBuildingOptionalNumber(item.Area),
			"visitors":              citizenBuildingOptionalNumber(item.Visitors),
			"empty_units":           citizenBuildingOptionalNumber(item.EmptyUnits),
			"floors":                citizenBuildingOptionalNumber(item.Floors),
			"construction_end_date": citizenBuildingOptionalString(item.ConstructionEndDate),
		}
		data = append(data, row)
	}

	basePath := requestPath(r)
	meta := map[string]interface{}{
		"current_page": int32(1),
		"from":         nil,
		"last_page":    int32(1),
		"path":         basePath,
		"per_page":     int32(10),
		"to":           nil,
		"total":        int32(0),
	}
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
		"data":  data,
		"links": links,
		"meta":  meta,
	})
}

func citizenBuildingOptionalNumber(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func citizenBuildingOptionalString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
