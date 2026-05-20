package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	"metargb/grpc-gateway/internal/middleware"
	pb "metargb/shared/pb/auth"
	featurespb "metargb/shared/pb/features"
)

type FeaturesHandler struct {
	featureClient     featurespb.FeatureServiceClient
	marketplaceClient featurespb.FeatureMarketplaceServiceClient
	profitClient      featurespb.FeatureProfitServiceClient
	buildingClient    featurespb.BuildingServiceClient
	authClient        pb.AuthServiceClient
	locale            string
}

func NewFeaturesHandler(featuresConn, authConn *grpc.ClientConn, locale string) *FeaturesHandler {
	return &FeaturesHandler{
		featureClient:     featurespb.NewFeatureServiceClient(featuresConn),
		marketplaceClient: featurespb.NewFeatureMarketplaceServiceClient(featuresConn),
		profitClient:      featurespb.NewFeatureProfitServiceClient(featuresConn),
		buildingClient:    featurespb.NewBuildingServiceClient(featuresConn),
		authClient:        pb.NewAuthServiceClient(authConn),
		locale:            locale,
	}
}

// ListFeatures handles GET /api/features
// Query params: points (array), load_buildings (bool), user_features_location (bool)
// Optional authentication - if token provided, includes is_owned_by_auth_user
func (h *FeaturesHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters (Laravel: points[0]=x,y&points[1]=..., points[]=..., or JSON array)
	query := r.URL.Query()
	points, ok := parsePointsFromQuery(query)
	if !ok {
		writeValidationErrorWithLocale(w, "points parameter is required", h.locale)
		return
	}

	// Validate points (min:4 per documentation)
	if len(points) < 4 {
		writeValidationErrorWithLocale(w, "points array must have at least 4 elements", h.locale)
		return
	}

	// Parse load_buildings
	loadBuildings := false
	if lb := r.URL.Query().Get("load_buildings"); lb == "true" || lb == "1" {
		loadBuildings = true
	}

	// Parse user_features_location (reserved, currently ignored)
	userFeaturesLocation := false
	if ufl := r.URL.Query().Get("user_features_location"); ufl == "true" || ufl == "1" {
		userFeaturesLocation = true
	}

	// Extract authenticated user ID from context (optional - set by optionalAuthMiddleware)
	var authUserID uint64
	userCtx, err := middleware.GetUserFromRequest(r)
	if err == nil {
		authUserID = userCtx.UserID
	}

	// Build gRPC request
	grpcReq := &featurespb.ListFeaturesRequest{
		Points:               points,
		LoadBuildings:        loadBuildings,
		UserFeaturesLocation: userFeaturesLocation,
	}

	// Call gRPC service
	resp, err := h.featureClient.ListFeatures(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build response matching Laravel FeatureResource format
	features := make([]map[string]interface{}, 0, len(resp.Features))
	for _, feature := range resp.Features {
		featureMap := map[string]interface{}{
			"id":    feature.Id,
			"owner": feature.OwnerId,
		}

		// Add properties
		if feature.Properties != nil {
			featureMap["properties"] = map[string]interface{}{
				"id":         feature.Properties.Id,
				"feature_id": feature.Id,
				"rgb":        feature.Properties.Rgb,
			}
		}

		// Add geometry with coordinates
		if feature.Geometry != nil {
			coordinates := make([]map[string]interface{}, 0, len(feature.Geometry.Coordinates))
			for _, coord := range feature.Geometry.Coordinates {
				coordinates = append(coordinates, map[string]interface{}{
					"id":          coord.Id,
					"geometry_id": feature.Geometry.Id,
					"x":           coord.X,
					"y":           coord.Y,
				})
			}
			featureMap["geometry"] = map[string]interface{}{
				"coordinates": coordinates,
			}
		}

		// Add building models if loaded
		if len(feature.BuildingModels) > 0 {
			buildings := make([]map[string]interface{}, 0, len(feature.BuildingModels))
			for _, building := range feature.BuildingModels {
				buildingMap := map[string]interface{}{
					"model_id":                building.Model.Id,
					"name":                    building.Model.Name,
					"file":                    building.Model.File,
					"images":                  building.Model.Images,
					"construction_start_date": building.ConstructionStartDate,
					"construction_end_date":   building.ConstructionEndDate,
					"rotation":                building.Rotation,
					"position":                building.Position,
				}
				buildings = append(buildings, buildingMap)
			}
			featureMap["building_models"] = buildings
		}

		// Add is_owned_by_auth_user if authenticated
		if authUserID > 0 {
			featureMap["is_owned_by_auth_user"] = feature.IsOwnedByAuthUser
		}

		features = append(features, featureMap)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": features})
}

// GetFeature handles GET /api/features/{feature}
// Path param: feature (feature ID)
// Optional authentication
func (h *FeaturesHandler) GetFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract feature ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/features/")
	path = strings.TrimSuffix(path, "/")
	featureID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	// Build gRPC request
	grpcReq := &featurespb.GetFeatureRequest{
		FeatureId: featureID,
	}

	// Call gRPC service
	resp, err := h.featureClient.GetFeature(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	feature := resp.Feature

	// Build response matching Laravel FeatureResource format
	featureMap := map[string]interface{}{
		"id":       feature.Id,
		"owner_id": feature.OwnerId,
	}

	// Add properties
	if feature.Properties != nil {
		featureMap["properties"] = map[string]interface{}{
			"id":                       feature.Properties.Id,
			"address":                  feature.Properties.Address,
			"density":                  feature.Properties.Density,
			"stability":                feature.Properties.Stability,
			"price_psc":                feature.Properties.PricePsc,
			"price_irr":                feature.Properties.PriceIrr,
			"minimum_price_percentage": feature.Properties.MinimumPricePercentage,
			"rgb":                      feature.Properties.Rgb,
			"karbari":                  feature.Properties.Karbari,
			"owner":                    feature.Properties.Owner,
			"label":                    feature.Properties.Label,
			"area":                     feature.Properties.Area,
		}
	}

	// Add images
	if len(feature.Images) > 0 {
		images := make([]map[string]interface{}, 0, len(feature.Images))
		for _, img := range feature.Images {
			images = append(images, map[string]interface{}{
				"id":  img.Id,
				"url": img.Url,
			})
		}
		featureMap["images"] = images
	}

	// Add seller (from latest trade)
	if feature.Seller != nil {
		featureMap["seller"] = map[string]interface{}{
			"id":   feature.Seller.Id,
			"name": feature.Seller.Name,
			"code": feature.Seller.Code,
		}
	}

	// Add hourly profit status
	featureMap["is_hourly_profit_active"] = feature.IsHourlyProfitActive

	// Add geometry
	if feature.Geometry != nil {
		coordinates := make([]map[string]interface{}, 0, len(feature.Geometry.Coordinates))
		for _, coord := range feature.Geometry.Coordinates {
			coordinates = append(coordinates, map[string]interface{}{
				"id":          coord.Id,
				"geometry_id": feature.Geometry.Id,
				"x":           coord.X,
				"y":           coord.Y,
			})
		}
		featureMap["geometry"] = map[string]interface{}{
			"coordinates": coordinates,
		}
	}

	// Add building models (construction status)
	if len(feature.BuildingModels) > 0 {
		buildings := make([]map[string]interface{}, 0, len(feature.BuildingModels))
		for _, building := range feature.BuildingModels {
			// Determine construction status
			status := "completed"
			if building.ConstructionEndDate != "" {
				// Check if end_date is in the future for "in progress"
				if endDate, err := time.Parse(time.RFC3339, building.ConstructionEndDate); err == nil {
					if endDate.After(time.Now()) {
						status = "in_progress"
					}
				}
			}

			buildingMap := map[string]interface{}{
				"model_id":                building.Model.Id,
				"name":                    building.Model.Name,
				"file":                    building.Model.File,
				"images":                  building.Model.Images,
				"construction_start_date": building.ConstructionStartDate,
				"construction_end_date":   building.ConstructionEndDate,
				"rotation":                building.Rotation,
				"position":                building.Position,
				"status":                  status,
			}
			buildings = append(buildings, buildingMap)
		}
		featureMap["construction_status"] = buildings
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": featureMap})
}

// BuyFeature handles POST /api/features/buy/{feature}
// Path param: feature (feature ID)
// Requires authentication
func (h *FeaturesHandler) BuyFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	buyerID := userCtx.UserID

	// Extract feature ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/features/buy/")
	path = strings.TrimSuffix(path, "/")
	featureID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	// Build gRPC request
	grpcReq := &featurespb.BuyFeatureRequest{
		FeatureId: featureID,
		BuyerId:   buyerID,
	}

	// Call gRPC service
	resp, err := h.marketplaceClient.BuyFeature(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build response with updated feature (matching Laravel FeatureResource format)
	featureMap := map[string]interface{}{}
	if resp.Feature != nil {
		feature := resp.Feature
		featureMap["id"] = feature.Id
		featureMap["owner_id"] = feature.OwnerId

		if feature.Properties != nil {
			featureMap["properties"] = map[string]interface{}{
				"id":                       feature.Properties.Id,
				"address":                  feature.Properties.Address,
				"density":                  feature.Properties.Density,
				"stability":                feature.Properties.Stability,
				"price_psc":                feature.Properties.PricePsc,
				"price_irr":                feature.Properties.PriceIrr,
				"minimum_price_percentage": feature.Properties.MinimumPricePercentage,
				"rgb":                      feature.Properties.Rgb,
				"karbari":                  feature.Properties.Karbari,
				"owner":                    feature.Properties.Owner,
				"label":                    feature.Properties.Label,
				"area":                     feature.Properties.Area,
			}
		}

		if feature.Geometry != nil {
			coordinates := make([]map[string]interface{}, 0, len(feature.Geometry.Coordinates))
			for _, coord := range feature.Geometry.Coordinates {
				coordinates = append(coordinates, map[string]interface{}{
					"id":          coord.Id,
					"geometry_id": feature.Geometry.Id,
					"x":           coord.X,
					"y":           coord.Y,
				})
			}
			featureMap["geometry"] = map[string]interface{}{
				"coordinates": coordinates,
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": featureMap})
}

// Building Feature API Handlers (v2) - See api-docs/features-service/build_feature_api.md

// GetBuildPackage handles GET /api/v2/features/{feature}/build/package
func (h *FeaturesHandler) GetBuildPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware) - authentication required but user ID not used
	_, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/features/"), "/")
	if len(pathParts) == 0 {
		writeError(w, http.StatusBadRequest, "feature ID is required")
		return
	}
	featureID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	page := int32(1)
	if p := r.URL.Query().Get("page"); p != "" {
		if pInt, err := strconv.ParseInt(p, 10, 32); err == nil && pInt > 0 {
			page = int32(pInt)
		}
	}

	grpcReq := &featurespb.GetBuildPackageRequest{
		FeatureId: featureID,
		Page:      page,
	}

	resp, err := h.buildingClient.GetBuildPackage(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	models := make([]map[string]interface{}, 0, len(resp.Models))
	for _, model := range resp.Models {
		var images, attributes, file interface{}
		json.Unmarshal([]byte(model.Images), &images)
		json.Unmarshal([]byte(model.Attributes), &attributes)
		json.Unmarshal([]byte(model.File), &file)

		modelMap := map[string]interface{}{
			"id":                    model.Id,
			"model_id":              model.ModelId,
			"name":                  model.Name,
			"sku":                   model.Sku,
			"images":                images,
			"attributes":            attributes,
			"file":                  file,
			"required_satisfaction": model.RequiredSatisfaction,
		}
		models = append(models, modelMap)
	}

	response := map[string]interface{}{
		"data": models,
		"feature": map[string]interface{}{
			"coordinates": resp.Coordinates,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// BuildFeature handles POST /api/v2/features/{feature}/build/{buildingModel}
func (h *FeaturesHandler) BuildFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	_, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/features/"), "/")
	if len(pathParts) < 3 {
		writeError(w, http.StatusBadRequest, "feature ID and building model ID are required")
		return
	}
	featureID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}
	buildingModelID, err := strconv.ParseUint(pathParts[2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid building model ID")
		return
	}

	var reqBody map[string]interface{}
	if err := decodeRequestBody(r, &reqBody); err != nil {
		if err == io.EOF {
			writeValidationErrorWithLocale(w, "request body is required", h.locale)
		} else {
			writeValidationErrorWithLocale(w, "invalid request body", h.locale)
		}
		return
	}

	grpcReq := &featurespb.BuildFeatureRequest{
		FeatureId:       featureID,
		BuildingModelId: buildingModelID,
	}

	if launchedSatisfaction, ok := reqBody["launched_satisfaction"].(string); ok {
		grpcReq.LaunchedSatisfaction = launchedSatisfaction
	} else if ls, ok := reqBody["launched_satisfaction"].(float64); ok {
		grpcReq.LaunchedSatisfaction = strconv.FormatFloat(ls, 'f', -1, 64)
	}

	if rotation, ok := reqBody["rotation"].(string); ok {
		grpcReq.Rotation = rotation
	} else if rot, ok := reqBody["rotation"].(float64); ok {
		grpcReq.Rotation = strconv.FormatFloat(rot, 'f', -1, 64)
	}

	if position, ok := reqBody["position"].(string); ok {
		grpcReq.Position = position
	}

	if info, ok := reqBody["information"].(map[string]interface{}); ok {
		grpcReq.Information = &featurespb.BuildingInformation{}
		if activityLine, ok := info["activity_line"].(string); ok {
			grpcReq.Information.ActivityLine = activityLine
		}
		if name, ok := info["name"].(string); ok {
			grpcReq.Information.Name = name
		}
		if address, ok := info["address"].(string); ok {
			grpcReq.Information.Address = address
		}
		if postalCode, ok := info["postal_code"].(string); ok {
			grpcReq.Information.PostalCode = postalCode
		}
		if website, ok := info["website"].(string); ok {
			grpcReq.Information.Website = website
		}
		if description, ok := info["description"].(string); ok {
			grpcReq.Information.Description = description
		}
	}

	_, err = h.buildingClient.BuildFeature(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// GetBuildings handles GET /api/v2/features/{feature}/build/buildings
func (h *FeaturesHandler) GetBuildings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/features/"), "/")
	if len(pathParts) == 0 {
		writeError(w, http.StatusBadRequest, "feature ID is required")
		return
	}
	featureID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	grpcReq := &featurespb.GetBuildingsRequest{
		FeatureId: featureID,
	}

	resp, err := h.buildingClient.GetBuildings(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	buildings := make([]map[string]interface{}, 0, len(resp.Buildings))
	for _, building := range resp.Buildings {
		var modelImages, modelAttributes, modelFile interface{}
		if building.Model != nil {
			json.Unmarshal([]byte(building.Model.Images), &modelImages)
			json.Unmarshal([]byte(building.Model.Attributes), &modelAttributes)
			json.Unmarshal([]byte(building.Model.File), &modelFile)

			buildingMap := map[string]interface{}{
				"id":                    building.Model.Id,
				"model_id":              building.Model.ModelId,
				"name":                  building.Model.Name,
				"sku":                   building.Model.Sku,
				"images":                modelImages,
				"attributes":            modelAttributes,
				"file":                  modelFile,
				"required_satisfaction": building.Model.RequiredSatisfaction,
				"building": map[string]interface{}{
					"model_id":                building.Model.ModelId,
					"feature_id":              featureID,
					"construction_start_date": building.ConstructionStartDate,
					"construction_end_date":   building.ConstructionEndDate,
					"launched_satisfaction":   building.LaunchedSatisfaction,
					"information":             building.Information,
					"rotation":                building.Rotation,
					"position":                building.Position,
					"bubble_diameter":         building.BubbleDiameter,
				},
			}
			buildings = append(buildings, buildingMap)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildings})
}

// UpdateBuilding handles PUT /api/v2/features/{feature}/build/buildings/{buildingModel}
func (h *FeaturesHandler) UpdateBuilding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	_, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/features/"), "/")
	if len(pathParts) < 4 {
		writeError(w, http.StatusBadRequest, "feature ID and building model ID are required")
		return
	}
	featureID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}
	buildingModelID, err := strconv.ParseUint(pathParts[3], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid building model ID")
		return
	}

	var reqBody map[string]interface{}
	if err := decodeRequestBody(r, &reqBody); err != nil {
		if err == io.EOF {
			writeValidationErrorWithLocale(w, "request body is required", h.locale)
		} else {
			writeValidationErrorWithLocale(w, "invalid request body", h.locale)
		}
		return
	}

	grpcReq := &featurespb.UpdateBuildingRequest{
		FeatureId:       featureID,
		BuildingModelId: buildingModelID,
	}

	if launchedSatisfaction, ok := reqBody["launched_satisfaction"].(string); ok {
		grpcReq.LaunchedSatisfaction = launchedSatisfaction
	} else if ls, ok := reqBody["launched_satisfaction"].(float64); ok {
		grpcReq.LaunchedSatisfaction = strconv.FormatFloat(ls, 'f', -1, 64)
	}

	if rotation, ok := reqBody["rotation"].(string); ok {
		grpcReq.Rotation = rotation
	} else if rot, ok := reqBody["rotation"].(float64); ok {
		grpcReq.Rotation = strconv.FormatFloat(rot, 'f', -1, 64)
	}

	if position, ok := reqBody["position"].(string); ok {
		grpcReq.Position = position
	}

	if info, ok := reqBody["information"].(map[string]interface{}); ok {
		grpcReq.Information = &featurespb.BuildingInformation{}
		if activityLine, ok := info["activity_line"].(string); ok {
			grpcReq.Information.ActivityLine = activityLine
		}
		if name, ok := info["name"].(string); ok {
			grpcReq.Information.Name = name
		}
		if address, ok := info["address"].(string); ok {
			grpcReq.Information.Address = address
		}
		if postalCode, ok := info["postal_code"].(string); ok {
			grpcReq.Information.PostalCode = postalCode
		}
		if website, ok := info["website"].(string); ok {
			grpcReq.Information.Website = website
		}
		if description, ok := info["description"].(string); ok {
			grpcReq.Information.Description = description
		}
	}

	_, err = h.buildingClient.UpdateBuilding(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// DestroyBuilding handles DELETE /api/v2/features/{feature}/build/buildings/{buildingModel}
func (h *FeaturesHandler) DestroyBuilding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	_, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/features/"), "/")
	if len(pathParts) < 4 {
		writeError(w, http.StatusBadRequest, "feature ID and building model ID are required")
		return
	}
	featureID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}
	buildingModelID, err := strconv.ParseUint(pathParts[3], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid building model ID")
		return
	}

	grpcReq := &featurespb.DestroyBuildingRequest{
		FeatureId:       featureID,
		BuildingModelId: buildingModelID,
	}

	_, err = h.buildingClient.DestroyBuilding(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// ListSellRequests handles GET /api/sell-requests
// Implements Laravel's SellRequestsController@index
func (h *FeaturesHandler) ListSellRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	sellerID := userCtx.UserID

	grpcReq := &featurespb.ListSellRequestsRequest{
		SellerId: sellerID,
	}

	resp, err := h.marketplaceClient.ListSellRequests(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build response matching Laravel SellRequestResource format
	sellRequests := make([]map[string]interface{}, 0, len(resp.SellRequests))
	for _, req := range resp.SellRequests {
		reqMap := map[string]interface{}{
			"id":         req.Id,
			"feature_id": req.FeatureId,
			"seller_id":  req.SellerId,
			"price_psc":  req.PricePsc,
			"price_irr":  req.PriceIrr,
			"status":     req.Status,
			"created_at": req.CreatedAt,
		}

		// Add feature properties if available
		if req.FeatureProperties != nil {
			reqMap["feature_properties"] = map[string]interface{}{
				"id":                       req.FeatureProperties.Id,
				"address":                  req.FeatureProperties.Address,
				"density":                  req.FeatureProperties.Density,
				"label":                    req.FeatureProperties.Label,
				"karbari":                  req.FeatureProperties.Karbari,
				"area":                     req.FeatureProperties.Area,
				"stability":                req.FeatureProperties.Stability,
				"region":                   req.FeatureProperties.Region,
				"owner":                    req.FeatureProperties.Owner,
				"rgb":                      req.FeatureProperties.Rgb,
				"price_psc":                req.FeatureProperties.PricePsc,
				"price_irr":                req.FeatureProperties.PriceIrr,
				"minimum_price_percentage": req.FeatureProperties.MinimumPricePercentage,
			}
		}

		// Add feature coordinates if available
		if len(req.FeatureCoordinates) > 0 {
			coords := make([]map[string]interface{}, 0, len(req.FeatureCoordinates))
			for _, coord := range req.FeatureCoordinates {
				coords = append(coords, map[string]interface{}{
					"id": coord.Id,
					"x":  coord.X,
					"y":  coord.Y,
				})
			}
			reqMap["feature_coordinates"] = coords
		}

		sellRequests = append(sellRequests, reqMap)
	}

	writeJSON(w, http.StatusOK, sellRequests)
}

// CreateSellRequest handles POST /api/sell-requests/store/{feature}
// Implements Laravel's SellRequestsController@store
func (h *FeaturesHandler) CreateSellRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	sellerID := userCtx.UserID

	// Extract feature ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/sell-requests/store/")
	path = strings.TrimSuffix(path, "/")
	featureID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	// Parse request body
	var reqBody map[string]interface{}
	if err := decodeRequestBody(r, &reqBody); err != nil {
		if err == io.EOF {
			writeValidationErrorWithLocale(w, "request body is required", h.locale)
		} else {
			writeValidationErrorWithLocale(w, "invalid request body", h.locale)
		}
		return
	}

	grpcReq := &featurespb.CreateSellRequestRequest{
		FeatureId: featureID,
		SellerId:  sellerID,
	}

	// Parse price_psc (optional)
	if pricePsc, ok := reqBody["price_psc"].(float64); ok {
		grpcReq.PricePsc = strconv.FormatFloat(pricePsc, 'f', -1, 64)
	} else if pricePsc, ok := reqBody["price_psc"].(string); ok {
		grpcReq.PricePsc = pricePsc
	}

	// Parse price_irr (optional)
	if priceIrr, ok := reqBody["price_irr"].(float64); ok {
		grpcReq.PriceIrr = strconv.FormatFloat(priceIrr, 'f', -1, 64)
	} else if priceIrr, ok := reqBody["price_irr"].(string); ok {
		grpcReq.PriceIrr = priceIrr
	}

	// Parse minimum_price_percentage (optional)
	if minPerc, ok := reqBody["minimum_price_percentage"].(float64); ok {
		grpcReq.MinimumPricePercentage = int32(minPerc)
	} else if minPerc, ok := reqBody["minimum_price_percentage"].(int); ok {
		grpcReq.MinimumPricePercentage = int32(minPerc)
	}

	resp, err := h.marketplaceClient.CreateSellRequest(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build response matching Laravel SellRequestResource format
	respMap := map[string]interface{}{
		"id":         resp.Id,
		"feature_id": resp.FeatureId,
		"seller_id":  resp.SellerId,
		"price_psc":  resp.PricePsc,
		"price_irr":  resp.PriceIrr,
		"status":     resp.Status,
		"created_at": resp.CreatedAt,
	}

	// Add feature properties if available
	if resp.FeatureProperties != nil {
		respMap["feature_properties"] = map[string]interface{}{
			"id":                       resp.FeatureProperties.Id,
			"address":                  resp.FeatureProperties.Address,
			"density":                  resp.FeatureProperties.Density,
			"label":                    resp.FeatureProperties.Label,
			"karbari":                  resp.FeatureProperties.Karbari,
			"area":                     resp.FeatureProperties.Area,
			"stability":                resp.FeatureProperties.Stability,
			"region":                   resp.FeatureProperties.Region,
			"owner":                    resp.FeatureProperties.Owner,
			"rgb":                      resp.FeatureProperties.Rgb,
			"price_psc":                resp.FeatureProperties.PricePsc,
			"price_irr":                resp.FeatureProperties.PriceIrr,
			"minimum_price_percentage": resp.FeatureProperties.MinimumPricePercentage,
		}
	}

	// Add feature coordinates if available
	if len(resp.FeatureCoordinates) > 0 {
		coords := make([]map[string]interface{}, 0, len(resp.FeatureCoordinates))
		for _, coord := range resp.FeatureCoordinates {
			coords = append(coords, map[string]interface{}{
				"id": coord.Id,
				"x":  coord.X,
				"y":  coord.Y,
			})
		}
		respMap["feature_coordinates"] = coords
	}

	writeJSON(w, http.StatusCreated, respMap)
}

// DeleteSellRequest handles DELETE /api/sell-requests/{sellRequest}
// Implements Laravel's SellRequestsController@destroy
func (h *FeaturesHandler) DeleteSellRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	sellerID := userCtx.UserID

	// Extract sell request ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/sell-requests/")
	path = strings.TrimSuffix(path, "/")
	sellRequestID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid sell request ID")
		return
	}

	grpcReq := &featurespb.DeleteSellRequestRequest{
		SellRequestId: sellRequestID,
		SellerId:      sellerID,
	}

	_, err = h.marketplaceClient.DeleteSellRequest(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Return empty 200 response (Laravel uses response()->noContent(200))
	w.WriteHeader(http.StatusOK)
}

// UpdateGracePeriod handles POST /api/buy-requests/add-grace-period/{buyFeatureRequest}
// Implements Laravel's BuyRequestsController@addGracePeriod
func (h *FeaturesHandler) UpdateGracePeriod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	sellerID := userCtx.UserID

	// Extract buy request ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests/add-grace-period/")
	path = strings.TrimSuffix(path, "/")
	requestID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid buy request ID")
		return
	}

	// Parse request body
	var reqBody map[string]interface{}
	if err := decodeRequestBody(r, &reqBody); err != nil {
		if err == io.EOF {
			writeValidationErrorWithLocale(w, "request body is required", h.locale)
		} else {
			writeValidationErrorWithLocale(w, "invalid request body", h.locale)
		}
		return
	}

	// Extract grace_period from request body
	var gracePeriodDays int32
	if gp, ok := reqBody["grace_period"].(float64); ok {
		gracePeriodDays = int32(gp)
	} else if gp, ok := reqBody["grace_period"].(int); ok {
		gracePeriodDays = int32(gp)
	} else {
		writeValidationErrorWithLocale(w, "grace_period is required and must be an integer", h.locale)
		return
	}

	// Validate grace period range (1-30)
	if gracePeriodDays < 1 || gracePeriodDays > 30 {
		writeValidationErrorWithLocale(w, "grace_period must be between 1 and 30", h.locale)
		return
	}

	// Build gRPC request
	grpcReq := &featurespb.UpdateGracePeriodRequest{
		RequestId:       requestID,
		SellerId:        sellerID,
		GracePeriodDays: gracePeriodDays,
	}

	// Call gRPC service
	_, err = h.marketplaceClient.UpdateGracePeriod(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Return empty JSON response (Laravel returns {})
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}
