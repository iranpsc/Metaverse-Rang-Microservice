// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/grpc-gateway/internal/middleware"
	pb "metarang/shared/pb/auth"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/helpers"
)

type AuthHandler struct {
	authClient              pb.AuthServiceClient
	userClient              pb.UserServiceClient
	kycClient               pb.KYCServiceClient
	citizenClient           pb.CitizenServiceClient
	personalInfoClient      pb.PersonalInfoServiceClient
	profileLimitationClient pb.ProfileLimitationServiceClient
	profilePhotoClient      pb.ProfilePhotoServiceClient
	settingsClient          pb.SettingsServiceClient
	userEventsClient        pb.UserEventsServiceClient
	searchClient            pb.SearchServiceClient
	levelClient             levelspb.LevelServiceClient
	locale                  string
}

func NewAuthHandler(conn *grpc.ClientConn, levelConn *grpc.ClientConn, locale string) *AuthHandler {
	h := &AuthHandler{
		authClient:              pb.NewAuthServiceClient(conn),
		userClient:              pb.NewUserServiceClient(conn),
		kycClient:               pb.NewKYCServiceClient(conn),
		citizenClient:           pb.NewCitizenServiceClient(conn),
		personalInfoClient:      pb.NewPersonalInfoServiceClient(conn),
		profileLimitationClient: pb.NewProfileLimitationServiceClient(conn),
		profilePhotoClient:      pb.NewProfilePhotoServiceClient(conn),
		settingsClient:          pb.NewSettingsServiceClient(conn),
		userEventsClient:        pb.NewUserEventsServiceClient(conn),
		searchClient:            pb.NewSearchServiceClient(conn),
		locale:                  locale,
	}
	if levelConn != nil {
		h.levelClient = levelspb.NewLevelServiceClient(levelConn)
	}
	return h
}

// writeGRPCErrorLocale writes gRPC errors using the handler's locale
func (h *AuthHandler) writeGRPCErrorLocale(w http.ResponseWriter, err error) {
	writeGRPCErrorWithLocale(w, err, h.locale)
}

// Register handles POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) Redirect(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	grpcReq := &pb.CallbackRequest{
		State: state,
		Code:  code,
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
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
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
		"valid":   resp.Valid,
		"user_id": resp.UserId,
		"email":   resp.Email,
	})
}

// RequestAccountSecurity handles POST /api/account/security
func (h *AuthHandler) RequestAccountSecurity(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse request body (Laravel API field: time)
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
func (h *AuthHandler) VerifyAccountSecurity(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse request body (Laravel accepts numeric|string for code)
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

// GetUser handles GET /api/user
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	grpcReq := &pb.GetUserRequest{
		UserId: userID,
	}

	resp, err := h.userClient.GetUser(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateProfile handles PUT/PATCH /api/user/profile
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uint64 `json:"user_id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Phone  string `json:"phone"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.UpdateProfileRequest{
		UserId: req.UserID,
		Name:   req.Name,
		Email:  req.Email,
		Phone:  req.Phone,
	}

	resp, err := h.userClient.UpdateProfile(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetProfileLimitations handles GET /api/user/profile-limitations or GET /api/users/{user}/profile-limitations
func (h *AuthHandler) GetProfileLimitations(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	callerUserID := userCtx.UserID

	// Try to extract target user ID from path first (/api/users/{user}/profile-limitations)
	targetUserIDStr := ""
	if strings.HasPrefix(r.URL.Path, "/api/users/") {
		// Extract user ID from path: /api/users/{user}/profile-limitations
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			targetUserIDStr = pathParts[0]
		}
	}

	// Fallback to query parameter if not in path
	if targetUserIDStr == "" {
		targetUserIDStr = r.URL.Query().Get("target_user_id")
		// Also support "user_id" as query param
		if targetUserIDStr == "" {
			targetUserIDStr = r.URL.Query().Get("user_id")
		}
	}

	if targetUserIDStr == "" {
		writeError(w, http.StatusBadRequest, "target user_id is required (either in path /api/users/{user}/profile-limitations or as query parameter)")
		return
	}

	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target user_id")
		return
	}

	grpcReq := &pb.GetProfileLimitationsRequest{
		CallerUserId: callerUserID,
		TargetUserId: targetUserID,
	}

	resp, err := h.userClient.GetProfileLimitations(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to API spec: { "data": {...} } or { "data": [] } if not found
	if resp.Data == nil || resp.Data.Id == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}

	// Convert proto to JSON format matching Laravel API
	data := map[string]interface{}{
		"id":              resp.Data.Id,
		"limiter_user_id": resp.Data.LimiterUserId,
		"limited_user_id": resp.Data.LimitedUserId,
		"options": map[string]bool{
			"follow":                  resp.Data.Options.Follow,
			"send_message":            resp.Data.Options.SendMessage,
			"share":                   resp.Data.Options.Share,
			"send_ticket":             resp.Data.Options.SendTicket,
			"view_profile_images":     resp.Data.Options.ViewProfileImages,
			"view_features_locations": resp.Data.Options.ViewFeaturesLocations,
		},
		"created_at": resp.Data.CreatedAt,
		"updated_at": resp.Data.UpdatedAt,
	}

	// Only include note if caller is the limiter
	if resp.Data.Note != "" && callerUserID == resp.Data.LimiterUserId {
		data["note"] = resp.Data.Note
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// ListUsers handles GET /api/users
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	orderBy := r.URL.Query().Get("order-by")
	pageStr := r.URL.Query().Get("page")
	page := int32(1)
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	grpcReq := &pb.ListUsersRequest{
		Search:  search,
		OrderBy: orderBy,
		Page:    page,
	}

	resp, err := h.userClient.ListUsers(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildListUsersHTTPResponse(r, resp))
}

// GetUserLevels handles GET /api/users/{user}/levels
func (h *AuthHandler) GetUserLevels(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/users/{user}/levels
	pathStr := strings.TrimPrefix(r.URL.Path, "/api/users/")
	// Remove query params if any
	if idx := strings.Index(pathStr, "?"); idx != -1 {
		pathStr = pathStr[:idx]
	}
	pathParts := strings.Split(strings.Trim(pathStr, "/"), "/")
	// Filter out empty parts
	var cleanParts []string
	for _, part := range pathParts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	if len(cleanParts) < 2 || cleanParts[0] == "" || cleanParts[1] != "levels" {
		writeError(w, http.StatusBadRequest, "invalid path format: expected /api/users/{user}/levels")
		return
	}

	userIDStr := cleanParts[0]
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	grpcReq := &pb.GetUserLevelsRequest{
		UserId: userID,
	}

	resp, err := h.userClient.GetUserLevels(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel API spec
	data := map[string]interface{}{}

	if resp.Data.LatestLevel != nil {
		latestLevel := map[string]interface{}{
			"id":    resp.Data.LatestLevel.Id,
			"name":  resp.Data.LatestLevel.Title,
			"score": resp.Data.LatestLevel.Score,
			"slug":  resp.Data.LatestLevel.Slug,
		}
		if resp.Data.LatestLevel.ImageUrl != "" {
			latestLevel["image"] = resp.Data.LatestLevel.ImageUrl
		}
		data["latest_level"] = latestLevel
	} else {
		data["latest_level"] = nil
	}

	previousLevels := make([]map[string]interface{}, 0, len(resp.Data.PreviousLevels))
	for _, level := range resp.Data.PreviousLevels {
		levelData := map[string]interface{}{
			"id":    level.Id,
			"name":  level.Title,
			"score": level.Score,
			"slug":  level.Slug,
		}
		if level.ImageUrl != "" {
			levelData["image"] = level.ImageUrl
		}
		previousLevels = append(previousLevels, levelData)
	}
	data["previous_levels"] = previousLevels
	data["score_percentage_to_next_level"] = resp.Data.ScorePercentageToNextLevel

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// GetUserProfile handles GET /api/users/{user}/profile
func (h *AuthHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/users/{user}/profile
	pathStr := strings.TrimPrefix(r.URL.Path, "/api/users/")
	// Remove query params if any
	if idx := strings.Index(pathStr, "?"); idx != -1 {
		pathStr = pathStr[:idx]
	}
	pathParts := strings.Split(strings.Trim(pathStr, "/"), "/")
	// Filter out empty parts
	var cleanParts []string
	for _, part := range pathParts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	if len(cleanParts) < 2 || cleanParts[0] == "" || cleanParts[1] != "profile" {
		writeError(w, http.StatusBadRequest, "invalid path format: expected /api/users/{user}/profile")
		return
	}

	userIDStr := cleanParts[0]
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	// Get viewer user ID from token if authenticated
	var viewerUserID uint64
	userCtx, err := middleware.GetUserFromRequest(r)
	if err == nil {
		viewerUserID = userCtx.UserID
	}

	grpcReq := &pb.GetUserProfileRequest{
		UserId:       userID,
		ViewerUserId: viewerUserID,
	}

	resp, err := h.userClient.GetUserProfile(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel ProfileResource spec
	data := map[string]interface{}{
		"id":             resp.Data.Id,
		"code":           resp.Data.Code,
		"profile_images": resp.Data.ProfileImages,
	}

	// Add optional fields (may be empty/null if privacy disallows)
	if resp.Data.Name != "" {
		data["name"] = resp.Data.Name
	}
	if resp.Data.RegisteredAt != "" {
		data["registered_at"] = resp.Data.RegisteredAt
	}
	if resp.Data.FollowersCount != 0 {
		data["followers_count"] = resp.Data.FollowersCount
	}
	if resp.Data.FollowingCount != 0 {
		data["following_count"] = resp.Data.FollowingCount
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// GetAuthenticatedUserWallet handles GET /api/user/wallet (Laravel TransactionController@getWallet)
func (h *AuthHandler) GetAuthenticatedUserWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	h.writeUserWalletResponse(w, r, userCtx.UserID)
}

// GetUserWallet handles GET /api/users/{user}/wallet
func (h *AuthHandler) GetUserWallet(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/users/{user}/wallet
	pathStr := strings.TrimPrefix(r.URL.Path, "/api/users/")
	// Remove query params if any
	if idx := strings.Index(pathStr, "?"); idx != -1 {
		pathStr = pathStr[:idx]
	}
	pathParts := strings.Split(strings.Trim(pathStr, "/"), "/")
	// Filter out empty parts
	var cleanParts []string
	for _, part := range pathParts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	if len(cleanParts) < 2 || cleanParts[0] == "" || cleanParts[1] != "wallet" {
		writeError(w, http.StatusBadRequest, "invalid path format: expected /api/users/{user}/wallet")
		return
	}

	userIDStr := cleanParts[0]
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	h.writeUserWalletResponse(w, r, userID)
}

func (h *AuthHandler) writeUserWalletResponse(w http.ResponseWriter, r *http.Request, userID uint64) {
	grpcReq := &pb.GetUserWalletRequest{
		UserId: userID,
	}

	resp, err := h.userClient.GetUserWallet(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	parseFloat := func(s string) float64 {
		if s == "" {
			return 0
		}
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return val
	}

	data := map[string]interface{}{
		"psc":          parseFloat(resp.Psc),
		"irr":          parseFloat(resp.Irr),
		"red":          parseFloat(resp.Red),
		"blue":         parseFloat(resp.Blue),
		"yellow":       parseFloat(resp.Yellow),
		"satisfaction": parseFloat(resp.Satisfaction),
		"effect":       resp.Effect,
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// GetUserFeaturesCount handles GET /api/users/{user}/features/count
func (h *AuthHandler) GetUserFeaturesCount(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path: /api/users/{user}/features/count
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
	if len(pathParts) < 3 || pathParts[0] == "" || pathParts[1] != "features" || pathParts[2] != "count" {
		writeError(w, http.StatusBadRequest, "invalid path format: expected /api/users/{user}/features/count")
		return
	}

	userIDStr := pathParts[0]
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	grpcReq := &pb.GetUserFeaturesCountRequest{
		UserId: userID,
	}

	resp, err := h.userClient.GetUserFeaturesCount(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel API spec
	data := map[string]interface{}{
		"maskoni_features_count":   resp.Data.MaskoniFeaturesCount,
		"tejari_features_count":    resp.Data.TejariFeaturesCount,
		"amoozeshi_features_count": resp.Data.AmoozeshiFeaturesCount,
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// GetKYC handles GET /api/kyc
func (h *AuthHandler) GetKYC(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetKYCRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.kycClient.GetKYC(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// If KYC not found, return empty JSON object (matches Laravel behavior)
	if resp.Id == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}

	writeJSON(w, http.StatusOK, formatKYCResponse(resp))
}

// UpdateKYC handles PUT/PATCH /api/kyc
func (h *AuthHandler) UpdateKYC(w http.ResponseWriter, r *http.Request) {
	// Accept Laravel-style POST + _method=put|patch for multipart file uploads
	if m := EffectiveHTTPMethod(r); m != http.MethodPut && m != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse multipart form (max 10MB for melli_card image)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	// Extract form fields
	fname := r.FormValue("fname")
	lname := r.FormValue("lname")
	melliCode := r.FormValue("melli_code")
	birthdate := r.FormValue("birthdate")
	province := r.FormValue("province")
	verifyTextIDStr := r.FormValue("verify_text_id")
	gender := r.FormValue("gender")

	// Parse verify_text_id
	verifyTextID, err := strconv.ParseUint(verifyTextIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid verify_text_id")
		return
	}

	// Extract video info (path and name)
	videoPath := r.FormValue("video[path]")
	videoName := r.FormValue("video[name]")

	// Extract melli_card file
	file, header, err := r.FormFile("melli_card")
	if err != nil {
		writeError(w, http.StatusBadRequest, "melli_card file is required")
		return
	}
	defer file.Close()

	// Read full file body (multipart size may be unknown; single Read is not enough).
	melliCardData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read melli_card file data")
		return
	}

	// Build gRPC request
	grpcReq := &pb.UpdateKYCRequest{
		UserId:               userCtx.UserID,
		Fname:                fname,
		Lname:                lname,
		MelliCode:            melliCode,
		Birthdate:            birthdate,
		Province:             province,
		MelliCardData:        melliCardData,
		MelliCardFilename:    header.Filename,
		MelliCardContentType: header.Header.Get("Content-Type"),
		VerifyTextId:         verifyTextID,
		Gender:               gender,
	}

	// Add video info if provided
	if videoPath != "" && videoName != "" {
		grpcReq.Video = &pb.VideoInfo{
			Path: videoPath,
			Name: videoName,
		}
	}

	resp, err := h.kycClient.UpdateKYC(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatKYCResponse(resp))
}

// formatKYCResponse builds a Laravel KycResource-compatible JSON object.
func formatKYCResponse(resp *pb.KYCResponse) map[string]interface{} {
	response := map[string]interface{}{
		"id":         fmt.Sprintf("%d", resp.Id),
		"melli_card": resp.MelliCard,
		"fname":      resp.Fname,
		"lname":      resp.Lname,
		"melli_code": resp.MelliCode,
		"birthdate":  resp.Birthdate,
		"province":   resp.Province,
		"status":     resp.Status,
		"video":      resp.Video,
		"gender":     resp.Gender,
	}
	if resp.Errors != "" {
		var errorsField interface{}
		if err := json.Unmarshal([]byte(resp.Errors), &errorsField); err == nil {
			response["errors"] = errorsField
		} else {
			response["errors"] = resp.Errors
		}
	}
	return response
}

// ListBankAccounts handles GET /api/bank-accounts
func (h *AuthHandler) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.ListBankAccountsRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.kycClient.ListBankAccounts(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel API: { "data": [...] }
	data := make([]map[string]interface{}, 0, len(resp.Data))
	for _, account := range resp.Data {
		data = append(data, formatBankAccountResource(account))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": data,
	})
}

// CreateBankAccount handles POST /api/bank-accounts
func (h *AuthHandler) CreateBankAccount(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		BankName string `json:"bank_name"`
		ShabaNum string `json:"shaba_num"`
		CardNum  string `json:"card_num"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.CreateBankAccountRequest{
		UserId:   userCtx.UserID,
		BankName: req.BankName,
		ShabaNum: req.ShabaNum,
		CardNum:  req.CardNum,
	}

	resp, err := h.kycClient.CreateBankAccount(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, formatBankAccountResource(resp))
}

// GetBankAccount handles GET /api/bank-accounts/{bankAccount}
func (h *AuthHandler) GetBankAccount(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	bankAccountIDStr := extractIDFromPath(r.URL.Path, "/api/bank-accounts/")
	if bankAccountIDStr == "" {
		writeError(w, http.StatusBadRequest, "bank_account_id is required")
		return
	}

	bankAccountID, err := strconv.ParseUint(bankAccountIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bank_account_id")
		return
	}

	grpcReq := &pb.GetBankAccountRequest{
		UserId:        userCtx.UserID,
		BankAccountId: bankAccountID,
	}

	resp, err := h.kycClient.GetBankAccount(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatBankAccountResource(resp))
}

// UpdateBankAccount handles PUT/PATCH /api/bank-accounts/{bankAccount}
func (h *AuthHandler) UpdateBankAccount(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	bankAccountIDStr := extractIDFromPath(r.URL.Path, "/api/bank-accounts/")
	if bankAccountIDStr == "" {
		writeError(w, http.StatusBadRequest, "bank_account_id is required")
		return
	}

	bankAccountID, err := strconv.ParseUint(bankAccountIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bank_account_id")
		return
	}

	var req struct {
		BankName string `json:"bank_name"`
		ShabaNum string `json:"shaba_num"`
		CardNum  string `json:"card_num"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.UpdateBankAccountRequest{
		UserId:        userCtx.UserID,
		BankAccountId: bankAccountID,
		BankName:      req.BankName,
		ShabaNum:      req.ShabaNum,
		CardNum:       req.CardNum,
	}

	resp, err := h.kycClient.UpdateBankAccount(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatBankAccountResource(resp))
}

// DeleteBankAccount handles DELETE /api/bank-accounts/{bankAccount}
func (h *AuthHandler) DeleteBankAccount(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	bankAccountIDStr := extractIDFromPath(r.URL.Path, "/api/bank-accounts/")
	if bankAccountIDStr == "" {
		writeError(w, http.StatusBadRequest, "bank_account_id is required")
		return
	}

	bankAccountID, err := strconv.ParseUint(bankAccountIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bank_account_id")
		return
	}

	grpcReq := &pb.DeleteBankAccountRequest{
		UserId:        userCtx.UserID,
		BankAccountId: bankAccountID,
	}

	_, err = h.kycClient.DeleteBankAccount(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}

// formatBankAccountResource builds a Laravel BankAccountResource-compatible JSON object.
func formatBankAccountResource(resp *pb.BankAccountResponse) map[string]interface{} {
	response := map[string]interface{}{
		"id":        resp.Id,
		"bank_name": resp.BankName,
		"shaba_num": resp.ShabaNum,
		"card_num":  resp.CardNum,
		"status":    resp.Status,
	}
	if parsed := parseBankAccountErrors(resp.Errors); parsed != nil {
		response["errors"] = parsed
	}
	return response
}

// parseBankAccountErrors decodes JSON errors from the database (Laravel casts errors as array).
func parseBankAccountErrors(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		return arr
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		return obj
	}
	return raw
}

// RequireVerifiedEmail enforces Laravel's verified middleware (EnsureEmailIsVerified).
func (h *AuthHandler) RequireVerifiedEmail(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, err := middleware.GetUserFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		user, err := h.userClient.GetUser(r.Context(), &pb.GetUserRequest{UserId: userCtx.UserID})
		if err != nil {
			h.writeGRPCErrorLocale(w, err)
			return
		}
		if user.EmailVerifiedAt == nil {
			writeError(w, http.StatusForbidden, "Your email address is not verified.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper functions

// requestHasBody reports whether the request may carry a body.
// ContentLength -1 (chunked / unset) is common for API clients such as Bruno and must not be treated as empty.
func requestHasBody(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	if r.ContentLength == 0 {
		return false
	}
	return r.ContentLength > 0 || r.ContentLength < 0
}

// decodeRequest decodes request data from query parameters, JSON body, or form-data
// It tries query parameters first, then falls back to body (JSON or form-data)
// This allows handlers to accept data from multiple sources
func decodeRequest(r *http.Request, v interface{}) error {
	// First, try to populate from query parameters
	queryErr := decodeQueryParams(r, v)

	// Check if body exists and has content
	if !requestHasBody(r) {
		// If no body, return query params result (even if empty, that's OK)
		return queryErr
	}

	// If we have a body, try to decode it
	contentType := r.Header.Get("Content-Type")
	var bodyErr error

	// Handle JSON requests
	if strings.HasPrefix(contentType, "application/json") {
		bodyErr = decodeJSONBody(r, v)
	} else if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Handle form-data requests
		bodyErr = decodeFormData(r, v)
	} else {
		// Default to JSON if content type is not specified or unknown
		bodyErr = decodeJSONBody(r, v)
	}

	// If body decoding succeeded, use it (body takes precedence over query params for same fields)
	// If body decoding failed but query params succeeded, use query params
	if bodyErr == nil {
		return nil
	}

	// If body decoding failed but query params succeeded, return query params result
	if queryErr == nil {
		return nil
	}

	// Both failed, return body error (more specific)
	return bodyErr
}

// decodeRequestBody decodes request body from JSON or form-data (multipart/form-data or application/x-www-form-urlencoded)
// It automatically detects the content type and handles both formats
// If the body is empty, it will also check query string parameters
func decodeRequestBody(r *http.Request, v interface{}) error {
	if !requestHasBody(r) {
		return decodeQueryParams(r, v)
	}

	contentType := r.Header.Get("Content-Type")
	var bodyErr error

	// Handle JSON requests
	if strings.HasPrefix(contentType, "application/json") {
		bodyErr = decodeJSONBody(r, v)
	} else if strings.HasPrefix(contentType, "multipart/form-data") || strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Handle form-data requests
		bodyErr = decodeFormData(r, v)
	} else {
		// Default to JSON if content type is not specified or unknown
		// This maintains backward compatibility
		bodyErr = decodeJSONBody(r, v)
	}

	// If body decoding succeeded, also merge query parameters (query params can supplement body data)
	if bodyErr == nil {
		// Try to merge query parameters (non-destructive - only sets fields that are zero values)
		mergeQueryParams(r, v)
		return nil
	}

	// If body decoding failed, try query parameters as fallback when present
	if len(r.URL.Query()) > 0 {
		if queryErr := decodeQueryParams(r, v); queryErr == nil {
			return nil
		}
	}

	// Both failed, return body error
	return bodyErr
}

// mergeQueryParams merges query parameters into a struct, only setting fields that are zero values
// This allows query params to supplement body data without overwriting existing values
func mergeQueryParams(r *http.Request, v interface{}) {
	queryValues := r.URL.Query()
	if len(queryValues) == 0 {
		return
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Only set if field is zero value (empty)
		if !isZeroValue(fieldValue) {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check for "query" tag as well
		if queryTag := field.Tag.Get("query"); queryTag != "" && queryTag != "-" {
			parts := strings.Split(queryTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching
		fieldNameLower := strings.ToLower(fieldName)

		// Try to find the query value
		var values []string
		var found bool
		if vals, ok := queryValues[fieldName]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldNameLower]; ok && len(vals) > 0 {
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Get the first value and set it
		value := values[0]
		setFieldValue(fieldValue, value) // Ignore errors for merge
	}
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Map, reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

// decodeQueryParams decodes query string parameters into a struct
func decodeQueryParams(r *http.Request, v interface{}) error {
	queryValues := r.URL.Query()

	// If no query parameters, return nil (not an error, just empty)
	if len(queryValues) == 0 {
		return nil
	}

	// Use reflection to populate the struct
	return populateStructFromQuery(queryValues, v)
}

// decodeJSONBody safely decodes JSON from request body, handling empty bodies
func decodeJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return io.EOF
	}

	// Check if body is empty
	if r.ContentLength == 0 {
		return io.EOF
	}

	// Try to peek at the body to see if it's already consumed
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Restore body for potential subsequent reads
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if len(bodyBytes) == 0 {
		return io.EOF
	}

	return json.Unmarshal(bodyBytes, v)
}

// decodeFormData decodes form-data (multipart/form-data or application/x-www-form-urlencoded) into a struct
func decodeFormData(r *http.Request, v interface{}) error {
	contentType := r.Header.Get("Content-Type")

	var formValues map[string][]string

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form
		err := r.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			return fmt.Errorf("failed to parse multipart form: %w", err)
		}
		formValues = r.MultipartForm.Value
	} else {
		// Parse URL-encoded form
		err := r.ParseForm()
		if err != nil {
			return fmt.Errorf("failed to parse form: %w", err)
		}
		formValues = r.PostForm
	}

	// Use reflection to populate the struct
	return populateStructFromForm(formValues, v)
}

// populateStructFromQuery populates a struct from query parameter values using reflection
// It uses JSON struct tags to map query parameter names to struct fields
func populateStructFromQuery(queryValues map[string][]string, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like "omitempty" from the tag
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Check for "query" tag as well
		if queryTag := field.Tag.Get("query"); queryTag != "" && queryTag != "-" {
			parts := strings.Split(queryTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching (common in query parameters)
		fieldNameLower := strings.ToLower(fieldName)

		// Handle array notation: try fieldName[] first, then fieldName
		var values []string
		var found bool

		// Try array notation first (e.g., points[])
		arrayNotation := fieldName + "[]"
		if vals, ok := queryValues[arrayNotation]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[strings.ToLower(arrayNotation)]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldName]; ok && len(vals) > 0 {
			// Try exact field name
			values = vals
			found = true
		} else if vals, ok := queryValues[fieldNameLower]; ok && len(vals) > 0 {
			// Try lowercase field name
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Handle slice/array fields specially - use all values
		if fieldValue.Kind() == reflect.Slice {
			// Create a new slice with the appropriate type
			sliceType := fieldValue.Type().Elem()
			slice := reflect.MakeSlice(fieldValue.Type(), len(values), len(values))

			for i, val := range values {
				elemValue := reflect.New(sliceType).Elem()
				if err := setFieldValue(elemValue, val); err != nil {
					return fmt.Errorf("failed to set slice element %s[%d]: %w", fieldName, i, err)
				}
				slice.Index(i).Set(elemValue)
			}

			fieldValue.Set(slice)
		} else {
			// For non-slice fields, get the first value
			value := values[0]
			if err := setFieldValue(fieldValue, value); err != nil {
				return fmt.Errorf("failed to set field %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// populateStructFromForm populates a struct from form values using reflection
// It uses JSON struct tags to map form field names to struct fields
func populateStructFromForm(formValues map[string][]string, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("v must be a pointer to a struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get the JSON tag name, or use the field name as fallback
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove options like "omitempty" from the tag
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Convert field name to lowercase for matching (common in form submissions)
		fieldNameLower := strings.ToLower(fieldName)

		// Try to find the form value by exact name first, then by lowercase
		var values []string
		var found bool
		if vals, ok := formValues[fieldName]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := formValues[fieldNameLower]; ok && len(vals) > 0 {
			values = vals
			found = true
		} else if vals, ok := formValues[field.Tag.Get("form")]; ok && len(vals) > 0 {
			// Also check for "form" tag
			values = vals
			found = true
		}

		if !found || len(values) == 0 {
			continue
		}

		// Get the first value (form fields can have multiple values, we take the first)
		value := values[0]

		// Set the field value based on its type
		if err := setFieldValue(fieldValue, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldName, err)
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string form value
func setFieldValue(fieldValue reflect.Value, value string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %w", err)
		}
		fieldValue.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %w", err)
		}
		fieldValue.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %w", err)
		}
		fieldValue.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			// Also accept "1", "true", "yes", "on" as true, and "0", "false", "no", "off" as false
			lowerValue := strings.ToLower(value)
			if lowerValue == "1" || lowerValue == "true" || lowerValue == "yes" || lowerValue == "on" {
				boolVal = true
			} else if lowerValue == "0" || lowerValue == "false" || lowerValue == "no" || lowerValue == "off" {
				boolVal = false
			} else {
				return fmt.Errorf("invalid boolean value: %w", err)
			}
		}
		fieldValue.SetBool(boolVal)
	default:
		return fmt.Errorf("unsupported field type: %s", fieldValue.Kind())
	}

	return nil
}

// writeJSON writes a JSON response, wrapping it in a "data" field unless:
// 1. The data is already wrapped (has a "data" key at top level)
// 2. The data is an error response (has "error" key)
// 3. skipWrap is true (for special cases like /auth/redirect)
func writeJSON(w http.ResponseWriter, status int, data interface{}, skipWrap ...bool) {
	shouldSkipWrap := len(skipWrap) > 0 && skipWrap[0]

	// Handle nil data
	if data == nil {
		data = map[string]interface{}{}
	}

	// Check if we should wrap the response
	if !shouldSkipWrap {
		// Use reflection to check the type
		dataType := reflect.TypeOf(data)
		dataKind := dataType.Kind()

		// Check if data is a map (objects)
		if dataKind == reflect.Map {
			if dataMap, ok := data.(map[string]interface{}); ok {
				// Already has "data" key - don't wrap again
				if _, hasData := dataMap["data"]; hasData {
					shouldSkipWrap = true
				}
				// Has "error" key - don't wrap error responses
				if _, hasError := dataMap["error"]; hasError {
					shouldSkipWrap = true
				}
				// Has "message" and "errors" keys - validation error, don't wrap
				if _, hasMessage := dataMap["message"]; hasMessage {
					if _, hasErrors := dataMap["errors"]; hasErrors {
						shouldSkipWrap = true
					}
				}
			} else if dataMap, ok := data.(map[string]string); ok {
				// Check for error or special responses in map[string]string
				if _, hasError := dataMap["error"]; hasError {
					shouldSkipWrap = true
				}
				// Laravel-style bare responses: {"url": "..."} or {"link": "..."}
				if _, hasURL := dataMap["url"]; hasURL {
					shouldSkipWrap = true
				}
				if _, hasLink := dataMap["link"]; hasLink {
					shouldSkipWrap = true
				}
			}
		}
		// Arrays, slices, and other non-map types (including []map[string]interface{})
		// will be wrapped in the "data" field below

		// Wrap in data field if not skipping
		if !shouldSkipWrap {
			data = map[string]interface{}{
				"data": data,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeGRPCError(w http.ResponseWriter, err error) {
	writeGRPCErrorWithLocale(w, err, "en")
}

func writeGRPCErrorWithLocale(w http.ResponseWriter, err error, locale string) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, st.Message())
	case codes.NotFound:
		writeError(w, http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		// Try to decode structured validation errors from service
		errorMsg := st.Message()
		if fields, decoded := helpers.DecodeValidationError(errorMsg); decoded {
			// Use decoded field errors
			helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
		} else {
			// Fallback: try to map error message to fields
			if fields, mapped := helpers.DecodeValidationError(errorMsg); mapped {
				helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
			} else {
				// Last resort: return as generic validation error
				helpers.WriteValidationErrorResponseFromString(w, errorMsg, locale)
			}
		}
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, st.Message())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, st.Message())
	case codes.FailedPrecondition:
		writeError(w, http.StatusPreconditionFailed, st.Message())
	case codes.Unavailable:
		// Service unavailable - likely connection issue
		writeError(w, http.StatusServiceUnavailable, "service temporarily unavailable: "+st.Message())
	default:
		writeError(w, http.StatusInternalServerError, st.Message())
	}
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	// Note: X-Forwarded-For can contain multiple IPs, take the first one
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For format: "client, proxy1, proxy2"
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr (format: "IP:port")
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// extractTokenFromHeader extracts Bearer token from Authorization header
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Try cookie as fallback
		cookie, err := r.Cookie("token")
		if err == nil && cookie != nil {
			return cookie.Value
		}
		return ""
	}

	// Check for Bearer token format
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		// If no Bearer prefix, assume the whole header is the token
		return authHeader
	}

	return strings.TrimPrefix(authHeader, bearerPrefix)
}

func extractIDFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimPrefix(path, prefix)
	// Remove any trailing slashes or query params
	id = strings.TrimSuffix(id, "/")
	if idx := strings.Index(id, "?"); idx != -1 {
		id = id[:idx]
	}
	return id
}

// HandleUsersRoutes handles all /api/users/{user}/... routes
func (h *AuthHandler) HandleUsersRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Path format: /api/users/{user}/levels, /api/users/{user}/profile, etc.

	// Remove prefix to get the dynamic part
	userPath := strings.TrimPrefix(path, "/api/users/")
	// Clean up the path - remove leading/trailing slashes and query params
	userPath = strings.Trim(userPath, "/")
	if idx := strings.Index(userPath, "?"); idx != -1 {
		userPath = userPath[:idx]
	}

	if userPath == "" {
		// This should not happen as /api/users is handled above, but handle it anyway
		http.NotFound(w, r)
		return
	}

	pathParts := strings.Split(userPath, "/")
	// Filter out empty parts
	var cleanParts []string
	for _, part := range pathParts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}

	if len(cleanParts) == 0 {
		http.NotFound(w, r)
		return
	}

	// First part is the user ID, rest is the endpoint
	endpoint := ""
	if len(cleanParts) > 1 {
		endpoint = cleanParts[1]
	}

	// Route to appropriate handler based on endpoint
	switch endpoint {
	case "levels":
		if r.Method == http.MethodGet {
			h.GetUserLevels(w, r)
		} else {
			http.NotFound(w, r)
		}
	case "profile":
		if r.Method == http.MethodGet {
			h.GetUserProfile(w, r)
		} else {
			http.NotFound(w, r)
		}
	case "wallet":
		if r.Method == http.MethodGet {
			h.GetUserWallet(w, r)
		} else {
			http.NotFound(w, r)
		}
	case "features":
		if len(cleanParts) >= 3 && cleanParts[2] == "count" {
			if r.Method == http.MethodGet {
				h.GetUserFeaturesCount(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else {
			http.NotFound(w, r)
		}
	case "profile-limitations":
		if r.Method == http.MethodGet {
			h.GetProfileLimitations(w, r)
		} else {
			http.NotFound(w, r)
		}
	default:
		// If no endpoint specified, treat as invalid
		http.NotFound(w, r)
	}
}

// ============================================================================
// Citizen Service Handlers (Public endpoints - no auth required)
// ============================================================================

// HandleCitizenRoutes handles all citizen-related routes
func (h *AuthHandler) HandleCitizenRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Path format: /api/citizen/{code} or /api/citizen/{code}/referrals or /api/citizen/{code}/referrals/chart

	// Extract code from path
	parts := strings.Split(strings.TrimPrefix(path, "/api/citizen/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "citizen code is required")
		return
	}

	code := parts[0]

	if len(parts) > 1 {
		switch parts[1] {
		case "referrals":
			if len(parts) > 2 && parts[2] == "chart" {
				// /api/citizen/{code}/referrals/chart
				h.GetCitizenReferralChart(w, r, code)
			} else {
				// /api/citizen/{code}/referrals
				h.GetCitizenReferrals(w, r, code)
			}
		default:
			writeError(w, http.StatusNotFound, "invalid citizen endpoint")
		}
	} else {
		// /api/citizen/{code}
		h.GetCitizenProfile(w, r, code)
	}
}

// GetCitizenProfile handles GET /api/citizen/{code}
func (h *AuthHandler) GetCitizenProfile(w http.ResponseWriter, r *http.Request, code string) {
	grpcReq := &pb.GetCitizenProfileRequest{
		Code: code,
	}

	resp, err := h.citizenClient.GetCitizenProfile(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, citizenProfileJSONRoundTrip(buildCitizenProfileHTTPResponse(resp)))
}

// GetCitizenReferrals handles GET /api/citizen/{code}/referrals
func (h *AuthHandler) GetCitizenReferrals(w http.ResponseWriter, r *http.Request, code string) {
	search := r.URL.Query().Get("search")
	pageStr := r.URL.Query().Get("page")
	page := int32(1)
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil {
			page = int32(p)
		}
	}

	grpcReq := &pb.GetCitizenReferralsRequest{
		Code:   code,
		Search: search,
		Page:   page,
	}

	resp, err := h.citizenClient.GetCitizenReferrals(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCitizenReferralsHTTPResponse(r, resp))
}

// GetCitizenReferralChart handles GET /api/citizen/{code}/referrals/chart
func (h *AuthHandler) GetCitizenReferralChart(w http.ResponseWriter, r *http.Request, code string) {
	rangeType := r.URL.Query().Get("range")
	if rangeType == "" {
		rangeType = "daily"
	}

	grpcReq := &pb.GetCitizenReferralChartRequest{
		Code:  code,
		Range: rangeType,
	}

	resp, err := h.citizenClient.GetCitizenReferralChart(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCitizenReferralChartHTTPResponse(resp))
}

// buildCitizenReferralsHTTPResponse formats referrals as Laravel simplePaginate JSON.
func buildCitizenReferralsHTTPResponse(r *http.Request, resp *pb.CitizenReferralsResponse) map[string]interface{} {
	const perPage int32 = 10

	currentPage := int32(1)
	hasMore := false
	if resp.Meta != nil {
		currentPage = resp.Meta.CurrentPage
		if currentPage <= 0 {
			currentPage = 1
		}
		hasMore = resp.Meta.NextPageUrl != ""
	}

	referrals := make([]map[string]interface{}, 0, len(resp.Data))
	for _, ref := range resp.Data {
		item := map[string]interface{}{
			"id":   ref.Id,
			"code": ref.Code,
			"name": ref.Name,
		}
		if ref.Image != "" {
			item["image"] = ref.Image
		}
		orders := make([]map[string]interface{}, 0, len(ref.ReferrerOrders))
		for _, order := range ref.ReferrerOrders {
			orders = append(orders, map[string]interface{}{
				"id":         order.Id,
				"amount":     order.Amount,
				"created_at": order.CreatedAt,
			})
		}
		item["referrerOrders"] = orders
		referrals = append(referrals, item)
	}

	response := map[string]interface{}{
		"data": referrals,
	}

	response["links"] = buildSimplePaginationLinks(r, currentPage, hasMore)

	itemCount := len(referrals)
	var from interface{}
	var to interface{}
	if itemCount > 0 {
		fromVal := int((currentPage-1)*perPage) + 1
		from = fromVal
		to = fromVal + itemCount - 1
	}

	response["meta"] = map[string]interface{}{
		"current_page": currentPage,
		"from":         from,
		"path":         requestPath(r),
		"per_page":     perPage,
		"to":           to,
	}

	return response
}

// buildCitizenReferralChartHTTPResponse formats chart stats with a single Laravel-style data wrapper.
func buildCitizenReferralChartHTTPResponse(resp *pb.CitizenReferralChartResponse) map[string]interface{} {
	chartPayload := map[string]interface{}{
		"total_referrals_count":        "0",
		"total_referral_orders_amount": "0",
		"chart_data":                   []interface{}{},
	}

	if resp.Data != nil {
		chartPayload["total_referrals_count"] = resp.Data.TotalReferralsCount
		chartPayload["total_referral_orders_amount"] = resp.Data.TotalReferralOrdersAmount

		chartData := make([]map[string]interface{}, 0, len(resp.Data.ChartData))
		for _, point := range resp.Data.ChartData {
			chartData = append(chartData, map[string]interface{}{
				"label":        point.Label,
				"count":        point.Count,
				"total_amount": point.TotalAmount,
			})
		}
		chartPayload["chart_data"] = chartData
	}

	return map[string]interface{}{
		"data": chartPayload,
	}
}

// ============================================================================
// Personal Info Service Handlers
// ============================================================================

// GetPersonalInfo handles GET /api/personal-info
func (h *AuthHandler) GetPersonalInfo(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.GetPersonalInfoRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.personalInfoClient.GetPersonalInfo(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Check if personal info exists (has any data)
	// If all fields are empty/null, return empty array per Laravel API spec
	if resp.Data == nil || !hasPersonalInfoData(resp.Data) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{},
		})
		return
	}

	// Convert PersonalInfoData to Laravel-compatible format
	data := map[string]interface{}{}
	if resp.Data.Occupation != "" {
		data["occupation"] = resp.Data.Occupation
	}
	if resp.Data.Education != "" {
		data["education"] = resp.Data.Education
	}
	if resp.Data.Memory != "" {
		data["memory"] = resp.Data.Memory
	}
	if resp.Data.LovedCity != "" {
		data["loved_city"] = resp.Data.LovedCity
	}
	if resp.Data.LovedCountry != "" {
		data["loved_country"] = resp.Data.LovedCountry
	}
	if resp.Data.LovedLanguage != "" {
		data["loved_language"] = resp.Data.LovedLanguage
	}
	if resp.Data.ProblemSolving != "" {
		data["problem_solving"] = resp.Data.ProblemSolving
	}
	if resp.Data.Prediction != "" {
		data["prediction"] = resp.Data.Prediction
	}
	if resp.Data.About != "" {
		data["about"] = resp.Data.About
	}
	if len(resp.Data.Passions) > 0 {
		data["passions"] = resp.Data.Passions
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": data,
	})
}

// hasPersonalInfoData checks if PersonalInfoData has any non-empty values
func hasPersonalInfoData(data *pb.PersonalInfoData) bool {
	if data == nil {
		return false
	}
	if data.Occupation != "" || data.Education != "" || data.Memory != "" ||
		data.LovedCity != "" || data.LovedCountry != "" || data.LovedLanguage != "" ||
		data.ProblemSolving != "" || data.Prediction != "" || data.About != "" {
		return true
	}
	// Check if any passion is true
	if data.Passions != nil {
		for _, value := range data.Passions {
			if value {
				return true
			}
		}
	}
	return false
}

// UpdatePersonalInfo handles PUT/PATCH /api/personal-info
func (h *AuthHandler) UpdatePersonalInfo(w http.ResponseWriter, r *http.Request) {
	// Accept Laravel-style POST + _method=put|patch (same as KYC / bank-accounts routing)
	if m := EffectiveHTTPMethod(r); m != http.MethodPut && m != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Occupation     string          `json:"occupation"`
		Education      string          `json:"education"`
		Memory         string          `json:"memory"`
		LovedCity      string          `json:"loved_city"`
		LovedCountry   string          `json:"loved_country"`
		LovedLanguage  string          `json:"loved_language"`
		ProblemSolving string          `json:"problem_solving"`
		Prediction     string          `json:"prediction"`
		About          string          `json:"about"`
		Passions       map[string]bool `json:"passions"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.UpdatePersonalInfoRequest{
		UserId:         userCtx.UserID,
		Occupation:     req.Occupation,
		Education:      req.Education,
		Memory:         req.Memory,
		LovedCity:      req.LovedCity,
		LovedCountry:   req.LovedCountry,
		LovedLanguage:  req.LovedLanguage,
		ProblemSolving: req.ProblemSolving,
		Prediction:     req.Prediction,
		About:          req.About,
		Passions:       req.Passions,
	}

	_, err = h.personalInfoClient.UpdatePersonalInfo(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Profile Limitation Service Handlers
// ============================================================================

// CreateProfileLimitation handles POST /api/profile-limitations
func (h *AuthHandler) CreateProfileLimitation(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		LimitedUserID uint64 `json:"limited_user_id"`
		Options       struct {
			Follow                bool `json:"follow"`
			SendMessage           bool `json:"send_message"`
			Share                 bool `json:"share"`
			SendTicket            bool `json:"send_ticket"`
			ViewProfileImages     bool `json:"view_profile_images"`
			ViewFeaturesLocations bool `json:"view_features_locations"`
		} `json:"options"`
		Note string `json:"note,omitempty"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	// Validate that all six options are provided
	grpcReq := &pb.CreateProfileLimitationRequest{
		LimiterUserId: userCtx.UserID,
		LimitedUserId: req.LimitedUserID,
		Options: &pb.ProfileLimitationOptions{
			Follow:                req.Options.Follow,
			SendMessage:           req.Options.SendMessage,
			Share:                 req.Options.Share,
			SendTicket:            req.Options.SendTicket,
			ViewProfileImages:     req.Options.ViewProfileImages,
			ViewFeaturesLocations: req.Options.ViewFeaturesLocations,
		},
		Note: req.Note,
	}

	resp, err := h.profileLimitationClient.CreateProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel API spec: { "data": {...} }
	data := map[string]interface{}{
		"id":              resp.Data.Id,
		"limiter_user_id": resp.Data.LimiterUserId,
		"limited_user_id": resp.Data.LimitedUserId,
		"options": map[string]bool{
			"follow":                  resp.Data.Options.Follow,
			"send_message":            resp.Data.Options.SendMessage,
			"share":                   resp.Data.Options.Share,
			"send_ticket":             resp.Data.Options.SendTicket,
			"view_profile_images":     resp.Data.Options.ViewProfileImages,
			"view_features_locations": resp.Data.Options.ViewFeaturesLocations,
		},
		"created_at": resp.Data.CreatedAt,
		"updated_at": resp.Data.UpdatedAt,
	}

	if resp.Data.Note != "" {
		data["note"] = resp.Data.Note
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": data})
}

// UpdateProfileLimitation handles PUT/PATCH /api/profile-limitations/{limitation_id}
func (h *AuthHandler) UpdateProfileLimitation(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limitationIDStr := extractIDFromPath(r.URL.Path, "/api/profile-limitations/")
	if limitationIDStr == "" {
		writeError(w, http.StatusBadRequest, "limitation_id is required")
		return
	}

	limitationID, err := strconv.ParseUint(limitationIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid limitation_id")
		return
	}

	var req struct {
		Options struct {
			Follow                bool `json:"follow"`
			SendMessage           bool `json:"send_message"`
			Share                 bool `json:"share"`
			SendTicket            bool `json:"send_ticket"`
			ViewProfileImages     bool `json:"view_profile_images"`
			ViewFeaturesLocations bool `json:"view_features_locations"`
		} `json:"options"`
		Note string `json:"note,omitempty"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.UpdateProfileLimitationRequest{
		LimitationId:  limitationID,
		LimiterUserId: userCtx.UserID,
		Options: &pb.ProfileLimitationOptions{
			Follow:                req.Options.Follow,
			SendMessage:           req.Options.SendMessage,
			Share:                 req.Options.Share,
			SendTicket:            req.Options.SendTicket,
			ViewProfileImages:     req.Options.ViewProfileImages,
			ViewFeaturesLocations: req.Options.ViewFeaturesLocations,
		},
		Note: req.Note,
	}

	resp, err := h.profileLimitationClient.UpdateProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel API spec: { "data": {...} }
	data := map[string]interface{}{
		"id":              resp.Data.Id,
		"limiter_user_id": resp.Data.LimiterUserId,
		"limited_user_id": resp.Data.LimitedUserId,
		"options": map[string]bool{
			"follow":                  resp.Data.Options.Follow,
			"send_message":            resp.Data.Options.SendMessage,
			"share":                   resp.Data.Options.Share,
			"send_ticket":             resp.Data.Options.SendTicket,
			"view_profile_images":     resp.Data.Options.ViewProfileImages,
			"view_features_locations": resp.Data.Options.ViewFeaturesLocations,
		},
		"created_at": resp.Data.CreatedAt,
		"updated_at": resp.Data.UpdatedAt,
	}

	// Only include note if caller is the limiter
	if resp.Data.Note != "" && userCtx.UserID == resp.Data.LimiterUserId {
		data["note"] = resp.Data.Note
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// DeleteProfileLimitation handles DELETE /api/profile-limitations/{limitation_id}
func (h *AuthHandler) DeleteProfileLimitation(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limitationIDStr := extractIDFromPath(r.URL.Path, "/api/profile-limitations/")
	if limitationIDStr == "" {
		writeError(w, http.StatusBadRequest, "limitation_id is required")
		return
	}

	limitationID, err := strconv.ParseUint(limitationIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid limitation_id")
		return
	}

	grpcReq := &pb.DeleteProfileLimitationRequest{
		LimitationId:  limitationID,
		LimiterUserId: userCtx.UserID,
	}

	_, err = h.profileLimitationClient.DeleteProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetProfileLimitation handles GET /api/profile-limitations/{limitation_id}
func (h *AuthHandler) GetProfileLimitation(w http.ResponseWriter, r *http.Request) {
	limitationIDStr := extractIDFromPath(r.URL.Path, "/api/profile-limitations/")
	if limitationIDStr == "" {
		writeError(w, http.StatusBadRequest, "limitation_id is required")
		return
	}

	limitationID, err := strconv.ParseUint(limitationIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid limitation_id")
		return
	}

	grpcReq := &pb.GetProfileLimitationRequest{
		LimitationId: limitationID,
	}

	resp, err := h.profileLimitationClient.GetProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response according to Laravel API spec: { "data": {...} }
	// Get user from context to determine note visibility
	userCtx, err := middleware.GetUserFromRequest(r)
	callerUserID := uint64(0)
	if err == nil {
		callerUserID = userCtx.UserID
	}

	data := map[string]interface{}{
		"id":              resp.Data.Id,
		"limiter_user_id": resp.Data.LimiterUserId,
		"limited_user_id": resp.Data.LimitedUserId,
		"options": map[string]bool{
			"follow":                  resp.Data.Options.Follow,
			"send_message":            resp.Data.Options.SendMessage,
			"share":                   resp.Data.Options.Share,
			"send_ticket":             resp.Data.Options.SendTicket,
			"view_profile_images":     resp.Data.Options.ViewProfileImages,
			"view_features_locations": resp.Data.Options.ViewFeaturesLocations,
		},
		"created_at": resp.Data.CreatedAt,
		"updated_at": resp.Data.UpdatedAt,
	}

	// Only include note if caller is the limiter (as per API documentation)
	if resp.Data.Note != "" && callerUserID == resp.Data.LimiterUserId {
		data["note"] = resp.Data.Note
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// ============================================================================
// Profile Photo Service Handlers
// ============================================================================

// ListProfilePhotos handles GET /api/profilePhotos
func (h *AuthHandler) ListProfilePhotos(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	grpcReq := &pb.ListProfilePhotosRequest{
		UserId: userCtx.UserID,
	}

	resp, err := h.profilePhotoClient.ListProfilePhotos(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel ProfilePhotoResource: { "data": [...] }
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": resp.Data,
	})
}

// UploadProfilePhoto handles POST /api/profilePhotos
func (h *AuthHandler) UploadProfilePhoto(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer file.Close()

	// Read file data
	imageData := make([]byte, header.Size)
	if _, err := file.Read(imageData); err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image data")
		return
	}

	grpcReq := &pb.UploadProfilePhotoRequest{
		UserId:      userCtx.UserID,
		ImageData:   imageData,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
	}

	resp, err := h.profilePhotoClient.UploadProfilePhoto(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel ProfilePhotoResource: { "id": ..., "url": ... }
	writeJSON(w, http.StatusCreated, resp)
}

// GetProfilePhoto handles GET /api/profilePhotos/{profilePhoto}
func (h *AuthHandler) GetProfilePhoto(w http.ResponseWriter, r *http.Request) {
	profilePhotoIDStr := extractIDFromPath(r.URL.Path, "/api/profilePhotos/")
	if profilePhotoIDStr == "" {
		writeError(w, http.StatusBadRequest, "profile_photo_id is required")
		return
	}

	profilePhotoID, err := strconv.ParseUint(profilePhotoIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile_photo_id")
		return
	}

	grpcReq := &pb.GetProfilePhotoRequest{
		ProfilePhotoId: profilePhotoID,
	}

	resp, err := h.profilePhotoClient.GetProfilePhoto(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel ProfilePhotoResource: { "id": ..., "url": ... }
	writeJSON(w, http.StatusOK, resp)
}

// DeleteProfilePhoto handles DELETE /api/profilePhotos/{profilePhoto}
func (h *AuthHandler) DeleteProfilePhoto(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	profilePhotoIDStr := extractIDFromPath(r.URL.Path, "/api/profilePhotos/")
	if profilePhotoIDStr == "" {
		writeError(w, http.StatusBadRequest, "profile_photo_id is required")
		return
	}

	profilePhotoID, err := strconv.ParseUint(profilePhotoIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid profile_photo_id")
		return
	}

	grpcReq := &pb.DeleteProfilePhotoRequest{
		UserId:         userCtx.UserID,
		ProfilePhotoId: profilePhotoID,
	}

	_, err = h.profilePhotoClient.DeleteProfilePhoto(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Settings Service Handlers
// ============================================================================

// GetSettings handles GET /api/settings
func (h *AuthHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
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

	// Response format: { "checkout_days_count": ..., "automatic_logout": ... }
	response := map[string]interface{}{
		"checkout_days_count": resp.Data.CheckoutDaysCount,
		"automatic_logout":    resp.Data.AutomaticLogout,
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateSettings handles POST /api/settings
func (h *AuthHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) GetGeneralSettings(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) UpdateGeneralSettings(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) GetPrivacySettings(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) UpdatePrivacySettings(w http.ResponseWriter, r *http.Request) {
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

// ============================================================================
// User Events Service Handlers
// ============================================================================

// ListUserEvents handles GET /api/events
func (h *AuthHandler) ListUserEvents(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Get page from query parameter
	pageStr := r.URL.Query().Get("page")
	page := int32(1)
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	grpcReq := &pb.ListUserEventsRequest{
		UserId: userCtx.UserID,
		Page:   page,
	}

	resp, err := h.userEventsClient.ListUserEvents(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel UserEventResourceCollection
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": resp.Data,
		"links": map[string]interface{}{
			"next": resp.Pagination.NextPageUrl,
			"prev": resp.Pagination.PrevPageUrl,
		},
		"meta": map[string]interface{}{
			"current_page": resp.Pagination.CurrentPage,
		},
	})
}

// GetUserEvent handles GET /api/events/{userEvent}
func (h *AuthHandler) GetUserEvent(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract event ID from path: /api/events/{userEvent}
	eventIDStr := extractIDFromPath(r.URL.Path, "/api/events/")
	if eventIDStr == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event_id")
		return
	}

	grpcReq := &pb.GetUserEventRequest{
		UserId:  userCtx.UserID,
		EventId: eventID,
	}

	resp, err := h.userEventsClient.GetUserEvent(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel UserEventResource
	writeJSON(w, http.StatusOK, resp.Data)
}

// ReportUserEvent handles POST /api/events/report/{userEvent}
func (h *AuthHandler) ReportUserEvent(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract event ID from path: /api/events/report/{userEvent}
	eventIDStr := extractIDFromPath(r.URL.Path, "/api/events/report/")
	if eventIDStr == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event_id")
		return
	}

	var req struct {
		SuspeciousCitizen string `json:"suspecious_citizen,omitempty"`
		EventDescription  string `json:"event_description"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.ReportUserEventRequest{
		UserId:            userCtx.UserID,
		EventId:           eventID,
		SuspeciousCitizen: req.SuspeciousCitizen,
		EventDescription:  req.EventDescription,
	}

	resp, err := h.userEventsClient.ReportUserEvent(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel UserEventReportResource
	writeJSON(w, http.StatusCreated, resp.Data)
}

// SendReportResponse handles POST /api/events/report/response/{userEvent}
func (h *AuthHandler) SendReportResponse(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract event ID from path: /api/events/report/response/{userEvent}
	eventIDStr := extractIDFromPath(r.URL.Path, "/api/events/report/response/")
	if eventIDStr == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event_id")
		return
	}

	var req struct {
		Response string `json:"response"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	grpcReq := &pb.SendReportResponseRequest{
		UserId:   userCtx.UserID,
		EventId:  eventID,
		Response: req.Response,
	}

	resp, err := h.userEventsClient.SendReportResponse(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Format response to match Laravel UserEventReportResponseResource
	writeJSON(w, http.StatusCreated, resp.Data)
}

// CloseEventReport handles POST /api/events/report/close/{userEvent}
func (h *AuthHandler) CloseEventReport(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract event ID from path: /api/events/report/close/{userEvent}
	eventIDStr := extractIDFromPath(r.URL.Path, "/api/events/report/close/")
	if eventIDStr == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event_id")
		return
	}

	grpcReq := &pb.CloseEventReportRequest{
		UserId:  userCtx.UserID,
		EventId: eventID,
	}

	_, err = h.userEventsClient.CloseEventReport(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	// Return 204 No Content per API specification
	w.WriteHeader(http.StatusNoContent)
}

// SearchUsers handles POST /api/search/users
func (h *AuthHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) SearchFeatures(w http.ResponseWriter, r *http.Request) {
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
func (h *AuthHandler) SearchIsicCodes(w http.ResponseWriter, r *http.Request) {
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
