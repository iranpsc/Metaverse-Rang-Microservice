// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"net/http"
	"strconv"

	"metarang/grpc-gateway/internal/middleware"
	pb "metarang/shared/pb/auth"
)

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
	defer func() { _ = file.Close() }()

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
