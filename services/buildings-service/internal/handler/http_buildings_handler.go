package handler

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/buildings-service/internal/constants"
	authpb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
)

// HTTPBuildingsHandler exposes building RPC handlers over HTTP.
type HTTPBuildingsHandler struct {
	building BuildingHTTPAPI
}

func NewHTTPBuildingsHandler(building BuildingHTTPAPI) *HTTPBuildingsHandler {
	return &HTTPBuildingsHandler{building: building}
}

func (h *HTTPBuildingsHandler) HandleFeaturesBuildRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/features/"), "/")
	if path == "buildings/completed" {
		h.ListCompletedBuildings(w, r)
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
	http.NotFound(w, r)
}

func (h *HTTPBuildingsHandler) BuildPackage(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireUser(w, r); !ok {
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
		data = append(data, map[string]interface{}{
			"id": m.Id, "model_id": m.ModelId, "name": m.Name, "sku": m.Sku,
			"images": parseJSONString(m.Images), "attributes": parseJSONString(m.Attributes),
			"file": parseJSONString(m.File), "required_satisfaction": m.RequiredSatisfaction,
		})
	}
	writeJSON(w, 200, map[string]interface{}{"data": data, "feature": map[string]interface{}{"coordinates": resp.Coordinates}})
}

func (h *HTTPBuildingsHandler) BuildFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if _, ok := requireUser(w, r); !ok {
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

func (h *HTTPBuildingsHandler) GetBuildings(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, 200, map[string]interface{}{"data": buildingModelsMap(id, resp.Buildings)})
}

func (h *HTTPBuildingsHandler) BuildingMutation(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireUser(w, r); !ok {
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
	case http.MethodPut:
		body := map[string]interface{}{}
		if err = decodeBody(r, &body); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		_, err = h.building.UpdateBuilding(r.Context(), updateBuildingRequest(id, model, body))
	case http.MethodPatch:
		body := map[string]interface{}{}
		if err = decodeBody(r, &body); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		info := parseBuildingInformation(body)
		if info == nil {
			writeValidationError(w, "information is required")
			return
		}
		var resp *featurespb.UpdateBuildingInformationResponse
		resp, err = h.building.UpdateBuildingInformation(r.Context(), &featurespb.UpdateBuildingInformationRequest{
			FeatureId: id, BuildingModelId: model, Information: info,
		})
		if err == nil {
			writeJSON(w, 200, map[string]interface{}{"information": buildingInformationMap(resp.Information)}, true)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{})
}

func (h *HTTPBuildingsHandler) ListCompletedBuildings(w http.ResponseWriter, r *http.Request) {
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
		data = append(data, map[string]interface{}{
			"id": x.Id, "feature_id": x.FeatureId, "feature_properties_id": x.FeaturePropertiesId,
			"length": optionalString(x.Length), "width": optionalString(x.Width),
			"density": optionalString(x.Density), "karbari": x.Karbari,
		})
	}
	writeJSON(w, 200, paginated(data, resp.Links, resp.Meta))
}

type HTTPCitizenBuildingsHandler struct {
	api     CitizenBuildingsHTTPAPI
	citizen authpb.CitizenServiceClient
}

func NewHTTPCitizenBuildingsHandler(api CitizenBuildingsHTTPAPI, citizen authpb.CitizenServiceClient) *HTTPCitizenBuildingsHandler {
	return &HTTPCitizenBuildingsHandler{api: api, citizen: citizen}
}

var privacyKeys = map[string]string{
	"a": "amoozeshi_features", "m": "maskoni_features", "t": "tejari_features",
	"g": "gardeshgari_features", "s": "fazasabz_features", "b": "behdashti_features",
	"e": "edari_features", "n": "nemayeshgah_features",
}

func (h *HTTPCitizenBuildingsHandler) resolve(w http.ResponseWriter, r *http.Request, code string) (uint64, []string, bool) {
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
		requested = append([]string(nil), constants.DefaultKarbariFilterCodes...)
	}
	allowed := FilterAllowedKarbaris(info.Privacy, requested)
	return info.UserId, allowed, true
}

// FilterAllowedKarbaris applies privacy settings to the requested karbari list.
func FilterAllowedKarbaris(privacy map[string]int32, requested []string) []string {
	if len(requested) == 0 {
		requested = append([]string(nil), constants.DefaultKarbariFilterCodes...)
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

func (h *HTTPCitizenBuildingsHandler) Handle(w http.ResponseWriter, r *http.Request, code string, rest []string) {
	id, allowed, ok := h.resolve(w, r, code)
	if !ok {
		return
	}
	period := r.URL.Query().Get("period")
	if period != "weekly" && period != "monthly" && period != "yearly" {
		period = "daily"
	}
	if len(rest) > 0 && rest[0] == "summary" {
		resp, err := h.api.GetCitizenBuildingSummary(r.Context(), &featurespb.GetCitizenBuildingSummaryRequest{
			UserId: id, AllowedKarbaris: allowed,
		})
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
		resp, err := h.api.GetCitizenBuildingChart(r.Context(), &featurespb.GetCitizenBuildingChartRequest{
			UserId: id, Period: period, AllowedKarbaris: allowed,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": citizenChartPointsJSON(resp.Data.Completed)})
		return
	}
	resp, err := h.api.ListCitizenBuildings(r.Context(), &featurespb.ListCitizenBuildingsRequest{
		UserId: id, AllowedKarbaris: allowed, Page: pageQuery(r, 1),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, x := range resp.Data {
		data = append(data, map[string]interface{}{
			"building_id":           x.BuildingId,
			"karbari":               x.Karbari,
			"area":                  optionalFloat64(x.Area),
			"visitors":              optionalFloat64(x.Visitors),
			"empty_units":           optionalFloat64(x.EmptyUnits),
			"density":               optionalFloat64(x.Density),
			"construction_end_date": optionalString(x.ConstructionEndDate),
			"images":                citizenImagesJSON(x.Images),
		})
	}
	links, meta := httpPaginationFromRequest(r, resp.Meta)
	writeJSON(w, 200, map[string]interface{}{"data": data, "links": links, "meta": meta})
}
