package handler

import (
	"net/http"
	"strconv"
	"strings"

	"metarang/grpc-gateway/internal/middleware"
	featurespb "metarang/shared/pb/features"
)

// GetFeatureTradeHistory handles GET /api/features/{feature}/trade-history
func (h *FeaturesHandler) GetFeatureTradeHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, err := middleware.GetUserFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	featureIDStr := ExtractFeatureIDFromTradeHistoryPath(r)
	if featureIDStr == "" {
		writeError(w, http.StatusBadRequest, "feature ID is required")
		return
	}

	featureID, err := strconv.ParseUint(featureIDStr, 10, 64)
	if err != nil || featureID == 0 {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	resp, err := h.featureClient.GetFeatureTradeHistory(r.Context(), &featurespb.GetFeatureTradeHistoryRequest{
		FeatureId: featureID,
		Page:      page,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		entry := map[string]interface{}{
			"id":                optionalUint64(item.Id),
			"type":              item.Type,
			"participant_code":  optionalString(item.ParticipantCode),
			"participant_label": item.ParticipantLabel,
			"date_time":         nil,
			"price":             nil,
		}
		if item.DateTime != nil {
			entry["date_time"] = map[string]interface{}{
				"date":       item.DateTime.Date,
				"month_name": item.DateTime.MonthName,
				"year":       item.DateTime.Year,
				"time":       item.DateTime.Time,
				"formatted":  item.DateTime.Formatted,
			}
		}
		if item.Price != nil {
			entry["price"] = map[string]interface{}{
				"type":         item.Price.Type,
				"price_psc":    optionalInt64(item.Price.PricePsc),
				"price_irr":    optionalInt64(item.Price.PriceIrr),
				"color":        optionalString(item.Price.Color),
				"color_name":   optionalString(item.Price.ColorName),
				"color_amount": optionalInt64(item.Price.ColorAmount),
			}
		}
		data = append(data, entry)
	}

	links := map[string]interface{}{
		"first": nil,
		"last":  nil,
		"prev":  nil,
		"next":  nil,
	}
	if resp.Links != nil {
		links["first"] = emptyToNil(resp.Links.First)
		links["last"] = emptyToNil(resp.Links.Last)
		links["prev"] = emptyToNil(resp.Links.Prev)
		links["next"] = emptyToNil(resp.Links.Next)
	}

	meta := map[string]interface{}{
		"current_page": int32(1),
		"from":         nil,
		"last_page":    int32(1),
		"path":         "",
		"per_page":     int32(10),
		"to":           nil,
		"total":        int32(0),
	}
	if resp.Meta != nil {
		meta["current_page"] = resp.Meta.CurrentPage
		meta["last_page"] = resp.Meta.LastPage
		meta["path"] = resp.Meta.Path
		meta["per_page"] = resp.Meta.PerPage
		meta["total"] = resp.Meta.Total
		if resp.Meta.From != nil {
			meta["from"] = *resp.Meta.From
		}
		if resp.Meta.To != nil {
			meta["to"] = *resp.Meta.To
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  data,
		"links": links,
		"meta":  meta,
	})
}

func optionalUint64(v *uint64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func optionalInt64(v *int64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func optionalString(v *string) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func emptyToNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ExtractFeatureIDFromTradeHistoryPath reads the feature ID from a Go ServeMux
// path value when registered as /api/features/{feature}/trade-history, otherwise
// from /api/features/{id}/trade-history in the URL path.
func ExtractFeatureIDFromTradeHistoryPath(r *http.Request) string {
	if id := r.PathValue("feature"); id != "" {
		return id
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/")
	return FeatureIDFromTradeHistoryPath(path)
}

// IsFeatureTradeHistoryPath reports whether path is "{featureID}/trade-history".
func IsFeatureTradeHistoryPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return false
	}
	return strings.TrimRight(parts[len(parts)-1], ".") == "trade-history"
}

// FeatureIDFromTradeHistoryPath returns the feature ID segment before trade-history.
func FeatureIDFromTradeHistoryPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.TrimRight(parts[i+1], ".") == "trade-history" {
			return parts[i]
		}
	}
	return ""
}
