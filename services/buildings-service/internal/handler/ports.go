package handler

import (
	"context"

	"metarang/buildings-service/internal/models"
	pb "metarang/shared/pb/features"
)

// BuildingHTTPAPI is the subset of building gRPC methods exposed over HTTP.
type BuildingHTTPAPI interface {
	GetBuildPackage(context.Context, *pb.GetBuildPackageRequest) (*pb.BuildPackageResponse, error)
	BuildFeature(context.Context, *pb.BuildFeatureRequest) (*pb.BuildFeatureResponse, error)
	GetBuildings(context.Context, *pb.GetBuildingsRequest) (*pb.BuildingsResponse, error)
	UpdateBuilding(context.Context, *pb.UpdateBuildingRequest) (*pb.BuildingResponse, error)
	UpdateBuildingInformation(context.Context, *pb.UpdateBuildingInformationRequest) (*pb.UpdateBuildingInformationResponse, error)
	DestroyBuilding(context.Context, *pb.DestroyBuildingRequest) (*pb.BuildingResponse, error)
	ListCompletedBuildings(context.Context, *pb.ListCompletedBuildingsRequest) (*pb.ListCompletedBuildingsResponse, error)
}

// CitizenBuildingsHTTPAPI is the subset of citizen building gRPC methods exposed over HTTP.
type CitizenBuildingsHTTPAPI interface {
	GetCitizenBuildingSummary(context.Context, *pb.GetCitizenBuildingSummaryRequest) (*pb.GetCitizenBuildingSummaryResponse, error)
	GetCitizenBuildingChart(context.Context, *pb.GetCitizenBuildingChartRequest) (*pb.GetCitizenBuildingChartResponse, error)
	ListCitizenBuildings(context.Context, *pb.ListCitizenBuildingsRequest) (*pb.ListCitizenBuildingsResponse, error)
}

// BuildingServicePort is implemented by *service.BuildingService.
type BuildingServicePort interface {
	GetBuildPackage(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error)
	BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error)
	GetBuildings(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error)
	UpdateBuildingInformation(ctx context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.BuildingInformation, error)
	DestroyBuilding(ctx context.Context, featureID uint64, buildingModelID string) error
}

// CompletedBuildingServicePort is implemented by *service.CompletedBuildingService.
type CompletedBuildingServicePort interface {
	Paginate(ctx context.Context, page int) (*models.CompletedBuildingPage, error)
}

// CitizenBuildingsServicePort is implemented by *service.CitizenBuildingsService.
type CitizenBuildingsServicePort interface {
	GetSummary(ctx context.Context, userID uint64, allowedKarbaris []string) (*models.CitizenBuildingSummaryResult, error)
	GetChart(ctx context.Context, userID uint64, period string, allowedKarbaris []string) (*models.CitizenBuildingChartResult, error)
	GetBuildings(ctx context.Context, userID uint64, allowedKarbaris []string, page int) (*models.CitizenBuildingsPage, error)
}
