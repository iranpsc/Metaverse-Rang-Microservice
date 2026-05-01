package handler

import (
	"context"

	"metargb/features-service/internal/models"
	pb "metargb/shared/pb/features"
)

// FeatureServicePort is implemented by *service.FeatureService.
type FeatureServicePort interface {
	ListFeatures(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error)
	GetFeature(ctx context.Context, featureID uint64) (*pb.Feature, error)
	UpdateFeature(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error)
	AddFeatureImages(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error)
	GetMyFeatures(ctx context.Context, userID uint64) ([]*pb.Feature, error)
	ListMyFeatures(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error)
	GetMyFeature(ctx context.Context, userID, featureID uint64) (*pb.Feature, error)
	AddMyFeatureImages(ctx context.Context, userID, featureID uint64, imageURLs []string) (*pb.Feature, error)
	RemoveMyFeatureImage(ctx context.Context, userID, featureID, imageID uint64) error
	UpdateMyFeature(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error
}

// MarketplaceServicePort is implemented by *service.MarketplaceService.
type MarketplaceServicePort interface {
	BuyFeature(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error)
	SendBuyRequest(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error)
	AcceptBuyRequest(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error)
	CreateSellRequest(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error)
	ListSellRequests(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error)
	DeleteSellRequest(ctx context.Context, sellRequestID, sellerID uint64) error
	RequestGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriod string) error
	ListBuyRequests(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error)
	ListReceivedBuyRequests(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error)
	RejectBuyRequest(ctx context.Context, requestID, sellerID uint64) error
	DeleteBuyRequest(ctx context.Context, requestID, buyerID uint64) error
	UpdateGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriodDays int32) error
	GetUserCode(ctx context.Context, userID uint64) (string, error)
	GetLatestProfilePhoto(ctx context.Context, userID uint64) (string, error)
}

// FeaturePropertyReader loads feature row + properties for response shaping.
type FeaturePropertyReader interface {
	FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error)
}

// GeometryCoordinateReader loads coordinates for a feature.
type GeometryCoordinateReader interface {
	GetCoordinatesWithIDs(ctx context.Context, featureID uint64) ([]*models.Coordinate, error)
}

// BuildingServicePort is implemented by *service.BuildingService.
type BuildingServicePort interface {
	GetBuildPackage(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error)
	BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error)
	GetBuildings(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error)
	DestroyBuilding(ctx context.Context, featureID uint64, buildingModelID uint64) error
}
