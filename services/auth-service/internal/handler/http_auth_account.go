// Package handler provides Kong-facing HTTP handlers for auth-service.
package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"metarang/auth-service/internal/middleware"
	pb "metarang/shared/pb/auth"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/helpers"
)

// Register handles POST /api/auth/register
func (h *HTTPAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BackURL  string `json:"back_url"`
		Referral string `json:"referral"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.RegisterRequest{
		BackUrl:  req.BackURL,
		Referral: req.Referral,
	}

	resp, err := h.authClient.Register(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": resp.Url})
}

// Redirect handles GET /api/auth/redirect
func (h *HTTPAuthHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	redirectTo := r.URL.Query().Get("redirect_to")
	backURL := r.URL.Query().Get("back_url")

	grpcReq := &pb.RedirectRequest{
		RedirectTo: redirectTo,
		BackUrl:    backURL,
	}

	resp, err := h.authClient.Redirect(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// /auth/redirect endpoint should NOT wrap response in data field
	writeJSON(w, http.StatusOK, map[string]string{"url": resp.Url}, true)
}

// Callback handles GET /api/auth/callback
func (h *HTTPAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	grpcReq := &pb.CallbackRequest{
		State:       state,
		Code:        code,
		WalletLogin: parseWalletLoginQuery(r.URL.Query().Get("wallet_login")),
	}

	resp, err := h.authClient.Callback(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Redirect to the frontend URL with token and expires_at query parameters
	// According to spec: "Responds with a redirect to whichever cached URL is present"
	if resp.RedirectUrl != "" {
		// Log redirect for debugging
		http.Redirect(w, r, resp.RedirectUrl, http.StatusFound)
		return
	}

	// Fallback: if no redirect URL, return error with details for debugging
	writeError(w, http.StatusInternalServerError, "redirect URL not configured (empty response from auth service)")
}

// GetMe handles POST /api/auth/me
func (h *HTTPAuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetMeRequest{
		Token: userCtx.Token,
	}

	resp, err := h.authClient.GetMe(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	var walletAddress interface{}
	if resp.HasWallet {
		walletAddress = resp.WalletAddress
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"id":                             resp.Id,
			"name":                           resp.Name,
			"token":                          resp.Token,
			"automatic_logout":               resp.AutomaticLogout,
			"code":                           resp.Code,
			"image":                          resp.Image,
			"unread_notifications_count":     resp.UnreadNotificationsCount,
			"socre_percentage_to_next_level": resp.SocrePercentageToNextLevel,
			"hourly_profit_time_percentage":  resp.HourlyProfitTimePercentage,
			"verified_kyc":                   resp.VerifiedKyc,
			"birthdate":                      resp.Birthdate,
			"has_wallet":                     resp.HasWallet,
			"wallet_address":                 walletAddress,
			"wallet_login":                   resp.WalletLogin,
		},
	}

	if resp.Level != nil {
		levelPayload := map[string]interface{}{
			"id":          resp.Level.Id,
			"title":       resp.Level.Title,
			"description": resp.Level.Description,
			"score":       resp.Level.Score,
			"slug":        resp.Level.Slug,
			"fbx_file":    "",
		}

		if h.levelClient != nil {
			lvlResp, err := h.levelClient.GetUserLevel(r.Context(), &levelspb.GetUserLevelRequest{UserId: resp.Id})
			if err == nil && lvlResp != nil && lvlResp.LatestLevel != nil && lvlResp.LatestLevel.Gem != nil {
				levelPayload["fbx_file"] = lvlResp.LatestLevel.Gem.FbxFile
			}
		}

		response["data"].(map[string]interface{})["level"] = levelPayload
	}

	writeJSON(w, http.StatusOK, response)
}

// Logout handles POST /api/auth/logout
func (h *HTTPAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.LogoutRequest{
		Token: userCtx.Token,
	}

	_, err = h.authClient.Logout(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// ValidateToken handles POST /api/auth/validate
func (h *HTTPAuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.ValidateTokenRequest{
		Token: req.Token,
	}

	resp, err := h.authClient.ValidateToken(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":        resp.Valid,
		"user_id":      resp.UserId,
		"email":        resp.Email,
		"wallet_login": resp.WalletLogin,
	})
}

// RequestAccountSecurity handles POST /api/account/security
func (h *HTTPAuthHandler) RequestAccountSecurity(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Time  flexibleInt32 `json:"time" form:"time"`
		Phone string        `json:"phone,omitempty" form:"phone"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	minutes := req.Time.Int32()
	t := helpers.GetLocaleTranslations(h.locale)
	if minutes == 0 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"time": fmt.Sprintf(t.Required, "time"),
		}, h.locale)
		return
	}
	if minutes < 5 || minutes > 60 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"time": fmt.Sprintf(t.Invalid, "time"),
		}, h.locale)
		return
	}

	grpcReq := &pb.RequestAccountSecurityRequest{
		UserId:      userCtx.UserID,
		TimeMinutes: minutes,
		Phone:       req.Phone,
	}

	_, err = h.authClient.RequestAccountSecurity(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Return 204 No Content per API specification
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "OTP sent successfully",
	})
}

// VerifyAccountSecurity handles POST /api/account/security/verify
func (h *HTTPAuthHandler) VerifyAccountSecurity(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Code flexibleString `json:"code" form:"code"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	// Validation is now handled in the auth-service

	// Extract IP and UserAgent from request
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	code := strings.TrimSpace(helpers.NormalizePersianNumbers(req.Code.String()))

	grpcReq := &pb.VerifyAccountSecurityRequest{
		UserId:    userCtx.UserID,
		Code:      code,
		Ip:        ip,
		UserAgent: userAgent,
	}

	_, err = h.authClient.VerifyAccountSecurity(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Return 204 No Content per API specification
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "OTP verified successfully",
	})
}

// SearchUsers handles POST /api/search/users
func (h *HTTPAuthHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SearchTerm string `json:"searchTerm"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.SearchUsersRequest{
		SearchTerm: req.SearchTerm,
	}

	resp, err := h.searchClient.SearchUsers(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Convert protobuf response to JSON
	responseData := make([]map[string]interface{}, len(resp.Data))
	for i, result := range resp.Data {
		item := map[string]interface{}{
			"id":        result.Id,
			"code":      result.Code,
			"name":      result.Name,
			"followers": result.Followers,
		}
		if result.Level != "" {
			item["level"] = result.Level
		}
		if result.Photo != "" {
			item["photo"] = result.Photo
		}
		responseData[i] = item
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": responseData,
	})
}

// SearchFeatures handles POST /api/search/features
func (h *HTTPAuthHandler) SearchFeatures(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SearchTerm string `json:"searchTerm"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.SearchFeaturesRequest{
		SearchTerm: req.SearchTerm,
	}

	resp, err := h.searchClient.SearchFeatures(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Convert protobuf response to JSON
	responseData := make([]map[string]interface{}, len(resp.Data))
	for i, result := range resp.Data {
		item := map[string]interface{}{
			"id":                    result.Id,
			"feature_properties_id": result.FeaturePropertiesId,
			"address":               result.Address,
			"karbari":               result.Karbari,
			"price_psc":             result.PricePsc,
			"price_irr":             result.PriceIrr,
			"owner_code":            result.OwnerCode,
		}

		// Convert coordinates
		coordinates := make([]map[string]interface{}, len(result.Coordinates))
		for j, coord := range result.Coordinates {
			coordinates[j] = map[string]interface{}{
				"id": coord.Id,
				"x":  coord.X,
				"y":  coord.Y,
			}
		}
		item["coordinates"] = coordinates

		responseData[i] = item
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": responseData,
	})
}

// SearchIsicCodes handles POST /api/search/isic-codes
func (h *HTTPAuthHandler) SearchIsicCodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SearchTerm string `json:"searchTerm"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.SearchIsicCodesRequest{
		SearchTerm: req.SearchTerm,
	}

	resp, err := h.searchClient.SearchIsicCodes(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Convert protobuf response to JSON
	responseData := make([]map[string]interface{}, len(resp.Data))
	for i, result := range resp.Data {
		responseData[i] = map[string]interface{}{
			"id":   result.Id,
			"name": result.Name,
			"code": result.Code,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": responseData,
	})
}
