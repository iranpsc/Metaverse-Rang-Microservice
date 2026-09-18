// Package handler provides Kong-facing HTTP handlers for auth-service.
package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"metarang/auth-service/internal/middleware"
	pb "metarang/shared/pb/auth"
)

// ============================================================================
// Settings Service Handlers
// ============================================================================

// GetSettings handles GET /api/settings
func (h *HTTPAuthHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetSettingsRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.settingsClient.GetSettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	data := resp.GetData()
	response := map[string]interface{}{
		"checkout_days_count":           data.GetCheckoutDaysCount(),
		"automatic_logout":              data.GetAutomaticLogout(),
		"available_reset_mobile_resets": data.GetAvailableResetMobileResets(),
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateSettings handles POST /api/settings
func (h *HTTPAuthHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		CheckoutDaysCount uint32 `json:"checkout_days_count"`
		AutomaticLogout   int32  `json:"automatic_logout"`
		Setting           string `json:"setting"` // "status", "level", or "details"
		Status            bool   `json:"status"`  // boolean value
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	var checkoutDaysCount *uint32
	var automaticLogout *int32
	var setting *string
	var status *bool

	// Only set pointers if values are provided (non-zero for numeric types)
	if req.CheckoutDaysCount > 0 {
		checkoutDaysCount = &req.CheckoutDaysCount
	}
	if req.AutomaticLogout > 0 {
		automaticLogout = &req.AutomaticLogout
	}
	if req.Setting != "" {
		setting = &req.Setting
		status = &req.Status
	}

	grpcReq := &pb.UpdateSettingsRequest{
		UserId:            userCtx.UserID,
		CheckoutDaysCount: 0, // Will be set properly by handler logic
		AutomaticLogout:   0, // Will be set properly by handler logic
		Setting:           "",
		Status:            false,
	}

	// Set values if provided
	if checkoutDaysCount != nil {
		grpcReq.CheckoutDaysCount = *checkoutDaysCount
	}
	if automaticLogout != nil {
		grpcReq.AutomaticLogout = *automaticLogout
	}
	if setting != nil {
		grpcReq.Setting = *setting
		grpcReq.Status = *status
	}

	_, err = h.settingsClient.UpdateSettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Response: 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}

// GetGeneralSettings handles GET /api/general-settings
func (h *HTTPAuthHandler) GetGeneralSettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetGeneralSettingsRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.settingsClient.GetGeneralSettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Response format: NotificationSettingsResource with all channels as booleans
	response := map[string]interface{}{
		"id":                       resp.Data.Id,
		"announcements_sms":        resp.Data.AnnouncementsSms,
		"announcements_email":      resp.Data.AnnouncementsEmail,
		"reports_sms":              resp.Data.ReportsSms,
		"reports_email":            resp.Data.ReportsEmail,
		"login_verification_sms":   resp.Data.LoginVerificationSms,
		"login_verification_email": resp.Data.LoginVerificationEmail,
		"transactions_sms":         resp.Data.TransactionsSms,
		"transactions_email":       resp.Data.TransactionsEmail,
		"trades_sms":               resp.Data.TradesSms,
		"trades_email":             resp.Data.TradesEmail,
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateGeneralSettings handles PUT /api/general-settings/{setting}
func (h *HTTPAuthHandler) UpdateGeneralSettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract setting ID from path: /api/general-settings/{setting}
	settingIDStr := extractIDFromPath(r.URL.Path, "/api/general-settings/")
	if settingIDStr == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("setting ID is required in path. Received path: %s", r.URL.Path))
		return
	}

	settingID, err := strconv.ParseUint(settingIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid setting ID")
		return
	}

	var req struct {
		AnnouncementsSMS       bool `json:"announcements_sms"`
		AnnouncementsEmail     bool `json:"announcements_email"`
		ReportsSMS             bool `json:"reports_sms"`
		ReportsEmail           bool `json:"reports_email"`
		LoginVerificationSMS   bool `json:"login_verification_sms"`
		LoginVerificationEmail bool `json:"login_verification_email"`
		TransactionsSMS        bool `json:"transactions_sms"`
		TransactionsEmail      bool `json:"transactions_email"`
		TradesSMS              bool `json:"trades_sms"`
		TradesEmail            bool `json:"trades_email"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.UpdateGeneralSettingsRequest{
		UserId:    userCtx.UserID,
		SettingId: settingID,
		Notifications: &pb.NotificationSettingsData{
			AnnouncementsSms:       req.AnnouncementsSMS,
			AnnouncementsEmail:     req.AnnouncementsEmail,
			ReportsSms:             req.ReportsSMS,
			ReportsEmail:           req.ReportsEmail,
			LoginVerificationSms:   req.LoginVerificationSMS,
			LoginVerificationEmail: req.LoginVerificationEmail,
			TransactionsSms:        req.TransactionsSMS,
			TransactionsEmail:      req.TransactionsEmail,
			TradesSms:              req.TradesSMS,
			TradesEmail:            req.TradesEmail,
		},
	}

	resp, err := h.settingsClient.UpdateGeneralSettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Response format: NotificationSettingsResource with all channels as booleans
	response := map[string]interface{}{
		"announcements_sms":        resp.Data.AnnouncementsSms,
		"announcements_email":      resp.Data.AnnouncementsEmail,
		"reports_sms":              resp.Data.ReportsSms,
		"reports_email":            resp.Data.ReportsEmail,
		"login_verification_sms":   resp.Data.LoginVerificationSms,
		"login_verification_email": resp.Data.LoginVerificationEmail,
		"transactions_sms":         resp.Data.TransactionsSms,
		"transactions_email":       resp.Data.TransactionsEmail,
		"trades_sms":               resp.Data.TradesSms,
		"trades_email":             resp.Data.TradesEmail,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetPrivacySettings handles GET /api/privacy
func (h *HTTPAuthHandler) GetPrivacySettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetPrivacySettingsRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.settingsClient.GetPrivacySettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Response format: { "data": { <key>: <0|1>, ... } }
	response := map[string]interface{}{
		"data": resp.Data,
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdatePrivacySettings handles POST /api/privacy
func (h *HTTPAuthHandler) UpdatePrivacySettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"` // Accepts boolean or numeric (0/1)
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	// Convert value to int32 (handles both bool and numeric)
	var value int32
	switch v := req.Value.(type) {
	case bool:
		if v {
			value = 1
		} else {
			value = 0
		}
	case float64:
		value = int32(v)
	case int:
		value = int32(v)
	case int32:
		value = v
	default:
		writeError(w, http.StatusBadRequest, "value must be boolean or numeric (0 or 1)")
		return
	}

	grpcReq := &pb.UpdatePrivacySettingsRequest{
		UserId: userCtx.UserID,
		Key:    req.Key,
		Value:  value,
	}

	_, err = h.settingsClient.UpdatePrivacySettings(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Response: 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}
