// Package handler provides gRPC handlers for the buildings service.
package handler

import (
	"context"
	"fmt"
	"strings"

	"metarang/buildings-service/internal/lang"
	"metarang/buildings-service/internal/models"
	pb "metarang/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BuildingHandler struct {
	pb.UnimplementedBuildingServiceServer
	service   BuildingServicePort
	completed CompletedBuildingServicePort
}

func NewBuildingHandler(service BuildingServicePort, completed CompletedBuildingServicePort) *BuildingHandler {
	return &BuildingHandler{
		service:   service,
		completed: completed,
	}
}

func (h *BuildingHandler) GetBuildPackage(ctx context.Context, req *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}

	models, coordinates, err := h.service.GetBuildPackage(ctx, req.FeatureId, req.Page)
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{})
	}

	return &pb.BuildPackageResponse{
		Models:      models,
		Coordinates: coordinates,
	}, nil
}

func (h *BuildingHandler) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}
	if strings.TrimSpace(req.BuildingModelId) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "building_model_id is required"))
	}

	featureResp, err := h.service.BuildFeature(ctx, req)
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{
			mapPrecondition: true,
		})
	}

	return &pb.BuildFeatureResponse{Feature: featureResp}, nil
}

func (h *BuildingHandler) GetBuildings(ctx context.Context, req *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}

	buildings, err := h.service.GetBuildings(ctx, req.FeatureId)
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{})
	}

	return &pb.BuildingsResponse{Buildings: buildings}, nil
}

func (h *BuildingHandler) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}
	if strings.TrimSpace(req.BuildingModelId) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "building_model_id is required"))
	}

	building, err := h.service.UpdateBuilding(ctx, req)
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{
			mapNotFound:     true,
			mapPrecondition: true,
		})
	}

	return &pb.BuildingResponse{
		Success:  true,
		Message:  "Building updated successfully",
		Building: building,
	}, nil
}

func (h *BuildingHandler) UpdateBuildingInformation(ctx context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}
	if strings.TrimSpace(req.BuildingModelId) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "building_model_id is required"))
	}
	if req.Information == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "information is required"))
	}

	information, err := h.service.UpdateBuildingInformation(ctx, req)
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{
			mapNotFound: true,
		})
	}

	return &pb.UpdateBuildingInformationResponse{Information: information}, nil
}

func (h *BuildingHandler) DestroyBuilding(ctx context.Context, req *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error) {
	locale := GetProjectLocale()
	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "feature_id is required"))
	}
	if strings.TrimSpace(req.BuildingModelId) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "building_model_id is required"))
	}

	err := h.service.DestroyBuilding(ctx, req.FeatureId, strings.TrimSpace(req.BuildingModelId))
	if err != nil {
		return nil, mapBuildingServiceError(err, locale, buildingErrorMap{
			mapNotFound: true,
		})
	}

	return &pb.BuildingResponse{
		Success: true,
		Message: "Building destroyed successfully",
	}, nil
}

func (h *BuildingHandler) ListCompletedBuildings(
	ctx context.Context,
	req *pb.ListCompletedBuildingsRequest,
) (*pb.ListCompletedBuildingsResponse, error) {
	if h.completed == nil {
		return nil, status.Errorf(codes.Internal, "completed buildings service unavailable")
	}

	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}

	result, err := h.completed.Paginate(ctx, page)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list completed buildings: %v", err)
	}

	items := make([]*pb.CompletedBuilding, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapCompletedBuilding(item))
	}

	basePath := result.Path
	if basePath == "" {
		basePath = models.CompletedBuildingPath
	}

	links := &pb.PaginationLinks{
		First: fmt.Sprintf("%s?page=1", basePath),
		Last:  fmt.Sprintf("%s?page=%d", basePath, result.LastPage),
	}
	if result.CurrentPage > 1 {
		links.Prev = fmt.Sprintf("%s?page=%d", basePath, result.CurrentPage-1)
	}
	if result.CurrentPage < result.LastPage {
		links.Next = fmt.Sprintf("%s?page=%d", basePath, result.CurrentPage+1)
	}

	meta := &pb.FeatureTradeHistoryPaginationMeta{
		CurrentPage: int32(result.CurrentPage),
		LastPage:    int32(result.LastPage),
		Path:        basePath,
		PerPage:     int32(result.PerPage),
		Total:       int32(result.Total),
	}
	if result.From != nil {
		from := int32(*result.From)
		meta.From = &from
	}
	if result.To != nil {
		to := int32(*result.To)
		meta.To = &to
	}

	return &pb.ListCompletedBuildingsResponse{
		Data:  items,
		Links: links,
		Meta:  meta,
	}, nil
}

func mapCompletedBuilding(item models.CompletedBuilding) *pb.CompletedBuilding {
	out := &pb.CompletedBuilding{
		Id:                  item.ID,
		FeatureId:           item.FeatureID,
		FeaturePropertiesId: item.FeaturePropertiesID,
		Karbari:             item.Karbari,
	}
	if item.Length != nil {
		out.Length = item.Length
	}
	if item.Width != nil {
		out.Width = item.Width
	}
	if item.Density != nil {
		out.Density = item.Density
	}
	return out
}
