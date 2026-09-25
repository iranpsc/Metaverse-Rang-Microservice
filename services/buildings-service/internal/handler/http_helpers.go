package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"metarang/buildings-service/internal/middleware"
	"metarang/buildings-service/internal/models"
	featurespb "metarang/shared/pb/features"
	authpkg "metarang/shared/pkg/auth"
)

func effectiveHTTPMethod(r *http.Request) string {
	if r.Method != http.MethodPost {
		return r.Method
	}
	if value := r.URL.Query().Get("_method"); value != "" {
		return strings.ToUpper(strings.TrimSpace(value))
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(32 << 20)
		if r.MultipartForm != nil && len(r.MultipartForm.Value["_method"]) > 0 {
			return strings.ToUpper(strings.TrimSpace(r.MultipartForm.Value["_method"][0]))
		}
	} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || contentType == "" {
		_ = r.ParseForm()
		if value := r.PostForm.Get("_method"); value != "" {
			return strings.ToUpper(strings.TrimSpace(value))
		}
	}
	return r.Method
}

// requestHasBody reports whether the request may carry a body.
// ContentLength -1 (chunked / unset) is common for API clients and must not be treated as empty.
func requestHasBody(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	if r.ContentLength == 0 {
		return false
	}
	return r.ContentLength > 0 || r.ContentLength < 0
}

func decodeBody(r *http.Request, into interface{}) error {
	if !requestHasBody(r) {
		return io.EOF
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
		return decodeFormValues(r.Form, into)
	}
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return err
		}
		return decodeFormValues(r.Form, into)
	}
	return json.NewDecoder(r.Body).Decode(into)
}

func decodeFormValues(values url.Values, into interface{}) error {
	raw := make(map[string]interface{}, len(values))
	for key, value := range values {
		if len(value) > 0 {
			raw[key] = value[0]
		}
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, into)
}

func parseIndexedArray(query url.Values, name string) []string {
	type entry struct {
		index int
		value string
	}
	entries := []entry{}
	prefix := name + "["
	for key, values := range query {
		if len(values) == 0 || !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, "]") {
			continue
		}
		index, err := strconv.Atoi(key[len(prefix) : len(key)-1])
		if err == nil {
			entries = append(entries, entry{index, values[0]})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].index < entries[j].index })
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.value
	}
	return result
}

func parseJSONString(raw string) interface{} {
	if raw == "" {
		return nil
	}
	var result interface{}
	if json.Unmarshal([]byte(raw), &result) != nil {
		return raw
	}
	return result
}

func optionalString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func emptyToNil(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func optionalFloat64(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func toProtoCitizenChartPoints(points []models.CitizenChartPoint) []*featurespb.CitizenChartPoint {
	out := make([]*featurespb.CitizenChartPoint, len(points))
	for i, p := range points {
		out[i] = &featurespb.CitizenChartPoint{Karbari: p.Karbari, Label: p.Label, Amount: p.Amount}
	}
	return out
}

func citizenChartPointsJSON(points []*featurespb.CitizenChartPoint) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(points))
	for _, p := range points {
		if p == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"karbari": p.Karbari,
			"label":   p.Label,
			"amount":  p.Amount,
		})
	}
	return out
}

func citizenImagesJSON(images []*featurespb.Image) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(images))
	for _, image := range images {
		if image == nil {
			continue
		}
		out = append(out, map[string]interface{}{"id": image.Id, "url": image.Url})
	}
	return out
}

func parseBuildingInformation(body map[string]interface{}) *featurespb.BuildingInformation {
	source := body
	hasFlat := false
	for _, key := range []string{"activity_line", "name", "address", "postal_code", "website", "description"} {
		if _, ok := body[key]; ok {
			hasFlat = true
			break
		}
	}
	if !hasFlat {
		if nested, ok := body["information"].(map[string]interface{}); ok {
			source = nested
		}
	}
	get := func(key string) string { value, _ := source[key].(string); return value }
	info := &featurespb.BuildingInformation{
		ActivityLine: get("activity_line"),
		Name:         get("name"),
		Address:      get("address"),
		PostalCode:   get("postal_code"),
		Website:      get("website"),
		Description:  get("description"),
	}
	if info.ActivityLine == "" && info.Name == "" && info.Address == "" && info.PostalCode == "" && info.Website == "" && info.Description == "" {
		return nil
	}
	return info
}

func buildingInformationMap(info *featurespb.BuildingInformation) map[string]interface{} {
	if info == nil {
		return map[string]interface{}{}
	}
	result := map[string]interface{}{}
	if info.ActivityLine != "" {
		result["activity_line"] = info.ActivityLine
	}
	if info.Name != "" {
		result["name"] = info.Name
	}
	if info.Address != "" {
		result["address"] = info.Address
	}
	if info.PostalCode != "" {
		result["postal_code"] = info.PostalCode
	}
	if info.Website != "" {
		result["website"] = info.Website
	}
	if info.Description != "" {
		result["description"] = info.Description
	}
	return result
}

func pageQuery(r *http.Request, fallback int32) int32 {
	if value, err := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32); err == nil && value > 0 {
		return int32(value)
	}
	return fallback
}

func featureID(r *http.Request) (uint64, error) {
	return strconv.ParseUint(strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/"), "/")[0], 10, 64)
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

func stringValue(value interface{}) string {
	v, _ := value.(string)
	return v
}

func buildPlacementFromBody(body map[string]interface{}) (launched, rotation, position string, info *featurespb.BuildingInformation) {
	return numberString(body["launched_satisfaction"]),
		numberString(body["rotation"]),
		stringValue(body["position"]),
		parseBuildingInformation(body)
}

func buildingRequest(id uint64, model string, body map[string]interface{}) *featurespb.BuildFeatureRequest {
	launched, rotation, position, info := buildPlacementFromBody(body)
	return &featurespb.BuildFeatureRequest{
		FeatureId:            id,
		BuildingModelId:      model,
		LaunchedSatisfaction: launched,
		Rotation:             rotation,
		Position:             position,
		Information:          info,
	}
}

func updateBuildingRequest(id uint64, model string, body map[string]interface{}) *featurespb.UpdateBuildingRequest {
	launched, rotation, position, info := buildPlacementFromBody(body)
	return &featurespb.UpdateBuildingRequest{
		FeatureId:            id,
		BuildingModelId:      model,
		LaunchedSatisfaction: launched,
		Rotation:             rotation,
		Position:             position,
		Information:          info,
	}
}

func buildingModelsMap(featureID uint64, buildings []*featurespb.Building) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(buildings))
	for _, b := range buildings {
		if b == nil || b.Model == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":                    b.Model.Id,
			"model_id":              b.Model.ModelId,
			"name":                  b.Model.Name,
			"sku":                   b.Model.Sku,
			"images":                parseJSONString(b.Model.Images),
			"attributes":            parseJSONString(b.Model.Attributes),
			"file":                  parseJSONString(b.Model.File),
			"required_satisfaction": b.Model.RequiredSatisfaction,
			"building": map[string]interface{}{
				"model_id":                b.Model.ModelId,
				"feature_id":              featureID,
				"construction_start_date": b.ConstructionStartDate,
				"construction_end_date":   b.ConstructionEndDate,
				"launched_satisfaction":   b.LaunchedSatisfaction,
				"information":             parseJSONString(b.Information),
				"rotation":                b.Rotation,
				"position":                b.Position,
				"bubble_diameter":         b.BubbleDiameter,
			},
		})
	}
	return out
}

func paginated(data interface{}, links *featurespb.PaginationLinks, meta *featurespb.FeatureTradeHistoryPaginationMeta) map[string]interface{} {
	result := map[string]interface{}{
		"data":  data,
		"links": map[string]interface{}{"first": nil, "last": nil, "prev": nil, "next": nil},
		"meta":  map[string]interface{}{"current_page": int32(1), "from": nil, "last_page": int32(1), "path": "", "per_page": int32(10), "to": nil, "total": int32(0)},
	}
	if links != nil {
		result["links"] = map[string]interface{}{
			"first": emptyToNil(links.First),
			"last":  emptyToNil(links.Last),
			"prev":  emptyToNil(links.Prev),
			"next":  emptyToNil(links.Next),
		}
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

func requireUser(w http.ResponseWriter, r *http.Request) (*authpkg.UserContext, bool) {
	user, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return nil, false
	}
	if middleware.RejectCookieCSRF(r) {
		writeError(w, http.StatusForbidden, "csrf token required")
		return nil, false
	}
	return user, true
}
