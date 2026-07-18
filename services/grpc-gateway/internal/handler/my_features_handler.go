package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"metarang/grpc-gateway/internal/middleware"
	pb "metarang/shared/pb/auth"
	featurespb "metarang/shared/pb/features"
)

// ListMyFeatures handles GET /api/my-features
func (h *FeaturesHandler) ListMyFeatures(w http.ResponseWriter, r *http.Request) {
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

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	grpcReq := &featurespb.ListMyFeaturesRequest{
		UserId: userCtx.UserID,
		Page:   page,
	}

	resp, err := h.featureClient.ListMyFeatures(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	features := make([]map[string]interface{}, 0, len(resp.Data))
	for _, feature := range resp.Data {
		featureMap := map[string]interface{}{
			"id": feature.Id,
		}

		if feature.Properties != nil {
			featureMap["properties"] = map[string]interface{}{
				"id":                       feature.Properties.Id,
				"price_psc":                feature.Properties.PricePsc,
				"price_irr":                feature.Properties.PriceIrr,
				"stability":                feature.Properties.Stability,
				"minimum_price_percentage": feature.Properties.MinimumPricePercentage,
				"karbari":                  feature.Properties.Karbari,
				"rgb":                      feature.Properties.Rgb,
			}
		}

		featureMap["images"] = []interface{}{}
		features = append(features, featureMap)
	}

	response := map[string]interface{}{
		"data": features,
		"links": map[string]interface{}{
			"first": resp.Links.First,
			"last":  resp.Links.Last,
			"prev":  resp.Links.Prev,
			"next":  resp.Links.Next,
		},
		"meta": map[string]interface{}{
			"current_page": resp.Meta.CurrentPage,
			"path":         resp.Meta.Path,
			"per_page":     resp.Meta.PerPage,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// GetMyFeature handles GET /api/my-features/{user}/features/{feature}
func (h *FeaturesHandler) GetMyFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := extractTokenFromHeader(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	validateReq := &pb.ValidateTokenRequest{Token: token}
	_, err := h.authClient.ValidateToken(r.Context(), validateReq)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/my-features/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "features" {
		writeError(w, http.StatusBadRequest, "invalid path format")
		return
	}

	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	featureID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	grpcReq := &featurespb.GetMyFeatureRequest{
		UserId:    userID,
		FeatureId: featureID,
	}

	resp, err := h.featureClient.GetMyFeature(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	feature := resp.Feature
	featureMap := buildFeatureMap(feature)
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": featureMap})
}

// AddMyFeatureImages handles POST /api/my-features/{user}/add-image/{feature}
func (h *FeaturesHandler) AddMyFeatureImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := extractTokenFromHeader(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	validateReq := &pb.ValidateTokenRequest{Token: token}
	_, err := h.authClient.ValidateToken(r.Context(), validateReq)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/my-features/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "add-image" {
		writeError(w, http.StatusBadRequest, "invalid path format")
		return
	}

	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	featureID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		writeValidationError(w, "images field is required")
		return
	}

	imageData := make([][]byte, 0, len(files))
	filenames := make([]string, 0, len(files))
	contentTypes := make([]string, 0, len(files))

	for _, fileHeader := range files {
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/bmp" {
			writeValidationError(w, "invalid image type: must be PNG, JPG, or BMP")
			return
		}

		if fileHeader.Size > 1024*1024 {
			writeValidationError(w, "image size exceeds 1024 KB limit")
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file")
			return
		}
		defer func() { _ = file.Close() }()

		data := make([]byte, fileHeader.Size)
		if _, err := file.Read(data); err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file data")
			return
		}

		imageData = append(imageData, data)
		filenames = append(filenames, fileHeader.Filename)
		contentTypes = append(contentTypes, contentType)
	}

	grpcReq := &featurespb.AddMyFeatureImagesRequest{
		UserId:       userID,
		FeatureId:    featureID,
		ImageData:    imageData,
		Filenames:    filenames,
		ContentTypes: contentTypes,
	}

	resp, err := h.featureClient.AddMyFeatureImages(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	images := make([]map[string]interface{}, 0, len(resp.Feature.Images))
	for _, img := range resp.Feature.Images {
		images = append(images, map[string]interface{}{
			"id":  img.Id,
			"url": img.Url,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": images})
}

// RemoveMyFeatureImage handles POST /api/my-features/{user}/remove-image/{feature}/image/{image}
func (h *FeaturesHandler) RemoveMyFeatureImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := extractTokenFromHeader(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	validateReq := &pb.ValidateTokenRequest{Token: token}
	_, err := h.authClient.ValidateToken(r.Context(), validateReq)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/my-features/")
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[1] != "remove-image" || parts[3] != "image" {
		writeError(w, http.StatusBadRequest, "invalid path format")
		return
	}

	userID, _ := strconv.ParseUint(parts[0], 10, 64)
	featureID, _ := strconv.ParseUint(parts[2], 10, 64)
	imageID, _ := strconv.ParseUint(parts[4], 10, 64)

	grpcReq := &featurespb.RemoveMyFeatureImageRequest{
		UserId:    userID,
		FeatureId: featureID,
		ImageId:   imageID,
	}

	_, err = h.featureClient.RemoveMyFeatureImage(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// UpdateMyFeature handles POST /api/my-features/{user}/features/{feature}
func (h *FeaturesHandler) UpdateMyFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := extractTokenFromHeader(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	validateReq := &pb.ValidateTokenRequest{Token: token}
	_, err := h.authClient.ValidateToken(r.Context(), validateReq)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/my-features/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "features" {
		writeError(w, http.StatusBadRequest, "invalid path format")
		return
	}

	userID, _ := strconv.ParseUint(parts[0], 10, 64)
	featureID, _ := strconv.ParseUint(parts[2], 10, 64)

	var reqBody struct {
		MinimumPricePercentage int32 `json:"minimum_price_percentage"`
	}

	if err := decodeRequestBody(r, &reqBody); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if reqBody.MinimumPricePercentage < 80 {
		writeValidationError(w, "minimum_price_percentage must be at least 80")
		return
	}

	grpcReq := &featurespb.UpdateMyFeatureRequest{
		UserId:                 userID,
		FeatureId:              featureID,
		MinimumPricePercentage: reqBody.MinimumPricePercentage,
	}

	_, err = h.featureClient.UpdateMyFeature(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func buildFeatureMap(feature *featurespb.Feature) map[string]interface{} {
	featureMap := map[string]interface{}{
		"id": feature.Id,
	}

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
		}
	}

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

	if feature.Seller != nil {
		featureMap["seller"] = map[string]interface{}{
			"id":   feature.Seller.Id,
			"name": feature.Seller.Name,
			"code": feature.Seller.Code,
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

	return featureMap
}
