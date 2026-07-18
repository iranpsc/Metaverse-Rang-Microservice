// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"metarang/grpc-gateway/internal/middleware"
	pb "metarang/shared/pb/auth"
)

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
	defer func() { _ = file.Close() }()

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
