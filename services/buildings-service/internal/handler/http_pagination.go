package handler

import (
	"net/http"
	"strconv"

	featurespb "metarang/shared/pb/features"
)

func paginationPageURL(r *http.Request, page int32) string {
	query := r.URL.Query()
	query.Set("page", strconv.FormatInt(int64(page), 10))
	return requestPath(r) + "?" + query.Encode()
}

func httpPaginationFromRequest(r *http.Request, meta *featurespb.FeatureTradeHistoryPaginationMeta) (links, metaOut map[string]interface{}) {
	basePath := requestPath(r)
	metaOut = map[string]interface{}{
		"current_page": int32(1),
		"from":         nil,
		"last_page":    int32(1),
		"path":         basePath,
		"per_page":     int32(10),
		"to":           nil,
		"total":        int32(0),
	}
	links = map[string]interface{}{
		"first": paginationPageURL(r, 1),
		"last":  paginationPageURL(r, 1),
		"prev":  nil,
		"next":  nil,
	}
	if meta == nil {
		return links, metaOut
	}
	metaOut["current_page"] = meta.CurrentPage
	metaOut["last_page"] = meta.LastPage
	metaOut["per_page"] = meta.PerPage
	metaOut["total"] = meta.Total
	metaOut["path"] = basePath
	if meta.From != nil {
		metaOut["from"] = *meta.From
	}
	if meta.To != nil {
		metaOut["to"] = *meta.To
	}
	links["first"] = paginationPageURL(r, 1)
	links["last"] = paginationPageURL(r, meta.LastPage)
	if meta.CurrentPage > 1 {
		links["prev"] = paginationPageURL(r, meta.CurrentPage-1)
	}
	if meta.CurrentPage < meta.LastPage {
		links["next"] = paginationPageURL(r, meta.CurrentPage+1)
	}
	return links, metaOut
}
