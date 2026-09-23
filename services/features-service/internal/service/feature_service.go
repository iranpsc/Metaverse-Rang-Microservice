package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/repository"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/helpers"
)

type FeatureService struct {
	featureRepo      *repository.FeatureRepository
	propertiesRepo   *repository.PropertiesRepository
	geometryRepo     *repository.GeometryRepository
	imageRepo        *repository.ImageRepository
	buildingRepo     *repository.BuildingRepository
	tradeRepo        *repository.TradeRepository
	hourlyProfitRepo *repository.HourlyProfitRepository
	sellRequestRepo  *repository.SellRequestRepository
	pricingService   *FeaturePricingService
	fileStorage      FileStorage
	apiGatewayURL    string
	db               *sql.DB
}

func NewFeatureService(
	featureRepo *repository.FeatureRepository,
	propertiesRepo *repository.PropertiesRepository,
	geometryRepo *repository.GeometryRepository,
	imageRepo *repository.ImageRepository,
	buildingRepo *repository.BuildingRepository,
	tradeRepo *repository.TradeRepository,
	hourlyProfitRepo *repository.HourlyProfitRepository,
	pricingService *FeaturePricingService,
	db *sql.DB,
	fileStorage FileStorage,
	apiGatewayURL string,
) *FeatureService {
	return &FeatureService{
		featureRepo:      featureRepo,
		propertiesRepo:   propertiesRepo,
		geometryRepo:     geometryRepo,
		imageRepo:        imageRepo,
		buildingRepo:     buildingRepo,
		tradeRepo:        tradeRepo,
		hourlyProfitRepo: hourlyProfitRepo,
		sellRequestRepo:  repository.NewSellRequestRepository(db),
		pricingService:   pricingService,
		fileStorage:      fileStorage,
		apiGatewayURL:    apiGatewayURL,
		db:               db,
	}
}

// ListFeatures retrieves features within a bounding box
// Supports optional authentication (is_owned_by_auth_user) and building models
func (s *FeatureService) ListFeatures(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error) {
	// Validate points array (min:4, regex validation per documentation)
	if len(points) < 4 {
		return nil, fmt.Errorf("points array must have at least 4 elements")
	}

	// Parse points into coordinates
	// points[0] = "x1,y1", points[1] = "x2,y2", etc.
	// Expected format: [topLeft, topRight, bottomLeft, bottomRight]

	features, propertiesList, err := s.featureRepo.FindByBoundingBoxWithProperties(ctx, points)
	if err != nil {
		return nil, fmt.Errorf("failed to find features by bbox: %w", err)
	}

	featureIDs := make([]uint64, len(features))
	for i, feature := range features {
		featureIDs[i] = feature.ID
	}

	geometriesByFeature, err := s.geometryRepo.GetByFeatureIDs(ctx, featureIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load geometries: %w", err)
	}

	coordinatesByFeature, err := s.geometryRepo.GetCoordinatesByFeatureIDs(ctx, featureIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load coordinates: %w", err)
	}

	var buildingsByFeature map[uint64][]*pb.Building
	if loadBuildings {
		buildingsByFeature, err = s.buildingRepo.FindByFeatureIDs(ctx, featureIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to load buildings: %w", err)
		}
	}

	// Convert to protobuf with all relations
	result := make([]*pb.Feature, 0, len(features))
	for i, feature := range features {
		properties := propertiesList[i]

		geometry := geometriesByFeature[feature.ID]
		var pbGeometry *pb.Geometry
		if geometry != nil {
			coordinates := coordinatesByFeature[feature.ID]
			if len(coordinates) > 0 {
				pbCoordinates := make([]*pb.Coordinate, 0, len(coordinates))
				for _, coord := range coordinates {
					pbCoordinates = append(pbCoordinates, &pb.Coordinate{
						Id:         coord.ID,
						GeometryId: coord.GeometryID,
						X:          formatCoordValue(coord.X),
						Y:          formatCoordValue(coord.Y),
					})
				}
				pbGeometry = &pb.Geometry{
					Id:          geometry.ID,
					Type:        geometry.Type,
					Coordinates: pbCoordinates,
				}
			} else {
				pbGeometry = &pb.Geometry{
					Id:   geometry.ID,
					Type: geometry.Type,
				}
			}
		}

		var buildings []*pb.Building
		if loadBuildings {
			buildings = buildingsByFeature[feature.ID]
		}

		// Check if owned by authenticated user
		isOwned := false
		if authUserID > 0 {
			isOwned = feature.OwnerID == authUserID
		}

		pbFeature := &pb.Feature{
			Id:                feature.ID,
			OwnerId:           feature.OwnerID,
			Properties:        models.PropertiesToPB(properties),
			Geometry:          pbGeometry,
			IsOwnedByAuthUser: isOwned,
			BuildingModels:    buildings,
		}

		result = append(result, pbFeature)
	}

	return result, nil
}

// isForSaleFromLatestSellRequest maps the latest sell-request status to the
// my-features is_for_sale flag: open (0) => 1, completed (1) or missing => 0.
func isForSaleFromLatestSellRequest(req *models.SellFeatureRequest) int32 {
	if req != nil && req.Status == 0 {
		return 1
	}
	return 0
}

func (s *FeatureService) fetchLatestSellRequest(ctx context.Context, featureID uint64) *models.SellFeatureRequest {
	if s.sellRequestRepo == nil {
		return nil
	}
	latest, err := s.sellRequestRepo.GetLatestByFeatureID(ctx, featureID)
	if err != nil {
		return nil
	}
	return latest
}

func sellFeatureRequestToPB(req *models.SellFeatureRequest) *pb.SellRequestResponse {
	if req == nil {
		return nil
	}
	return &pb.SellRequestResponse{
		Id:        req.ID,
		SellerId:  req.SellerID,
		FeatureId: req.FeatureID,
		PricePsc:  strconv.FormatFloat(req.PricePSC, 'f', -1, 64),
		PriceIrr:  strconv.FormatFloat(req.PriceIRR, 'f', -1, 64),
		Status:    int32(req.Status),
		CreatedAt: helpers.FormatJalaliDate(req.CreatedAt),
	}
}

// GetFeature retrieves a single feature with all relations
// Loads: properties, images, latestTraded.seller, hourlyProfit, buildingModels
func (s *FeatureService) GetFeature(ctx context.Context, featureID uint64) (*pb.Feature, error) {
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Load geometry with coordinates
	geometry, _ := s.geometryRepo.GetByFeatureID(ctx, featureID)
	var pbGeometry *pb.Geometry
	if geometry != nil {
		coordinates, err := s.geometryRepo.GetCoordinatesByFeatureID(ctx, featureID)
		if err == nil {
			pbCoordinates := make([]*pb.Coordinate, 0, len(coordinates))
			for _, coordStr := range coordinates {
				parts := strings.Split(coordStr, ",")
				if len(parts) == 2 {
					pbCoordinates = append(pbCoordinates, &pb.Coordinate{
						X: parts[0],
						Y: parts[1],
					})
				}
			}
			pbGeometry = &pb.Geometry{
				Id:          geometry.ID,
				Type:        geometry.Type,
				Coordinates: pbCoordinates,
			}
		} else {
			pbGeometry = &pb.Geometry{
				Id:   geometry.ID,
				Type: geometry.Type,
			}
		}
	}

	// Load images
	images, err := s.imageRepo.GetImagesByFeatureID(ctx, featureID)
	if err != nil {
		images = nil
	}
	pbImages := make([]*pb.Image, 0, len(images))
	for _, img := range images {
		pbImages = append(pbImages, &pb.Image{
			Id:  img.ID,
			Url: img.URL,
		})
	}

	// Load latest trade with seller
	_, seller, _ := s.tradeRepo.GetLatestForFeatureWithSeller(ctx, featureID)
	var pbSeller *pb.Seller
	if seller != nil && seller.ID > 0 {
		pbSeller = &pb.Seller{
			Id:   seller.ID,
			Name: seller.Name,
			Code: seller.Code,
		}
	}

	// Load hourly profit status
	hourlyProfit, err := s.hourlyProfitRepo.GetByFeatureAndUser(ctx, featureID, feature.OwnerID)
	isHourlyProfitActive := false
	if err == nil && hourlyProfit != nil {
		isHourlyProfitActive = hourlyProfit.IsActive
	}

	// Load building models
	buildings, err := s.buildingRepo.FindByFeatureID(ctx, featureID)
	if err != nil {
		buildings = nil
	}

	// Build complete feature response
	pbFeature := &pb.Feature{
		Id:                   feature.ID,
		OwnerId:              feature.OwnerID,
		Properties:           models.PropertiesToPB(properties),
		Geometry:             pbGeometry,
		Images:               pbImages,
		Seller:               pbSeller,
		IsHourlyProfitActive: isHourlyProfitActive,
		BuildingModels:       buildings,
	}
	s.applyLatestSellRequest(ctx, pbFeature)

	return pbFeature, nil
}

// UpdateFeature updates feature properties
func (s *FeatureService) UpdateFeature(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error) {
	// Convert protobuf properties to map for update
	updates := map[string]interface{}{
		"karbari": properties.Karbari,
		"rgb":     properties.Rgb,
		"owner":   properties.Owner,
		"label":   properties.Label,
	}

	if properties.PricePsc != "" {
		updates["price_psc"] = properties.PricePsc
	}
	if properties.PriceIrr != "" {
		updates["price_irr"] = properties.PriceIrr
	}
	if properties.MinimumPricePercentage > 0 {
		updates["minimum_price_percentage"] = properties.MinimumPricePercentage
	}

	// Update properties
	if err := s.propertiesRepo.Update(ctx, featureID, updates); err != nil {
		return nil, fmt.Errorf("failed to update properties: %w", err)
	}

	// Return updated feature
	return s.GetFeature(ctx, featureID)
}

// AddFeatureImages adds images to a feature
func (s *FeatureService) AddFeatureImages(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error) {
	// TODO: Implement image addition
	// For now, just return the feature
	return s.GetFeature(ctx, featureID)
}

// GetMyFeatures retrieves all features owned by a user
func (s *FeatureService) GetMyFeatures(ctx context.Context, userID uint64) ([]*pb.Feature, error) {
	features, err := s.featureRepo.FindByOwner(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user features: %w", err)
	}

	return models.FeaturesToPB(features), nil
}

// ListMyFeatures retrieves paginated features owned by authenticated user (5 per page)
// Only loads properties (images are empty on this endpoint), is-for-sale, and latest-sell-request
// search matches feature_properties.id or address; filter matches karbari
func (s *FeatureService) ListMyFeatures(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
	if page < 1 {
		page = 1
	}

	features, propertiesList, err := s.featureRepo.FindByOwnerPaginated(ctx, userID, int(page), search, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find user features: %w", err)
	}

	// Convert to protobuf (only properties loaded, images empty)
	result := make([]*pb.Feature, 0, len(features))
	for i, feature := range features {
		properties := propertiesList[i]
		latestSell := s.fetchLatestSellRequest(ctx, feature.ID)
		pbFeature := &pb.Feature{
			Id:                feature.ID,
			OwnerId:           feature.OwnerID,
			Properties:        models.PropertiesToPB(properties),
			Images:            []*pb.Image{}, // Always empty on list endpoint
			IsForSale:         isForSaleFromLatestSellRequest(latestSell),
			LatestSellRequest: sellFeatureRequestToPB(latestSell),
		}
		result = append(result, pbFeature)
	}
	s.applyLatestSellRequests(ctx, result)

	return result, nil
}

// GetMyFeature retrieves a single feature with all relations (properties, images, latestTraded, geometry)
// Verifies that the feature belongs to the user (scoped binding)
func (s *FeatureService) GetMyFeature(ctx context.Context, userID, featureID uint64) (*pb.Feature, error) {
	// Verify ownership via scoped binding
	feature, properties, err := s.featureRepo.FindByOwnerAndFeatureID(ctx, userID, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to find feature: %w", err)
	}
	if feature == nil {
		return nil, fmt.Errorf("feature not found or does not belong to user")
	}

	// Load geometry with coordinates
	geometry, _ := s.geometryRepo.GetByFeatureID(ctx, featureID)
	var pbGeometry *pb.Geometry
	if geometry != nil {
		coordinates, err := s.geometryRepo.GetCoordinatesByFeatureID(ctx, featureID)
		if err == nil {
			pbCoordinates := make([]*pb.Coordinate, 0, len(coordinates))
			for _, coordStr := range coordinates {
				parts := strings.Split(coordStr, ",")
				if len(parts) == 2 {
					pbCoordinates = append(pbCoordinates, &pb.Coordinate{
						X: parts[0],
						Y: parts[1],
					})
				}
			}
			pbGeometry = &pb.Geometry{
				Id:          geometry.ID,
				Type:        geometry.Type,
				Coordinates: pbCoordinates,
			}
		} else {
			pbGeometry = &pb.Geometry{
				Id:   geometry.ID,
				Type: geometry.Type,
			}
		}
	}

	// Load images
	images, err := s.imageRepo.GetImagesByFeatureID(ctx, featureID)
	if err != nil {
		images = nil
	}
	pbImages := make([]*pb.Image, 0, len(images))
	for _, img := range images {
		pbImages = append(pbImages, &pb.Image{
			Id:  img.ID,
			Url: img.URL,
		})
	}

	// Load latest trade with seller
	_, seller, _ := s.tradeRepo.GetLatestForFeatureWithSeller(ctx, featureID)
	var pbSeller *pb.Seller
	if seller != nil && seller.ID > 0 {
		pbSeller = &pb.Seller{
			Id:   seller.ID,
			Name: seller.Name,
			Code: seller.Code,
		}
	}

	// Build complete feature response
	pbFeature := &pb.Feature{
		Id:         feature.ID,
		OwnerId:    feature.OwnerID,
		Properties: models.PropertiesToPB(properties),
		Geometry:   pbGeometry,
		Images:     pbImages,
		Seller:     pbSeller,
	}
	s.applyLatestSellRequest(ctx, pbFeature)

	return pbFeature, nil
}

// AddMyFeatureImages uploads images to storage-service and attaches them to a feature owned by the user.
func (s *FeatureService) AddMyFeatureImages(ctx context.Context, userID, featureID uint64, imageData [][]byte, filenames, contentTypes []string) (*pb.Feature, error) {
	feature, _, err := s.featureRepo.FindByOwnerAndFeatureID(ctx, userID, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to find feature: %w", err)
	}
	if feature == nil {
		return nil, fmt.Errorf("feature not found or does not belong to user")
	}

	imageURLs, err := s.uploadFeatureImages(ctx, featureID, imageData, filenames, contentTypes)
	if err != nil {
		return nil, err
	}

	for _, url := range imageURLs {
		if _, err := s.imageRepo.CreateImage(ctx, featureID, url); err != nil {
			return nil, fmt.Errorf("failed to create image: %w", err)
		}
	}

	return s.GetMyFeature(ctx, userID, featureID)
}

func (s *FeatureService) uploadFeatureImages(ctx context.Context, featureID uint64, imageData [][]byte, filenames, contentTypes []string) ([]string, error) {
	if s.fileStorage == nil {
		return nil, ErrStorageUnavailable
	}

	imageURLs := make([]string, 0, len(imageData))
	for i, data := range imageData {
		filename := ""
		contentType := ""
		if i < len(filenames) {
			filename = filenames[i]
		}
		if i < len(contentTypes) {
			contentType = contentTypes[i]
		}
		if err := validateFeatureImage(data, filename, contentType); err != nil {
			return nil, err
		}
		if filename == "" {
			filename = defaultFeatureImageFilename(contentType, i)
		}

		relativePath, err := s.fileStorage.UploadChunk(
			ctx,
			newFeatureImageUploadID(featureID, i),
			featureImageUploadPath(featureID),
			filename,
			contentType,
			data,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to upload image: %w", err)
		}
		imageURLs = append(imageURLs, prependPublicURL(s.apiGatewayURL, relativePath))
	}
	return imageURLs, nil
}

// RemoveMyFeatureImage removes an image from a feature
// Verifies that both feature and image belong to the user
func (s *FeatureService) RemoveMyFeatureImage(ctx context.Context, userID, featureID, imageID uint64) error {
	// Verify ownership
	feature, _, err := s.featureRepo.FindByOwnerAndFeatureID(ctx, userID, featureID)
	if err != nil {
		return fmt.Errorf("failed to find feature: %w", err)
	}
	if feature == nil {
		return fmt.Errorf("feature not found or does not belong to user")
	}

	// Verify image belongs to feature and delete
	err = s.imageRepo.DeleteImage(ctx, featureID, imageID)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	return nil
}

// UpdateMyFeature updates the minimum price percentage for a feature
// Verifies ownership and calculates new pricing based on stability and rates
func (s *FeatureService) UpdateMyFeature(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
	// Verify ownership
	feature, _, err := s.featureRepo.FindByOwnerAndFeatureID(ctx, userID, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to find feature: %w", err)
	}
	if feature == nil {
		return nil, fmt.Errorf("feature not found or does not belong to user")
	}

	// Use pricing service to update (handles validation and calculation)
	if s.pricingService == nil {
		return nil, fmt.Errorf("pricing service not initialized")
	}

	pricing, err := s.pricingService.UpdateFeaturePricing(ctx, featureID, userID, int(minimumPricePercentage))
	if err != nil {
		return nil, fmt.Errorf("failed to update feature pricing: %w", err)
	}
	if pricing == nil {
		return &pb.UpdateMyFeatureResponse{}, nil
	}

	return &pb.UpdateMyFeatureResponse{
		PricePsc: pricing.PricePSC,
		PriceIrr: pricing.PriceIRR,
	}, nil
}

func (s *FeatureService) applyLatestSellRequest(ctx context.Context, feature *pb.Feature) {
	if feature == nil || s.db == nil {
		return
	}
	req, err := repository.NewSellRequestRepository(s.db).GetLatestOpenByFeatureID(ctx, feature.Id)
	if err != nil || req == nil {
		return
	}
	feature.IsForSale = isForSaleFromLatestSellRequest(req)
	feature.LatestSellRequest = sellRequestToPB(req)
}

func (s *FeatureService) applyLatestSellRequests(ctx context.Context, features []*pb.Feature) {
	if s.db == nil || len(features) == 0 {
		return
	}
	ids := make([]uint64, 0, len(features))
	for _, feature := range features {
		if feature != nil {
			ids = append(ids, feature.Id)
		}
	}
	latest, err := repository.NewSellRequestRepository(s.db).GetLatestOpenByFeatureIDs(ctx, ids)
	if err != nil {
		return
	}
	for _, feature := range features {
		if feature == nil {
			continue
		}
		req := latest[feature.Id]
		if req == nil {
			continue
		}
		feature.IsForSale = isForSaleFromLatestSellRequest(req)
		feature.LatestSellRequest = sellRequestToPB(req)
	}
}

func sellRequestToPB(req *models.SellFeatureRequest) *pb.SellRequestResponse {
	if req == nil {
		return nil
	}
	return &pb.SellRequestResponse{
		Id:        req.ID,
		SellerId:  req.SellerID,
		FeatureId: req.FeatureID,
		PricePsc:  fmt.Sprintf("%.10f", req.PricePSC),
		PriceIrr:  fmt.Sprintf("%.10f", req.PriceIRR),
		Status:    int32(req.Status),
		CreatedAt: helpers.FormatJalaliDate(req.CreatedAt),
	}
}

func formatCoordValue(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
