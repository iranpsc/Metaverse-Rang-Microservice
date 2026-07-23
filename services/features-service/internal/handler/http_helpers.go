package handler

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	featurespb "metarang/shared/pb/features"
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

func decodeBody(r *http.Request, into interface{}) error {
	if r.Body == nil || r.ContentLength == 0 {
		return io.EOF
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
		values := r.Form
		raw := make(map[string]interface{}, len(values))
		for key, value := range values {
			if len(value) > 0 {
				raw[key] = value[0]
			}
		}
		bytes, _ := json.Marshal(raw)
		return json.Unmarshal(bytes, into)
	}
	return json.NewDecoder(r.Body).Decode(into)
}

func parsePoints(query url.Values) ([]string, bool) {
	if values := parseIndexedArray(query, "points"); len(values) >= 4 {
		return values, true
	}
	if values := query["points[]"]; len(values) >= 4 {
		return values, true
	}
	if values := query["points"]; len(values) >= 4 {
		return values, true
	}
	if raw := query.Get("points"); strings.HasPrefix(raw, "[") {
		var values []string
		if json.Unmarshal([]byte(raw), &values) == nil && len(values) >= 4 {
			return values, true
		}
	}
	return nil, false
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
	for i, entry := range entries {
		result[i] = entry.value
	}
	return result
}

func contextWithClientIP(r *http.Request) context.Context {
	ip := clientIP(r)
	if ip == "" {
		return r.Context()
	}
	return metadata.NewIncomingContext(r.Context(), metadata.Pairs("x-forwarded-for", ip))
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

func optionalUint64(value *uint64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
func optionalInt64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
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

func citizenCenterJSON(center *featurespb.CitizenFeatureCenter) interface{} {
	if center == nil {
		return nil
	}
	return map[string]interface{}{"x": center.X, "y": center.Y}
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

func parseFlexibleNumber(raw string) interface{} {
	if raw == "" {
		return raw
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	return raw
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
	info := &featurespb.BuildingInformation{ActivityLine: get("activity_line"), Name: get("name"), Address: get("address"), PostalCode: get("postal_code"), Website: get("website"), Description: get("description")}
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
