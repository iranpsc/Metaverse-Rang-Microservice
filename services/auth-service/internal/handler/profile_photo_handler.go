package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
	storagepb "metargb/shared/pb/storage"
)

// ProfilePhotoHandler handles profile photo gRPC requests
// Exported for testing purposes
type ProfilePhotoHandler struct {
	pb.UnimplementedProfilePhotoServiceServer
	ProfilePhotoService service.ProfilePhotoService
	StorageClient       storagepb.FileStorageServiceClient
	ApiGatewayURL       string
}

func RegisterProfilePhotoHandler(grpcServer *grpc.Server, profilePhotoService service.ProfilePhotoService, storageClient storagepb.FileStorageServiceClient, apiGatewayURL string) {
	pb.RegisterProfilePhotoServiceServer(grpcServer, &ProfilePhotoHandler{
		ProfilePhotoService: profilePhotoService,
		StorageClient:       storageClient,
		ApiGatewayURL:       apiGatewayURL,
	})
}

// PrependGatewayURL prepends the API gateway URL to the image URL if it's not already a full URL
// Exported for testing purposes
func (h *ProfilePhotoHandler) PrependGatewayURL(url string) string {
	if url == "" {
		return url
	}
	// If URL already starts with http:// or https://, return as is
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	// If API gateway URL is not set, return the original URL
	if h.ApiGatewayURL == "" {
		return url
	}
	// Remove leading slash from URL if present
	url = strings.TrimPrefix(url, "/")
	// Remove trailing slash from gateway URL if present
	gatewayURL := strings.TrimSuffix(h.ApiGatewayURL, "/")
	// Prepend gateway URL
	return gatewayURL + "/" + url
}

// ListProfilePhotos returns all profile photos for the authenticated user
func (h *ProfilePhotoHandler) ListProfilePhotos(ctx context.Context, req *pb.ListProfilePhotosRequest) (*pb.ListProfilePhotosResponse, error) {
	locale := getProjectLocale()
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "user_id is required"))
	}

	photos, err := h.ProfilePhotoService.ListProfilePhotos(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to list profile photos: %v", err))
	}

	response := &pb.ListProfilePhotosResponse{
		Data: make([]*pb.ProfilePhoto, 0, len(photos)),
	}

	for _, photo := range photos {
		// URLs are now stored as full URLs in the database, but keep fallback for existing records
		url := photo.URL
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			// Fallback: prepend gateway URL for old records with relative URLs
			url = h.PrependGatewayURL(url)
		}
		response.Data = append(response.Data, &pb.ProfilePhoto{
			Id:  photo.ID,
			Url: url,
		})
	}

	return response, nil
}

// UploadProfilePhoto uploads a new profile photo for the authenticated user
func (h *ProfilePhotoHandler) UploadProfilePhoto(ctx context.Context, req *pb.UploadProfilePhotoRequest) (*pb.ProfilePhotoResponse, error) {
	locale := getProjectLocale()
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "user_id is required"))
	}

	if len(req.ImageData) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "image_data is required"))
	}

	if req.Filename == "" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "filename is required"))
	}

	if req.ContentType == "" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "content_type is required"))
	}

	// Validate file size (≤1 MB = 1024 * 1024 bytes)
	const maxSize = 1024 * 1024
	if len(req.ImageData) > maxSize {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "invalid image: must be PNG or JPEG, ≤1 MB"))
	}

	// Validate content type
	contentType := strings.ToLower(req.ContentType)
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/jpg" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "invalid image: must be PNG or JPEG, ≤1 MB"))
	}

	// Validate filename extension
	filenameLower := strings.ToLower(req.Filename)
	if !strings.HasSuffix(filenameLower, ".png") && !strings.HasSuffix(filenameLower, ".jpg") && !strings.HasSuffix(filenameLower, ".jpeg") {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "invalid image: must be PNG or JPEG, ≤1 MB"))
	}

	// Upload file to storage-service
	if h.StorageClient == nil {
		return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service not available"))
	}

	// Create upload ID for chunk upload
	uploadID := fmt.Sprintf("profile_photo_%d_%d", req.UserId, time.Now().UnixNano())

	// Upload file using ChunkUpload (single chunk since file is small)
	chunkReq := &storagepb.ChunkUploadRequest{
		UploadId:    uploadID,
		ChunkData:   req.ImageData,
		ChunkIndex:  0,
		TotalChunks: 1,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		TotalSize:   int64(len(req.ImageData)),
		UploadPath:  "/uploads/profile", // Upload path for profile photos
	}

	chunkResp, err := h.StorageClient.ChunkUpload(ctx, chunkReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to upload file to storage service: %v", err))
	}

	if !chunkResp.Success {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "storage service upload failed: %s", chunkResp.Message))
	}

	if !chunkResp.IsFinished {
		return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service upload did not complete"))
	}

	// Get file path from storage service response
	// FileUrl contains the directory path (e.g., "uploads/image-jpeg/2024-01-01/")
	// FilePath contains the filename
	// Combine them to get the full path
	dirPath := chunkResp.FileUrl
	filename := chunkResp.FilePath
	if filename == "" {
		// Fallback to FinalFilename if FilePath is not set
		filename = chunkResp.FinalFilename
	}

	if dirPath == "" {
		return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service did not return file directory path"))
	}
	if filename == "" {
		return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service did not return filename"))
	}

	// Construct full path: directory + filename
	fullPath := strings.TrimSuffix(dirPath, "/") + "/" + filename

	// Create database record with the full file path from storage-service
	photo, err := h.ProfilePhotoService.CreateProfilePhotoRecord(ctx, req.UserId, fullPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to create profile photo record: %v", err))
	}

	// URLs are now stored as full URLs in the database, but keep fallback for existing records
	url := photo.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Fallback: prepend gateway URL for old records with relative URLs
		url = h.PrependGatewayURL(url)
	}
	return &pb.ProfilePhotoResponse{
		Id:  photo.ID,
		Url: url,
	}, nil
}

// GetProfilePhoto retrieves a profile photo by ID
func (h *ProfilePhotoHandler) GetProfilePhoto(ctx context.Context, req *pb.GetProfilePhotoRequest) (*pb.ProfilePhotoResponse, error) {
	locale := getProjectLocale()
	if req.ProfilePhotoId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "profile_photo_id is required"))
	}

	photo, err := h.ProfilePhotoService.GetProfilePhoto(ctx, req.ProfilePhotoId)
	if err != nil {
		switch err {
		case service.ErrProfilePhotoNotFound:
			return nil, status.Errorf(codes.NotFound, lang.T(locale, "profile photo not found"))
		default:
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get profile photo: %v", err))
		}
	}

	// URLs are now stored as full URLs in the database, but keep fallback for existing records
	url := photo.URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Fallback: prepend gateway URL for old records with relative URLs
		url = h.PrependGatewayURL(url)
	}
	return &pb.ProfilePhotoResponse{
		Id:  photo.ID,
		Url: url,
	}, nil
}

// DeleteProfilePhoto deletes a profile photo (with ownership check)
func (h *ProfilePhotoHandler) DeleteProfilePhoto(ctx context.Context, req *pb.DeleteProfilePhotoRequest) (*emptypb.Empty, error) {
	locale := getProjectLocale()
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "user_id is required"))
	}

	if req.ProfilePhotoId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "profile_photo_id is required"))
	}

	err := h.ProfilePhotoService.DeleteProfilePhoto(ctx, req.UserId, req.ProfilePhotoId)
	if err != nil {
		switch err {
		case service.ErrProfilePhotoNotFound:
			return nil, status.Errorf(codes.NotFound, lang.T(locale, "profile photo not found"))
		case service.ErrUnauthorized:
			return nil, status.Errorf(codes.PermissionDenied, lang.T(locale, "unauthorized: profile photo does not belong to user"))
		default:
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to delete profile photo: %v", err))
		}
	}

	return &emptypb.Empty{}, nil
}
