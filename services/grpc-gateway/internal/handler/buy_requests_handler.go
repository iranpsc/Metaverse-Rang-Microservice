package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"metarang/grpc-gateway/internal/middleware"
	featurespb "metarang/shared/pb/features"
)

func buyRequestToMap(req *featurespb.BuyRequestResponse) map[string]interface{} {
	reqMap := map[string]interface{}{
		"id":         req.Id,
		"feature_id": req.FeatureId,
		"status":     req.Status,
		"note":       req.Note,
		"price_psc":  req.PricePsc,
		"price_irr":  req.PriceIrr,
		"created_at": req.CreatedAt,
	}

	if req.Buyer != nil {
		reqMap["buyer"] = map[string]interface{}{
			"id":            req.Buyer.Id,
			"code":          req.Buyer.Code,
			"profile_photo": req.Buyer.ProfilePhoto,
		}
	}

	if req.Seller != nil {
		reqMap["seller"] = map[string]interface{}{
			"id":   req.Seller.Id,
			"code": req.Seller.Code,
		}
	}

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

	if req.RequestedGracePeriod != "" {
		reqMap["requested_grace_period"] = req.RequestedGracePeriod
	}

	return reqMap
}

// ListBuyRequests handles GET /api/buy-requests
func (h *FeaturesHandler) ListBuyRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resp, err := h.marketplaceClient.ListBuyRequests(r.Context(), &featurespb.ListBuyRequestsRequest{
		BuyerId: userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	buyRequests := make([]map[string]interface{}, 0, len(resp.BuyRequests))
	for _, req := range resp.BuyRequests {
		buyRequests = append(buyRequests, buyRequestToMap(req))
	}

	writeJSON(w, http.StatusOK, buyRequests)
}

// ListReceivedBuyRequests handles GET /api/buy-requests/recieved
func (h *FeaturesHandler) ListReceivedBuyRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resp, err := h.marketplaceClient.ListReceivedBuyRequests(r.Context(), &featurespb.ListReceivedBuyRequestsRequest{
		SellerId: userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	buyRequests := make([]map[string]interface{}, 0, len(resp.BuyRequests))
	for _, req := range resp.BuyRequests {
		buyRequests = append(buyRequests, buyRequestToMap(req))
	}

	writeJSON(w, http.StatusOK, buyRequests)
}

// SendBuyRequest handles POST /api/buy-requests/store/{feature}
func (h *FeaturesHandler) SendBuyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests/store/")
	path = strings.TrimSuffix(path, "/")
	featureID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature ID")
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

	grpcReq := &featurespb.SendBuyRequestRequest{
		FeatureId: featureID,
		BuyerId:   userCtx.UserID,
	}

	if note, ok := reqBody["note"].(string); ok {
		grpcReq.Note = note
	}
	if pricePsc, ok := reqBody["price_psc"].(float64); ok {
		grpcReq.PricePsc = strconv.FormatFloat(pricePsc, 'f', -1, 64)
	} else if pricePsc, ok := reqBody["price_psc"].(string); ok {
		grpcReq.PricePsc = pricePsc
	}
	if priceIrr, ok := reqBody["price_irr"].(float64); ok {
		grpcReq.PriceIrr = strconv.FormatFloat(priceIrr, 'f', -1, 64)
	} else if priceIrr, ok := reqBody["price_irr"].(string); ok {
		grpcReq.PriceIrr = priceIrr
	}

	resp, err := h.marketplaceClient.SendBuyRequest(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buyRequestToMap(resp))
}

// AcceptBuyRequest handles POST /api/buy-requests/accept/{buyFeatureRequest}
func (h *FeaturesHandler) AcceptBuyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests/accept/")
	path = strings.TrimSuffix(path, "/")
	requestID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid buy request ID")
		return
	}

	resp, err := h.marketplaceClient.AcceptBuyRequest(r.Context(), &featurespb.AcceptBuyRequestRequest{
		RequestId: requestID,
		SellerId:  userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buyRequestToMap(resp))
}

// RejectBuyRequest handles POST /api/buy-requests/reject/{buyFeatureRequest}
func (h *FeaturesHandler) RejectBuyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests/reject/")
	path = strings.TrimSuffix(path, "/")
	requestID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid buy request ID")
		return
	}

	_, err = h.marketplaceClient.RejectBuyRequest(r.Context(), &featurespb.RejectBuyRequestRequest{
		RequestId: requestID,
		SellerId:  userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBuyRequest handles DELETE /api/buy-requests/delete/{buyFeatureRequest}
func (h *FeaturesHandler) DeleteBuyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests/delete/")
	path = strings.TrimSuffix(path, "/")
	requestID, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid buy request ID")
		return
	}

	_, err = h.marketplaceClient.DeleteBuyRequest(r.Context(), &featurespb.DeleteBuyRequestRequest{
		RequestId: requestID,
		BuyerId:   userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleBuyRequestsRoutes dispatches buy-request routes under /api/buy-requests
func (h *FeaturesHandler) HandleBuyRequestsRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/buy-requests")
	path = strings.Trim(path, "/")

	if path == "" {
		switch r.Method {
		case http.MethodGet:
			h.ListBuyRequests(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}

	if path == "recieved" {
		if r.Method == http.MethodGet {
			h.ListReceivedBuyRequests(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "store/") {
		if r.Method == http.MethodPost {
			h.SendBuyRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "accept/") {
		if r.Method == http.MethodPost {
			h.AcceptBuyRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "reject/") {
		if r.Method == http.MethodPost {
			h.RejectBuyRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "delete/") {
		if r.Method == http.MethodDelete {
			h.DeleteBuyRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "add-grace-period/") {
		if r.Method == http.MethodPost {
			h.UpdateGracePeriod(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	http.NotFound(w, r)
}

// HandleSellRequestsRoutes dispatches sell-request routes under /api/sell-requests
func (h *FeaturesHandler) HandleSellRequestsRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sell-requests")
	path = strings.Trim(path, "/")

	if path == "" {
		switch r.Method {
		case http.MethodGet:
			h.ListSellRequests(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}

	if strings.HasPrefix(path, "store/") {
		if r.Method == http.MethodPost {
			h.CreateSellRequest(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if r.Method == http.MethodDelete {
		h.DeleteSellRequest(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleFeaturesRoutes dispatches feature routes under /api/features/
func (h *FeaturesHandler) HandleFeaturesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/features/")
	path = strings.Trim(path, "/")

	if strings.HasPrefix(path, "buy/") {
		if r.Method == http.MethodPost {
			h.BuyFeature(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if path == "build/completed" || path == "building/completed" {
		if r.Method == http.MethodGet {
			h.ListCompletedBuildings(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if IsFeatureTradeHistoryPath(path) {
		if r.Method == http.MethodGet {
			h.GetFeatureTradeHistory(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.Contains(path, "/build/package") {
		if r.Method == http.MethodGet {
			h.GetBuildPackage(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.Contains(path, "/build/buildings/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 4 && parts[1] == "build" && parts[2] == "buildings" {
			switch r.Method {
			case http.MethodPut:
				h.UpdateBuilding(w, r)
			case http.MethodDelete:
				h.DestroyBuilding(w, r)
			default:
				http.NotFound(w, r)
			}
			return
		}
	}

	if strings.Contains(path, "/build/buildings") {
		if r.Method == http.MethodGet {
			h.GetBuildings(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if strings.Contains(path, "/build/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 3 && parts[1] == "build" {
			if r.Method == http.MethodPost {
				h.BuildFeature(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
	}

	if path != "" {
		h.GetFeature(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleMyFeaturesRoutes dispatches my-features routes under /api/my-features/
func (h *FeaturesHandler) HandleMyFeaturesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/my-features/")
	path = strings.Trim(path, "/")

	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")

	if len(parts) >= 3 && parts[1] == "add-image" {
		if r.Method == http.MethodPost {
			h.AddMyFeatureImages(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if len(parts) >= 5 && parts[1] == "remove-image" && parts[3] == "image" {
		if r.Method == http.MethodPost {
			h.RemoveMyFeatureImage(w, r)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	if len(parts) >= 3 && parts[1] == "features" {
		switch r.Method {
		case http.MethodGet:
			h.GetMyFeature(w, r)
		case http.MethodPost:
			h.UpdateMyFeature(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}

	http.NotFound(w, r)
}
