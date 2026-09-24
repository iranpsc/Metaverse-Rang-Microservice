package handler

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"metarang/features-service/internal/service"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ListMyFeatures handles GET /api/my-features
// Returns paginated list of authenticated user's features (5 per page)
func (h *FeatureHandler) ListMyFeatures(ctx context.Context, req *pb.ListMyFeaturesRequest) (*pb.ListMyFeaturesResponse, error) {
	// Get authenticated user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	// Use authenticated user ID (ignore req.UserId from path)
	page := req.Page
	if page < 1 {
		page = 1
	}
	search := strings.TrimSpace(req.Search)
	filter := strings.TrimSpace(req.Filter)

	features, err := h.service.ListMyFeatures(ctx, user.UserID, page, search, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list features: %v", err)
	}

	// Build pagination links (simple pagination - no total counts)
	links := &pb.PaginationLinks{
		First: myFeaturesListPath(1, search, filter),
		Last:  "", // Unknown without total
		Prev:  "",
		Next:  "",
	}

	if page > 1 {
		links.Prev = myFeaturesListPath(page-1, search, filter)
	}

	// If we got 5 results, there might be a next page
	if len(features) == 5 {
		links.Next = myFeaturesListPath(page+1, search, filter)
	}

	meta := &pb.SimplePaginationMeta{
		CurrentPage: page,
		Path:        "/api/my-features",
		PerPage:     5,
	}

	return &pb.ListMyFeaturesResponse{
		Data:  features,
		Links: links,
		Meta:  meta,
	}, nil
}

func myFeaturesListPath(page int32, search, filter string) string {
	values := url.Values{}
	values.Set("page", strconv.Itoa(int(page)))
	if search != "" {
		values.Set("search", search)
	}
	if filter != "" {
		values.Set("filter", filter)
	}
	return "/api/my-features?" + values.Encode()
}

// GetMyFeature handles GET /api/my-features/{user}/features/{feature}
// Returns a single feature with all relations (properties, images, latestTraded, geometry)
func (h *FeatureHandler) GetMyFeature(ctx context.Context, req *pb.GetMyFeatureRequest) (*pb.FeatureResponse, error) {
	// Get authenticated user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	// Verify scoped binding: feature must belong to user_id from path
	if req.UserId != user.UserID {
		return nil, status.Errorf(codes.PermissionDenied, "feature does not belong to user")
	}

	feature, err := h.service.GetMyFeature(ctx, req.UserId, req.FeatureId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get feature: %v", err)
	}

	return &pb.FeatureResponse{
		Feature: feature,
	}, nil
}

// AddMyFeatureImages handles POST /api/my-features/{user}/add-image/{feature}
// Uploads images and attaches them to a feature
func (h *FeatureHandler) AddMyFeatureImages(ctx context.Context, req *pb.AddMyFeatureImagesRequest) (*pb.FeatureResponse, error) {
	// Get authenticated user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	// Verify scoped binding
	if req.UserId != user.UserID {
		return nil, status.Errorf(codes.PermissionDenied, "feature does not belong to user")
	}

	if len(req.ImageData) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "at least one image is required")
	}

	for i, imgData := range req.ImageData {
		filename := ""
		contentType := ""
		if i < len(req.Filenames) {
			filename = req.Filenames[i]
		}
		if i < len(req.ContentTypes) {
			contentType = req.ContentTypes[i]
		}
		if err := validateMyFeatureImage(imgData, filename, contentType); err != nil {
			return nil, err
		}
	}

	feature, err := h.service.AddMyFeatureImages(ctx, req.UserId, req.FeatureId, req.ImageData, req.Filenames, req.ContentTypes)
	if err != nil {
		if errors.Is(err, service.ErrInvalidFeatureImage) || errors.Is(err, service.ErrFeatureImageRequired) {
			return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
		}
		if errors.Is(err, service.ErrStorageUnavailable) {
			return nil, status.Errorf(codes.Internal, "storage service not available")
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to add images: %v", err)
	}

	return &pb.FeatureResponse{
		Feature: feature,
	}, nil
}

// RemoveMyFeatureImage handles POST /api/my-features/{user}/remove-image/{feature}/image/{image}
// Removes a single image from a feature
func (h *FeatureHandler) RemoveMyFeatureImage(ctx context.Context, req *pb.RemoveMyFeatureImageRequest) (*emptypb.Empty, error) {
	// Get authenticated user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	// Verify scoped binding
	if req.UserId != user.UserID {
		return nil, status.Errorf(codes.PermissionDenied, "feature does not belong to user")
	}

	err = h.service.RemoveMyFeatureImage(ctx, req.UserId, req.FeatureId, req.ImageId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "image not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to remove image: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateMyFeature handles POST /api/my-features/{user}/features/{feature}
// Updates minimum price percentage and recalculates pricing
func (h *FeatureHandler) UpdateMyFeature(ctx context.Context, req *pb.UpdateMyFeatureRequest) (*pb.UpdateMyFeatureResponse, error) {
	// Get authenticated user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	// Verify scoped binding
	if req.UserId != user.UserID {
		return nil, status.Errorf(codes.PermissionDenied, "feature does not belong to user")
	}

	// Validate minimum_price_percentage
	if req.MinimumPricePercentage < 80 {
		return nil, status.Errorf(codes.InvalidArgument, "minimum_price_percentage must be at least 80")
	}

	result, err := h.service.UpdateMyFeature(ctx, req.UserId, req.FeatureId, req.MinimumPricePercentage)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		// Check for validation errors from pricing service
		if strings.Contains(err.Error(), "حداقل درصد") {
			return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to update feature: %v", err)
	}

	if result == nil {
		result = &pb.UpdateMyFeatureResponse{}
	}
	return result, nil
}

func validateMyFeatureImage(data []byte, filename, contentType string) error {
	if len(data) == 0 {
		return status.Errorf(codes.InvalidArgument, "image data is required")
	}
	if len(data) > 1024*1024 {
		return status.Errorf(codes.InvalidArgument, "image size exceeds 1024 KB limit")
	}

	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/jpg" && contentType != "image/bmp" {
		return status.Errorf(codes.InvalidArgument, "invalid image type: must be PNG, JPG, or BMP")
	}

	if filename != "" {
		lower := strings.ToLower(filename)
		if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") && !strings.HasSuffix(lower, ".bmp") {
			return status.Errorf(codes.InvalidArgument, "invalid image type: must be PNG, JPG, or BMP")
		}
	}
	return nil
}
