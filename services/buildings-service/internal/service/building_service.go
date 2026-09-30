package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/pkg/threed_client"
	commercialpb "metarang/shared/pb/commercial"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"
	"metarang/shared/pkg/helpers"
)

// buildingRepository defines persistence used by BuildingService.
type buildingRepository interface {
	UpsertBuildingModel(ctx context.Context, modelID uint64, name, sku, images, attributes, file string, requiredSatisfaction float64) error
	FindBuildingModelByModelID(ctx context.Context, modelID string) (*pb.BuildingModel, error)
	HasBuilding(ctx context.Context, featureID uint64) (bool, error)
	CreateBuilding(ctx context.Context, featureID, userID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, startDate, endDate time.Time, bubbleDiameter float64) error
	FindByFeatureID(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	UpdateBuilding(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, endDate time.Time, bubbleDiameter float64) (*pb.Building, error)
	UpdateBuildingInformation(ctx context.Context, featureID uint64, buildingModelID string, information string) error
	FindBuildingByFeatureAndModel(ctx context.Context, featureID uint64, buildingModelID string) (*pb.Building, error)
	DeleteBuilding(ctx context.Context, featureID uint64, buildingModelID string) (string, error)
	FirstOrCreateIsicCode(ctx context.Context, activityLine string) (uint64, error)
}

type buildingFeatureRepository interface {
	FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error)
}

type buildingGeometryRepository interface {
	GetCoordinatesByFeatureID(ctx context.Context, featureID uint64) ([]string, error)
}

type buildingHourlyProfitRepository interface {
	DeactivateProfitsForFeature(ctx context.Context, featureID uint64) error
	ActivateProfitsForFeature(ctx context.Context, featureID uint64) error
}

type buildingThreeDClient interface {
	GetBuildPackage(ctx context.Context, req threed_client.BuildPackageRequest) (*threed_client.BuildPackageResponse, error)
}

type buildingCommercialClient interface {
	GetWallet(ctx context.Context, userID uint64) (*commercialpb.WalletResponse, error)
	DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) error
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
}

type BuildingService struct {
	buildingRepo     buildingRepository
	featureRepo      buildingFeatureRepository
	geometryRepo     buildingGeometryRepository
	hourlyProfitRepo buildingHourlyProfitRepository
	threeDClient     buildingThreeDClient
	commercialClient buildingCommercialClient
}

func NewBuildingService(
	buildingRepo buildingRepository,
	featureRepo buildingFeatureRepository,
	geometryRepo buildingGeometryRepository,
	hourlyProfitRepo buildingHourlyProfitRepository,
	threeDClient buildingThreeDClient,
) *BuildingService {
	return &BuildingService{
		buildingRepo:     buildingRepo,
		featureRepo:      featureRepo,
		geometryRepo:     geometryRepo,
		hourlyProfitRepo: hourlyProfitRepo,
		threeDClient:     threeDClient,
	}
}

// SetCommercialClient sets the commercial client for wallet operations
func (s *BuildingService) SetCommercialClient(c buildingCommercialClient) {
	s.commercialClient = c
}

// GetBuildPackage retrieves building models from 3D Meta API
// Checks ownership, calls 3D API, calculates required_satisfaction, upserts models, and returns with coordinates
func (s *BuildingService) GetBuildPackage(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error) {
	// Get feature with properties
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, nil, fmt.Errorf("feature not found: %w", err)
	}

	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unauthorized: authentication required")
	}
	if feature.OwnerID != user.UserID {
		return nil, nil, fmt.Errorf("unauthorized: user does not own this feature")
	}

	// Get coordinates for feature
	coordinates, err := s.geometryRepo.GetCoordinatesByFeatureID(ctx, featureID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coordinates: %w", err)
	}

	requiredSatisfaction, err := requiredSatisfactionForFeature(properties)
	if err != nil {
		return nil, nil, err
	}
	density := properties.Density
	if density == 0 {
		density = 1
	}

	apiResp, err := s.threeDClient.GetBuildPackage(ctx, threed_client.BuildPackageRequest{
		FeatureID: featureID,
		Area:      fmt.Sprintf("%.2f", properties.Area),
		Density:   fmt.Sprintf("%d", density),
		Karbari:   properties.Karbari,
		Page:      page,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("3D API call failed: %w", err)
	}

	// Persist catalog fields only. required_satisfaction is feature-specific and
	// is returned in the response, not written onto the shared model row.
	models := make([]*pb.BuildingModel, 0, len(apiResp.Data))
	for _, item := range apiResp.Data {
		imagesJSON, _ := json.Marshal(item.Images)
		attrsJSON, _ := json.Marshal(item.Attributes)
		fileJSON, _ := json.Marshal(item.File)

		if err := s.buildingRepo.UpsertBuildingModel(ctx, item.ID, item.Name, item.SKU,
			string(imagesJSON), string(attrsJSON), string(fileJSON), 0); err != nil {
			return nil, nil, fmt.Errorf("failed to upsert building model: %w", err)
		}

		models = append(models, &pb.BuildingModel{
			Id:                   item.ID,
			ModelId:              fmt.Sprintf("%d", item.ID),
			Name:                 item.Name,
			Sku:                  item.SKU,
			Images:               string(imagesJSON),
			Attributes:           string(attrsJSON),
			File:                 string(fileJSON),
			RequiredSatisfaction: fmt.Sprintf("%.4f", requiredSatisfaction),
		})
	}

	return models, coordinates, nil
}

// BuildFeature starts construction of a building on a feature
func (s *BuildingService) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error) {
	feature, properties, user, err := s.ownedFeature(ctx, req.FeatureId)
	if err != nil {
		return nil, err
	}

	hasBuilding, err := s.buildingRepo.HasBuilding(ctx, req.FeatureId)
	if err != nil {
		return nil, fmt.Errorf("failed to check building existence: %w", err)
	}
	if hasBuilding {
		return nil, fmt.Errorf("feature already has a building")
	}

	buildingModel, err := s.loadBuildingModel(ctx, req.BuildingModelId)
	if err != nil {
		return nil, err
	}

	requiredSatisfaction, err := requiredSatisfactionForFeature(properties)
	if err != nil {
		return nil, err
	}
	launchedSatisfaction, err := validateLaunchedAmount(req.LaunchedSatisfaction, requiredSatisfaction)
	if err != nil {
		return nil, err
	}
	if err := validateBuildPlacement(req.Rotation, req.Position); err != nil {
		return nil, err
	}
	informationJSON, err := s.prepareInformationJSON(ctx, req.Information)
	if err != nil {
		return nil, err
	}

	if err := s.chargeSatisfaction(ctx, user.UserID, launchedSatisfaction); err != nil {
		return nil, err
	}

	constructionStartDate := time.Now()
	constructionEndDate := constructionEndFromStart(constructionStartDate, requiredSatisfaction, launchedSatisfaction)

	if err := s.hourlyProfitRepo.DeactivateProfitsForFeature(ctx, req.FeatureId); err != nil {
		if refundErr := s.refundSatisfaction(ctx, user.UserID, launchedSatisfaction); refundErr != nil {
			return nil, fmt.Errorf("failed to deactivate profits: %w; %v", err, refundErr)
		}
		return nil, fmt.Errorf("failed to deactivate profits: %w", err)
	}

	bubbleDiameter := s.CalculateBubbleDiameter(buildingModel.Attributes)
	buildingModelIDStr := strings.TrimSpace(req.BuildingModelId)
	err = s.buildingRepo.CreateBuilding(ctx, req.FeatureId, user.UserID, buildingModelIDStr,
		strconv.FormatFloat(launchedSatisfaction, 'f', -1, 64), req.Rotation, req.Position, informationJSON,
		constructionStartDate, constructionEndDate, bubbleDiameter)
	if err != nil {
		activateErr := s.hourlyProfitRepo.ActivateProfitsForFeature(ctx, req.FeatureId)
		refundErr := s.refundSatisfaction(ctx, user.UserID, launchedSatisfaction)
		if activateErr != nil || refundErr != nil {
			return nil, fmt.Errorf("failed to create building: %w; reactivate: %v; refund: %v", err, activateErr, refundErr)
		}
		return nil, fmt.Errorf("failed to create building: %w", err)
	}

	buildings, err := s.buildingRepo.FindByFeatureID(ctx, req.FeatureId)
	if err != nil {
		// Log error but return feature anyway
		buildings = nil
	}

	// Build minimal Feature response with building models
	// Note: We don't have all FeatureService dependencies, so we return minimal Feature
	// with just the essential fields and building models
	return &pb.Feature{
		Id:             feature.ID,
		OwnerId:        feature.OwnerID,
		BuildingModels: buildings,
	}, nil
}

// ExtractAttributeValue extracts a numeric value by slug from attributes array.
// Attributes format: [{"slug": "width", "value": 50}, ...]
func ExtractAttributeValue(attributes []map[string]interface{}, slug string) (float64, bool) {
	for _, attr := range attributes {
		s, ok := attr["slug"].(string)
		if !ok || s != slug {
			continue
		}
		return coerceAttributeNumber(attr["value"])
	}
	return 0, false
}

func coerceAttributeNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// CalculateBubbleDiameter calculates bubble diameter from model attributes
// Expects attributes JSON string with array format: [{"slug": "width", "value": 50}, ...]
// Formula: perimeter × coefficient where:
//   - perimeter = 2 × (width + length)
//   - coefficient = 1 + (0.3 × (density - 1))
func (s *BuildingService) CalculateBubbleDiameter(attributesJSON string) float64 {
	var attributes []map[string]interface{}
	if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
		return 0.0
	}

	width, widthOk := ExtractAttributeValue(attributes, "width")
	length, lengthOk := ExtractAttributeValue(attributes, "length")
	density, densityOk := ExtractAttributeValue(attributes, "density")

	if !widthOk || !lengthOk || !densityOk {
		return 0.0
	}

	// Calculate perimeter: 2 × (width + length)
	perimeter := 2.0 * (width + length)

	// Calculate coefficient: starts at 1, adds 0.3 for each density level above 1
	coefficient := 1 + (0.3 * (density - 1))

	// Final diameter: perimeter × coefficient
	return perimeter * coefficient
}

// ValidateBuildingInformation validates building information fields.
// Rules:
// - activity_line: nullable, max 255
// - name: nullable, max 255 (only saved if activity_line provided)
// - address: nullable, max 255
// - postal_code: nullable, iranian_postal_code (10 digits)
// - website: nullable, active_url, max 255 (DNS check)
// - description: nullable, max 5000
func (s *BuildingService) ValidateBuildingInformation(info *pb.BuildingInformation) error {
	if info == nil {
		return nil // Nullable, so nil is valid
	}

	// activity_line: nullable, max 255
	if info.ActivityLine != "" && len(info.ActivityLine) > 255 {
		return fmt.Errorf("activity_line must not exceed 255 characters")
	}

	// name: nullable, max 255 (only validated if provided)
	if info.Name != "" && len(info.Name) > 255 {
		return fmt.Errorf("name must not exceed 255 characters")
	}

	// address: nullable, max 255
	if info.Address != "" && len(info.Address) > 255 {
		return fmt.Errorf("address must not exceed 255 characters")
	}

	// postal_code: nullable, iranian_postal_code (10 digits)
	if info.PostalCode != "" {
		// Normalize Persian numbers and remove dashes/spaces
		postalCode := helpers.NormalizePersianNumbers(info.PostalCode)
		postalCode = strings.ReplaceAll(postalCode, "-", "")
		postalCode = strings.ReplaceAll(postalCode, " ", "")

		if !postalCodeRegex.MatchString(postalCode) {
			return fmt.Errorf("postal_code must be a valid Iranian postal code (10 digits)")
		}
	}

	// website: nullable, active_url, max 255 (DNS check)
	if info.Website != "" {
		if len(info.Website) > 255 {
			return fmt.Errorf("website must not exceed 255 characters")
		}

		// Validate URL format
		parsedURL, err := url.Parse(info.Website)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("website must be a valid URL")
		}

		// Check if scheme is http or https
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("website must use http or https protocol")
		}

		// Note: DNS check (active_url) would require network call, which we skip in service layer
		// The gateway or handler layer can perform DNS check if needed
	}

	// description: nullable, max 5000
	if info.Description != "" && len(info.Description) > 5000 {
		return fmt.Errorf("description must not exceed 5000 characters")
	}

	return nil
}

// GetBuildings retrieves all buildings on a feature with Jalali formatted dates.
// Address and other information JSON is included only for the feature owner.
func (s *BuildingService) GetBuildings(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
	buildings, err := s.buildingRepo.FindByFeatureID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildings: %w", err)
	}

	includeInformation := false
	if user, err := auth.GetUserFromContext(ctx); err == nil {
		feature, _, ferr := s.featureRepo.FindByID(ctx, featureID)
		if ferr == nil && feature != nil && feature.OwnerID == user.UserID {
			includeInformation = true
		}
	}

	for _, building := range buildings {
		if !includeInformation {
			building.Information = ""
		}
		formatBuildingDates(building)
	}

	return buildings, nil
}

// UpdateBuilding updates an existing building
func (s *BuildingService) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error) {
	_, properties, user, err := s.ownedFeature(ctx, req.FeatureId)
	if err != nil {
		return nil, err
	}

	buildingModelIDStr := strings.TrimSpace(req.BuildingModelId)
	if _, err := s.loadBuildingModel(ctx, buildingModelIDStr); err != nil {
		return nil, err
	}

	requiredSatisfaction, err := requiredSatisfactionForFeature(properties)
	if err != nil {
		return nil, err
	}
	launchedSatisfaction, err := validateLaunchedAmount(req.LaunchedSatisfaction, requiredSatisfaction)
	if err != nil {
		return nil, err
	}
	if err := validateBuildPlacement(req.Rotation, req.Position); err != nil {
		return nil, err
	}

	existingBuilding, err := s.buildingRepo.FindBuildingByFeatureAndModel(ctx, req.FeatureId, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find existing building: %w", err)
	}
	if existingBuilding == nil {
		return nil, fmt.Errorf("building not found")
	}
	previousLaunched, err := strconv.ParseFloat(strings.TrimSpace(existingBuilding.LaunchedSatisfaction), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid launched_satisfaction on building: %w", err)
	}

	informationJSON := existingBuilding.Information
	if req.Information != nil && strings.TrimSpace(req.Information.ActivityLine) != "" {
		informationJSON, err = s.prepareInformationJSON(ctx, req.Information)
		if err != nil {
			return nil, err
		}
	}

	delta := launchedSatisfaction - previousLaunched
	// Charge an increase before the stored stake changes, so a destroy cannot
	// refund satisfaction that was never taken. Refund a decrease only after
	// the stored stake is lowered.
	if delta > 0 {
		if err := s.chargeSatisfaction(ctx, user.UserID, delta); err != nil {
			return nil, err
		}
	}

	constructionStartDate := parseExistingConstructionStart(existingBuilding)
	constructionEndDate := constructionEndFromStart(constructionStartDate, requiredSatisfaction, launchedSatisfaction)
	existingBubbleDiameter, _ := strconv.ParseFloat(existingBuilding.BubbleDiameter, 64)

	updatedBuilding, err := s.buildingRepo.UpdateBuilding(ctx, req.FeatureId, buildingModelIDStr,
		strconv.FormatFloat(launchedSatisfaction, 'f', -1, 64), req.Rotation, req.Position, informationJSON,
		constructionEndDate, existingBubbleDiameter)
	if err != nil {
		if delta > 0 {
			if refundErr := s.refundSatisfaction(ctx, user.UserID, delta); refundErr != nil {
				return nil, fmt.Errorf("failed to update building: %w; %v", err, refundErr)
			}
		}
		return nil, fmt.Errorf("failed to update building: %w", err)
	}
	if delta < 0 {
		if err := s.refundSatisfaction(ctx, user.UserID, -delta); err != nil {
			return nil, err
		}
	}

	formatBuildingDates(updatedBuilding)
	return updatedBuilding, nil
}

// UpdateBuildingInformation updates only the building information JSON.
func (s *BuildingService) UpdateBuildingInformation(ctx context.Context, req *pb.UpdateBuildingInformationRequest) (*pb.BuildingInformation, error) {
	_, _, _, err := s.ownedFeature(ctx, req.FeatureId)
	if err != nil {
		return nil, err
	}

	buildingModelIDStr := strings.TrimSpace(req.BuildingModelId)
	if buildingModelIDStr == "" {
		return nil, fmt.Errorf("invalid building_model_id")
	}

	buildingModel, err := s.buildingRepo.FindBuildingModelByModelID(ctx, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find building model: %w", err)
	}
	if buildingModel == nil {
		return nil, fmt.Errorf("building model not found")
	}

	existingBuilding, err := s.buildingRepo.FindBuildingByFeatureAndModel(ctx, req.FeatureId, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find existing building: %w", err)
	}
	if existingBuilding == nil {
		return nil, fmt.Errorf("building not found")
	}

	if req.Information == nil {
		return nil, fmt.Errorf("invalid information: information is required")
	}

	mergedInformation, err := mergeBuildingInformation(existingBuilding.Information, req.Information)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateBuildingInformation(mergedInformation); err != nil {
		return nil, fmt.Errorf("invalid building information: %w", err)
	}

	informationJSON, err := marshalBuildingInformation(mergedInformation)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(mergedInformation.ActivityLine) != "" {
		if _, err := s.buildingRepo.FirstOrCreateIsicCode(ctx, strings.TrimSpace(mergedInformation.ActivityLine)); err != nil {
			return nil, fmt.Errorf("failed to create ISIC code: %w", err)
		}
	}

	if err := s.buildingRepo.UpdateBuildingInformation(ctx, req.FeatureId, buildingModelIDStr, informationJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("building not found")
		}
		return nil, fmt.Errorf("failed to update building information: %w", err)
	}

	return mergedInformation, nil
}

func mergeBuildingInformation(existingJSON string, update *pb.BuildingInformation) (*pb.BuildingInformation, error) {
	merged := &pb.BuildingInformation{}
	if strings.TrimSpace(existingJSON) != "" {
		if err := json.Unmarshal([]byte(existingJSON), merged); err != nil {
			return nil, fmt.Errorf("invalid existing building information: %w", err)
		}
	}

	if update.ActivityLine != "" {
		merged.ActivityLine = strings.TrimSpace(update.ActivityLine)
	}
	if update.Name != "" {
		merged.Name = strings.TrimSpace(update.Name)
	}
	if update.Address != "" {
		merged.Address = strings.TrimSpace(update.Address)
	}
	if update.PostalCode != "" {
		merged.PostalCode = strings.TrimSpace(update.PostalCode)
	}
	if update.Website != "" {
		merged.Website = strings.TrimSpace(update.Website)
	}
	if update.Description != "" {
		merged.Description = strings.TrimSpace(update.Description)
	}

	if merged.ActivityLine == "" && merged.Name == "" && merged.Address == "" &&
		merged.PostalCode == "" && merged.Website == "" && merged.Description == "" {
		return nil, fmt.Errorf("invalid information: at least one field is required")
	}

	return merged, nil
}

func marshalBuildingInformation(info *pb.BuildingInformation) (string, error) {
	infoMap := make(map[string]interface{})
	if info.ActivityLine != "" {
		infoMap["activity_line"] = info.ActivityLine
	}
	if info.Name != "" {
		infoMap["name"] = info.Name
	}
	if info.Address != "" {
		infoMap["address"] = info.Address
	}
	if info.PostalCode != "" {
		infoMap["postal_code"] = info.PostalCode
	}
	if info.Website != "" {
		infoMap["website"] = info.Website
	}
	if info.Description != "" {
		infoMap["description"] = info.Description
	}

	infoBytes, err := json.Marshal(infoMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal information: %w", err)
	}
	return string(infoBytes), nil
}

// DestroyBuilding removes a building from a feature and refunds invested satisfaction
// buildingModelID is the string model_id from 3D API
func (s *BuildingService) DestroyBuilding(ctx context.Context, featureID uint64, buildingModelID string) error {
	buildingModelIDStr := strings.TrimSpace(buildingModelID)
	_, _, user, err := s.ownedFeature(ctx, featureID)
	if err != nil {
		return err
	}

	launchedRaw, err := s.buildingRepo.DeleteBuilding(ctx, featureID, buildingModelIDStr)
	if err != nil {
		return err
	}
	var launchedSat float64
	if strings.TrimSpace(launchedRaw) != "" {
		launchedSat, err = strconv.ParseFloat(launchedRaw, 64)
		if err != nil {
			return fmt.Errorf("invalid launched_satisfaction for refund: %w", err)
		}
	}

	activateErr := s.hourlyProfitRepo.ActivateProfitsForFeature(ctx, featureID)
	refundErr := s.refundSatisfaction(ctx, user.UserID, launchedSat)
	if activateErr != nil && refundErr != nil {
		return fmt.Errorf("failed to reactivate profits: %w; %v", activateErr, refundErr)
	}
	if refundErr != nil {
		return refundErr
	}
	if activateErr != nil {
		return fmt.Errorf("failed to reactivate profits: %w", activateErr)
	}
	return nil
}
