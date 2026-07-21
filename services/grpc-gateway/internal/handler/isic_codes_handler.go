package handler

import (
	"net/http"
	"strconv"

	featurespb "metarang/shared/pb/features"
)

// ListIsicCodes handles GET /api/isic-codes.
func (h *FeaturesHandler) ListIsicCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	search := r.URL.Query().Get("search")

	resp, err := h.isicCodeClient.ListIsicCodes(r.Context(), &featurespb.ListIsicCodesRequest{
		Page:   page,
		Search: search,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		entry := map[string]interface{}{
			"id":       item.Id,
			"name":     item.Name,
			"code":     optionalUint64(item.Code),
			"verified": item.Verified,
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
