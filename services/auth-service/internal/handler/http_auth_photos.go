// Package handler provides Kong-facing HTTP handlers for auth-service.
package handler

import (
	"io"
	"net/http"
	"strconv"

	"metarang/auth-service/internal/middleware"
	pb "metarang/shared/pb/auth"
)

const maxProfilePhotoSize = 10 << 20

// ============================================================================
// Profile Photo Service Handlers
// ============================================================================

// ListProfilePhotos handles GET /api/profilePhotos
func (h *HTTPAuthHandler) ListProfilePhotos(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": resp.Data,
	})
}

// UploadProfilePhoto handles POST /api/profilePhotos
func (h *HTTPAuthHandler) UploadProfilePhoto(w http.ResponseWriter, r *http.Request) {
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
	defer func() { _ = file.Close() }()

	if header.Size < 0 || header.Size > maxProfilePhotoSize {
		writeError(w, http.StatusBadRequest, "image file too large")
		return
	}

	imageData, err := io.ReadAll(io.LimitReader(file, maxProfilePhotoSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image data")
		return
	}
	if int64(len(imageData)) > maxProfilePhotoSize {
		writeError(w, http.StatusBadRequest, "image file too large")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(imageData)
	}

	grpcReq := &pb.UploadProfilePhotoRequest{
		UserId:      userCtx.UserID,
		ImageData:   imageData,
		Filename:    header.Filename,
		ContentType: contentType,
	}

	resp, err := h.profilePhotoClient.UploadProfilePhoto(r.Context(), grpcReq)
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// GetProfilePhoto handles GET /api/profilePhotos/{profilePhoto}
func (h *HTTPAuthHandler) GetProfilePhoto(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, resp)
}

// DeleteProfilePhoto handles DELETE /api/profilePhotos/{profilePhoto}
func (h *HTTPAuthHandler) DeleteProfilePhoto(w http.ResponseWriter, r *http.Request) {
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
