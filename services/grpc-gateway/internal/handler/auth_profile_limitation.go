// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"metarang/grpc-gateway/internal/middleware"
	pb "metarang/shared/pb/auth"
)

// GetProfileLimitations handles GET /api/users/{user}/profile-limitations
func (h *AuthHandler) GetProfileLimitations(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	callerUserID := userCtx.UserID

	targetUserIDStr := ""
	if strings.HasPrefix(r.URL.Path, "/api/users/") {
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			targetUserIDStr = pathParts[0]
		}
	}

	if targetUserIDStr == "" {
		writeError(w, http.StatusBadRequest, "target user_id is required in path /api/users/{user}/profile-limitations")
		return
	}

	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 64)
	if err != nil || targetUserID == 0 {
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

	if resp.Data == nil || resp.Data.Id == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": profileLimitationResourceJSON(resp.Data, callerUserID),
	})
}

// ============================================================================
// Profile Limitation Service Handlers
// ============================================================================

// CreateProfileLimitation handles POST /api/profile-limitations
func (h *AuthHandler) CreateProfileLimitation(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	input, fieldErrors := parseCreateProfileLimitationBody(r)
	if fieldErrors != nil {
		writeProfileLimitationValidationErrors(w, fieldErrors, h.locale)
		return
	}

	grpcReq := &pb.CreateProfileLimitationRequest{
		LimiterUserId: userCtx.UserID,
		LimitedUserId: input.LimitedUserID,
		Options:       input.Options,
		Note:          notePtrFromInput(input.Note),
	}

	resp, err := h.profileLimitationClient.CreateProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"data": profileLimitationResourceJSON(resp.Data, userCtx.UserID),
	})
}

// UpdateProfileLimitation handles PUT/PATCH /api/profile-limitations/{limitation_id}
func (h *AuthHandler) UpdateProfileLimitation(w http.ResponseWriter, r *http.Request) {
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

	input, fieldErrors := parseUpdateProfileLimitationBody(r)
	if fieldErrors != nil {
		writeProfileLimitationValidationErrors(w, fieldErrors, h.locale)
		return
	}

	grpcReq := &pb.UpdateProfileLimitationRequest{
		LimitationId:  limitationID,
		LimiterUserId: userCtx.UserID,
		Options:       input.Options,
		Note:          notePtrFromInput(input.Note),
	}

	resp, err := h.profileLimitationClient.UpdateProfileLimitation(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": profileLimitationResourceJSON(resp.Data, userCtx.UserID),
	})
}

// DeleteProfileLimitation handles DELETE /api/profile-limitations/{limitation_id}
func (h *AuthHandler) DeleteProfileLimitation(w http.ResponseWriter, r *http.Request) {
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
