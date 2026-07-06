package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonpb "metargb/shared/pb/common"
	trainingpb "metargb/shared/pb/training"
	"metargb/training-service/internal/service"
)

type VideoHandler struct {
	trainingpb.UnimplementedVideoServiceServer
	service *service.VideoService
}

func RegisterVideoHandler(grpcServer *grpc.Server, svc *service.VideoService) {
	handler := &VideoHandler{service: svc}
	trainingpb.RegisterVideoServiceServer(grpcServer, handler)
}

// GetVideos retrieves paginated videos
func (h *VideoHandler) GetVideos(ctx context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
	page := int32(1)
	perPage := int32(18) // Default per API spec

	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			perPage = req.Pagination.PerPage
		}
	}

	var categoryID, subCategoryID *uint64
	if req.CategoryId > 0 {
		categoryID = &req.CategoryId
	}
	if req.SubCategoryId > 0 {
		subCategoryID = &req.SubCategoryId
	}

	videos, total, err := h.service.GetVideos(ctx, page, perPage, categoryID, subCategoryID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get videos: %v", err)
	}

	response := &trainingpb.VideosResponse{
		Videos: make([]*trainingpb.VideoResponse, 0, len(videos)),
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}

	for _, video := range videos {
		details, err := h.service.GetVideoWithDetails(ctx, video, nil)
		if err != nil {
			continue // Skip videos with errors
		}
		videoResp, err := buildVideoResponse(details)
		if err != nil {
			continue
		}
		response.Videos = append(response.Videos, videoResp)
	}

	return response, nil
}

// GetVideo retrieves a video by slug and increments view
func (h *VideoHandler) GetVideo(ctx context.Context, req *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error) {
	ipAddress := IPAddressFromGRPCContext(ctx)
	var userID *uint64
	if req.UserId > 0 {
		userID = &req.UserId
	}

	video, err := h.service.GetVideoBySlug(ctx, req.Slug, userID, ipAddress)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "video not found: %v", err)
	}

	details, err := h.service.GetVideoWithDetails(ctx, video, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get video details: %v", err)
	}

	resp, err := buildVideoResponse(details)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	setVideoInteractionHeader(ctx, details.UserInteraction)
	return resp, nil
}

// GetVideoByFileName retrieves a video by partial file name and increments view
func (h *VideoHandler) GetVideoByFileName(ctx context.Context, req *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error) {
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = IPAddressFromGRPCContext(ctx)
	}

	video, err := h.service.GetVideoByFileName(ctx, req.FileName, ipAddress)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "video not found: %v", err)
	}

	details, err := h.service.GetVideoWithDetails(ctx, video, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get video details: %v", err)
	}

	resp, err := buildVideoResponse(details)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return resp, nil
}

// SearchVideos searches videos by title
func (h *VideoHandler) SearchVideos(ctx context.Context, req *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error) {
	validationErrors := validateRequired("query", req.Query, getLocale(ctx))
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	page := int32(1)
	perPage := int32(18)

	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			perPage = req.Pagination.PerPage
		}
	}

	videos, total, err := h.service.SearchVideos(ctx, req.Query, page, perPage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search videos: %v", err)
	}

	response := &trainingpb.VideosResponse{
		Videos: make([]*trainingpb.VideoResponse, 0, len(videos)),
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}

	for _, video := range videos {
		details, err := h.service.GetVideoWithDetails(ctx, video, nil)
		if err != nil {
			continue
		}
		videoResp, err := buildVideoResponse(details)
		if err != nil {
			continue
		}
		response.Videos = append(response.Videos, videoResp)
	}

	return response, nil
}

// IncrementView increments view count for a video
func (h *VideoHandler) IncrementView(ctx context.Context, req *trainingpb.IncrementViewRequest) (*commonpb.Empty, error) {
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = IPAddressFromGRPCContext(ctx)
	}

	if err := h.service.IncrementView(ctx, req.VideoId, ipAddress); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to increment view: %v", err)
	}

	return &commonpb.Empty{}, nil
}

// AddInteraction adds or updates a user's interaction on a video
func (h *VideoHandler) AddInteraction(ctx context.Context, req *trainingpb.AddInteractionRequest) (*commonpb.Empty, error) {
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = IPAddressFromGRPCContext(ctx)
	}

	if err := h.service.AddInteraction(ctx, req.VideoId, req.UserId, req.Liked, ipAddress); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add interaction: %v", err)
	}

	return &commonpb.Empty{}, nil
}

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getIPAddress extracts IP address from context metadata
func (h *VideoHandler) getIPAddress(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
			return ips[0]
		}
		if ips := md.Get("x-real-ip"); len(ips) > 0 {
			return ips[0]
		}
	}
	return "unknown"
}
