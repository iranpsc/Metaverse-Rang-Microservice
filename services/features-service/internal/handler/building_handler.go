package handler

import (
	"context"
	"strings"

	"metargb/features-service/internal/lang"
	"metargb/features-service/internal/service"
	pb "metargb/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BuildingHandler struct {
	pb.UnimplementedBuildingServiceServer
	service *service.BuildingService
}

func NewBuildingHandler(service *service.BuildingService) *BuildingHandler {
	return &BuildingHandler{
		service: service,
	}
}

// GetBuildPackage retrieves available building models for a feature from 3D Meta API
// Implements Laravel's BuildFeatureController@getBuildPackage
func (h *BuildingHandler) GetBuildPackage(ctx context.Context, req *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error) {
	locale := getProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "feature_id is required"))
	}

	models, coordinates, err := h.service.GetBuildPackage(ctx, req.FeatureId, req.Page)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "does not own") {
			return nil, status.Errorf(codes.PermissionDenied, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get build package: %v", err))
	}

	return &pb.BuildPackageResponse{
		Models:      models,
		Coordinates: coordinates,
	}, nil
}

// BuildFeature starts construction of a building on a feature
// Implements Laravel's BuildFeatureController@buildFeature
func (h *BuildingHandler) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error) {
	locale := getProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "feature_id is required"))
	}
	// TODO: Change back to string check (== "") after proto regeneration
	// Currently proto generated code has uint64, but proto file says string
	if req.BuildingModelId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "building_model_id is required"))
	}

	_, err := h.service.BuildFeature(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "does not own") {
			return nil, status.Errorf(codes.PermissionDenied, "%s", err.Error())
		}
		if strings.Contains(err.Error(), "already has") || strings.Contains(err.Error(), "insufficient") {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
		}
		if strings.Contains(err.Error(), "invalid") {
			return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to build feature: %v", err))
	}

	// TODO: Change back to Feature field after proto regeneration
	// Currently proto generated code has Success/Message, but proto file says Feature
	return &pb.BuildFeatureResponse{
		Success: true,
		Message: "Building created successfully",
	}, nil
}

// GetBuildings retrieves all buildings on a feature
// Implements Laravel's BuildFeatureController@getBuildings
func (h *BuildingHandler) GetBuildings(ctx context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
	locale := getProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "feature_id is required"))
	}

	buildings, err := h.service.GetBuildings(ctx, req.FeatureId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get buildings: %v", err))
	}

	return &pb.BuildingsResponse{
		Buildings: buildings,
	}, nil
}

// UpdateBuilding updates an existing building
// Implements Laravel's BuildFeatureController@updateBuilding
func (h *BuildingHandler) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
	locale := getProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "feature_id is required"))
	}
	// TODO: Change back to string check (== "") after proto regeneration
	// Currently proto generated code has uint64, but proto file says string
	if req.BuildingModelId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "building_model_id is required"))
	}

	building, err := h.service.UpdateBuilding(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "does not own") {
			return nil, status.Errorf(codes.PermissionDenied, "%s", err.Error())
		}
		if strings.Contains(err.Error(), "insufficient") {
			return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
		}
		if strings.Contains(err.Error(), "invalid") {
			return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to update building: %v", err))
	}

	return &pb.BuildingResponse{
		Success:  true,
		Message:  "Building updated successfully",
		Building: building,
	}, nil
}

// DestroyBuilding removes a building from a feature
// Implements Laravel's BuildFeatureController@destroyBuilding
func (h *BuildingHandler) DestroyBuilding(ctx context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
	locale := getProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "feature_id is required"))
	}
	if req.BuildingModelId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "building_model_id is required"))
	}

	// Get authenticated user (ownership check should be done in service)
	err := h.service.DestroyBuilding(ctx, req.FeatureId, req.BuildingModelId)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "does not own") {
			return nil, status.Errorf(codes.PermissionDenied, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to destroy building: %v", err))
	}

	return &pb.BuildingResponse{
		Success: true,
		Message: "Building destroyed successfully",
	}, nil
}
