package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"metargb/features-service/internal/client"
	"metargb/features-service/internal/constants"
	"metargb/features-service/internal/repository"
	"metargb/features-service/pkg/threed_client"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/auth"
	"metargb/shared/pkg/helpers"
)

type BuildingService struct {
	buildingRepo     *repository.BuildingRepository
	featureRepo      *repository.FeatureRepository
	geometryRepo     *repository.GeometryRepository
	hourlyProfitRepo *repository.HourlyProfitRepository
	threeDClient     *threed_client.Client
	commercialClient *client.CommercialClient
}

func NewBuildingService(
	buildingRepo *repository.BuildingRepository,
	featureRepo *repository.FeatureRepository,
	geometryRepo *repository.GeometryRepository,
	hourlyProfitRepo *repository.HourlyProfitRepository,
	threeDClient *threed_client.Client,
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
func (s *BuildingService) SetCommercialClient(client *client.CommercialClient) {
	s.commercialClient = client
}

// GetBuildPackage retrieves building models from 3D Meta API
// Checks ownership, calls 3D API, calculates required_satisfaction, upserts models, and returns with coordinates
func (s *BuildingService) GetBuildPackage(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error) {
	// Get feature with properties
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, nil, fmt.Errorf("feature not found: %w", err)
	}

	// Get user from context for ownership check
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unauthorized: authentication required")
	}

	// Ownership check: user must own the feature
	if feature.OwnerID != user.UserID {
		return nil, nil, fmt.Errorf("unauthorized: user does not own this feature")
	}

	// Get coordinates for feature
	coordinates, err := s.geometryRepo.GetCoordinatesByFeatureID(ctx, featureID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get coordinates: %w", err)
	}

	// Get density from properties (default to 1 if not set)
	density := properties.Density
	if density == 0 {
		density = 1 // Default to 1 if density is 0
	}

	// Call 3D Meta API
	apiResp, err := s.threeDClient.GetBuildPackage(threed_client.BuildPackageRequest{
		FeatureID: featureID,
		Area:      fmt.Sprintf("%.2f", properties.Area),
		Density:   fmt.Sprintf("%d", density),
		Karbari:   properties.Karbari,
		Page:      page,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("3D API call failed: %w", err)
	}

	// Get karbari coefficient
	karbariCoeff := constants.GetKarbariCoefficient(properties.Karbari)

	// Convert API response to protobuf models and calculate required_satisfaction
	models := make([]*pb.BuildingModel, 0, len(apiResp.Data))
	for _, item := range apiResp.Data {
		imagesJSON, _ := json.Marshal(item.Images)
		attrsJSON, _ := json.Marshal(item.Attributes)
		fileJSON, _ := json.Marshal(item.File)

		// Calculate required_satisfaction: area * karbariCoefficient * density * 0.1 / 100
		requiredSatisfaction := properties.Area * karbariCoeff * float64(density) * 0.1 / 100.0

		// Upsert building model locally
		err = s.buildingRepo.UpsertBuildingModel(ctx, item.ID, item.Name, item.SKU,
			string(imagesJSON), string(attrsJSON), string(fileJSON), requiredSatisfaction)
		if err != nil {
			// Log error but continue processing other models
			fmt.Printf("failed to upsert building model %s: %v\n", item.ID, err)
		}

		models = append(models, &pb.BuildingModel{
			ModelId:              item.ID,
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
// Returns the Feature object with building models loaded (matching Laravel response)
func (s *BuildingService) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error) {
	// 1. Get feature and validate ownership
	feature, _, err := s.featureRepo.FindByID(ctx, req.FeatureId)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Get user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: authentication required")
	}

	if feature.OwnerID != user.UserID {
		return nil, fmt.Errorf("unauthorized: user does not own this feature")
	}

	// 2. Check if feature already has a building
	hasBuilding, err := s.buildingRepo.HasBuilding(ctx, req.FeatureId)
	if err != nil {
		return nil, fmt.Errorf("failed to check building existence: %w", err)
	}
	if hasBuilding {
		return nil, fmt.Errorf("feature already has a building")
	}

	// 3. Get building model
	buildingModelIDStr := strings.TrimSpace(req.BuildingModelId)
	buildingModel, err := s.buildingRepo.FindBuildingModelByModelID(ctx, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find building model: %w", err)
	}
	if buildingModel == nil {
		return nil, fmt.Errorf("building model not found")
	}

	// 4. Validate launched_satisfaction
	launchedSatisfaction, err := strconv.ParseFloat(req.LaunchedSatisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid launched_satisfaction: %w", err)
	}

	requiredSatisfaction, err := strconv.ParseFloat(buildingModel.RequiredSatisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid required_satisfaction: %w", err)
	}

	if launchedSatisfaction < requiredSatisfaction {
		return nil, fmt.Errorf("launched_satisfaction must be at least %f", requiredSatisfaction)
	}

	// Get user wallet satisfaction
	if s.commercialClient == nil {
		return nil, fmt.Errorf("commercial client not available")
	}
	wallet, err := s.commercialClient.GetWallet(ctx, user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	walletSatisfaction, err := strconv.ParseFloat(wallet.Satisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid wallet satisfaction: %w", err)
	}

	if launchedSatisfaction > walletSatisfaction {
		return nil, fmt.Errorf("insufficient satisfaction: required %f, available %f", launchedSatisfaction, walletSatisfaction)
	}

	// 5. Deduct satisfaction from wallet before creating building
	err = s.commercialClient.DeductBalance(ctx, user.UserID, "satisfaction", launchedSatisfaction)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct satisfaction: %w", err)
	}

	// 6. Validate rotation
	_, err = strconv.ParseFloat(req.Rotation, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid rotation: %w", err)
	}

	// 7. Validate position format (regex: ^(-?\d+(\.\d+)?),\s*(-?\d+(\.\d+)?)$)
	positionRegex := regexp.MustCompile(`^(-?\d+(\.\d+)?),\s*(-?\d+(\.\d+)?)$`)
	if !positionRegex.MatchString(req.Position) {
		return nil, fmt.Errorf("invalid position format: expected 'x,y'")
	}

	// 8. Validate and build information JSON only if activity_line is provided
	var informationJSON string
	if req.Information != nil && req.Information.ActivityLine != "" {
		// Validate BuildingInformation fields
		if err := s.validateBuildingInformation(req.Information); err != nil {
			return nil, fmt.Errorf("invalid building information: %w", err)
		}

		// Only create information JSON if activity_line is provided
		infoMap := make(map[string]interface{})
		infoMap["activity_line"] = strings.TrimSpace(req.Information.ActivityLine)

		if req.Information.Name != "" {
			infoMap["name"] = strings.TrimSpace(req.Information.Name)
		}
		if req.Information.Address != "" {
			infoMap["address"] = strings.TrimSpace(req.Information.Address)
		}
		if req.Information.PostalCode != "" {
			infoMap["postal_code"] = strings.TrimSpace(req.Information.PostalCode)
		}
		if req.Information.Website != "" {
			infoMap["website"] = strings.TrimSpace(req.Information.Website)
		}
		if req.Information.Description != "" {
			infoMap["description"] = strings.TrimSpace(req.Information.Description)
		}

		infoBytes, err := json.Marshal(infoMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal information: %w", err)
		}
		informationJSON = string(infoBytes)

		// Create ISIC code (trimmed)
		_, err = s.buildingRepo.FirstOrCreateIsicCode(ctx, strings.TrimSpace(req.Information.ActivityLine))
		if err != nil {
			return nil, fmt.Errorf("failed to create ISIC code: %w", err)
		}
	}
	// If activity_line not provided, informationJSON remains empty string

	// 9. Calculate construction end date
	// Duration in hours: buildingModel.required_satisfaction * 288000 / launched_satisfaction
	constructionDurationHours := requiredSatisfaction * 288000.0 / launchedSatisfaction
	constructionStartDate := time.Now()
	// Convert hours to seconds for time.Duration
	constructionEndDate := constructionStartDate.Add(time.Duration(constructionDurationHours * 3600) * time.Second)

	// 10. Deactivate hourly profits for this feature
	if err := s.hourlyProfitRepo.DeactivateProfitsForFeature(ctx, req.FeatureId); err != nil {
		// Refund wallet on error
		s.commercialClient.AddBalance(ctx, user.UserID, "satisfaction", launchedSatisfaction)
		return nil, fmt.Errorf("failed to deactivate profits: %w", err)
	}

	// 11. Calculate bubble diameter from model attributes
	// Attributes are stored as JSON string in buildingModel.Attributes
	bubbleDiameter := s.calculateBubbleDiameter(buildingModel.Attributes)

	// 12. Create building record
	buildingModelIDStr = strings.TrimSpace(req.BuildingModelId)
	err = s.buildingRepo.CreateBuilding(ctx, req.FeatureId, buildingModelIDStr,
		req.LaunchedSatisfaction, req.Rotation, req.Position, informationJSON,
		constructionStartDate, constructionEndDate, bubbleDiameter)
	if err != nil {
		// Rollback: reactivate profits and refund wallet on error
		s.hourlyProfitRepo.ActivateProfitsForFeature(ctx, req.FeatureId)
		s.commercialClient.AddBalance(ctx, user.UserID, "satisfaction", launchedSatisfaction)
		return nil, fmt.Errorf("failed to create building: %w", err)
	}

	// 13. Load and return Feature with building models (matching Laravel response)
	buildings, err := s.buildingRepo.FindByFeatureID(ctx, req.FeatureId)
	if err != nil {
		// Log error but return feature anyway
		buildings = nil
	}

	// Build minimal Feature response with building models
	// Note: We don't have all FeatureService dependencies, so we return minimal Feature
	// with just the essential fields and building models
	return &pb.Feature{
		Id:            feature.ID,
		OwnerId:      feature.OwnerID,
		BuildingModels: buildings,
	}, nil
}

// ExtractAttributeValue extracts a value by slug from attributes array
// Attributes format: [{"slug": "width", "value": 50}, ...]
func ExtractAttributeValue(attributes []map[string]interface{}, slug string) (float64, bool) {
	for _, attr := range attributes {
		if s, ok := attr["slug"].(string); ok && s == slug {
			if v, ok := attr["value"].(float64); ok {
				return v, true
			}
		}
	}
	return 0, false
}

// calculateBubbleDiameter calculates bubble diameter from model attributes
// Expects attributes JSON string with array format: [{"slug": "width", "value": 50}, ...]
// Formula: perimeter × coefficient where:
//   - perimeter = 2 × (width + length)
//   - coefficient = 1 + (0.3 × (density - 1))
func (s *BuildingService) calculateBubbleDiameter(attributesJSON string) float64 {
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
	coefficient := 1.0
	for i := 1; i < int(density); i++ {
		coefficient += 0.3
	}

	// Final diameter: perimeter × coefficient
	return perimeter * coefficient
}

// validateBuildingInformation validates BuildingInformation fields according to Laravel rules
// Rules:
// - activity_line: nullable, max 255
// - name: nullable, max 255 (only saved if activity_line provided)
// - address: nullable, max 255
// - postal_code: nullable, iranian_postal_code (10 digits)
// - website: nullable, active_url, max 255 (DNS check)
// - description: nullable, max 5000
func (s *BuildingService) validateBuildingInformation(info *pb.BuildingInformation) error {
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
		
		// Validate 10 digits
		postalCodeRegex := regexp.MustCompile(`^[0-9]{10}$`)
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

// GetBuildings retrieves all buildings on a feature with Jalali formatted dates
func (s *BuildingService) GetBuildings(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
	buildings, err := s.buildingRepo.FindByFeatureID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildings: %w", err)
	}

	// Format dates to Jalali format
	for _, building := range buildings {
		if building.ConstructionStartDate != "" {
			// Try multiple date formats that MySQL might return
			dateFormats := []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05",
				time.RFC3339,
			}
			var t time.Time
			var err error
			for _, format := range dateFormats {
				if t, err = time.Parse(format, building.ConstructionStartDate); err == nil {
					building.ConstructionStartDate = helpers.FormatJalaliDateTime(t)
					break
				}
			}
		}
		if building.ConstructionEndDate != "" {
			dateFormats := []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05",
				time.RFC3339,
			}
			var t time.Time
			var err error
			for _, format := range dateFormats {
				if t, err = time.Parse(format, building.ConstructionEndDate); err == nil {
					building.ConstructionEndDate = helpers.FormatJalaliDateTime(t)
					break
				}
			}
		}
		// Format launched_satisfaction to 4 decimals
		if building.LaunchedSatisfaction != "" {
			if sat, err := strconv.ParseFloat(building.LaunchedSatisfaction, 64); err == nil {
				building.LaunchedSatisfaction = fmt.Sprintf("%.4f", sat)
			}
		}
	}

	return buildings, nil
}

// UpdateBuilding updates an existing building
func (s *BuildingService) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error) {
	// 1. Get feature and validate ownership
	feature, _, err := s.featureRepo.FindByID(ctx, req.FeatureId)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Get user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: authentication required")
	}

	if feature.OwnerID != user.UserID {
		return nil, fmt.Errorf("unauthorized: user does not own this feature")
	}

	// 2. Get building model
	buildingModelIDStr := strings.TrimSpace(req.BuildingModelId)
	buildingModel, err := s.buildingRepo.FindBuildingModelByModelID(ctx, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find building model: %w", err)
	}
	if buildingModel == nil {
		return nil, fmt.Errorf("building model not found")
	}

	// 3. Validate launched_satisfaction
	launchedSatisfaction, err := strconv.ParseFloat(req.LaunchedSatisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid launched_satisfaction: %w", err)
	}

	requiredSatisfaction, err := strconv.ParseFloat(buildingModel.RequiredSatisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid required_satisfaction: %w", err)
	}

	if launchedSatisfaction < requiredSatisfaction {
		return nil, fmt.Errorf("launched_satisfaction must be at least %f", requiredSatisfaction)
	}

	// Get user wallet satisfaction
	if s.commercialClient == nil {
		return nil, fmt.Errorf("commercial client not available")
	}
	wallet, err := s.commercialClient.GetWallet(ctx, user.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	walletSatisfaction, err := strconv.ParseFloat(wallet.Satisfaction, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid wallet satisfaction: %w", err)
	}

	if launchedSatisfaction > walletSatisfaction {
		return nil, fmt.Errorf("insufficient satisfaction: required %f, available %f", launchedSatisfaction, walletSatisfaction)
	}

	// 4. Validate rotation
	_, err = strconv.ParseFloat(req.Rotation, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid rotation: %w", err)
	}

	// 5. Validate position format
	positionRegex := regexp.MustCompile(`^(-?\d+(\.\d+)?),\s*(-?\d+(\.\d+)?)$`)
	if !positionRegex.MatchString(req.Position) {
		return nil, fmt.Errorf("invalid position format: expected 'x,y'")
	}

	// 6. Get existing building to preserve start date and bubble diameter
	existingBuilding, err := s.buildingRepo.FindBuildingByFeatureAndModel(ctx, req.FeatureId, buildingModelIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find existing building: %w", err)
	}
	if existingBuilding == nil {
		return nil, fmt.Errorf("building not found")
	}

	// 7. Validate and build information JSON only if activity_line is provided
	var informationJSON string
	if req.Information != nil && req.Information.ActivityLine != "" {
		// Validate BuildingInformation fields
		if err := s.validateBuildingInformation(req.Information); err != nil {
			return nil, fmt.Errorf("invalid building information: %w", err)
		}

		// Only create information JSON if activity_line is provided
		infoMap := make(map[string]interface{})
		infoMap["activity_line"] = strings.TrimSpace(req.Information.ActivityLine)

		if req.Information.Name != "" {
			infoMap["name"] = strings.TrimSpace(req.Information.Name)
		}
		if req.Information.Address != "" {
			infoMap["address"] = strings.TrimSpace(req.Information.Address)
		}
		if req.Information.PostalCode != "" {
			infoMap["postal_code"] = strings.TrimSpace(req.Information.PostalCode)
		}
		if req.Information.Website != "" {
			infoMap["website"] = strings.TrimSpace(req.Information.Website)
		}
		if req.Information.Description != "" {
			infoMap["description"] = strings.TrimSpace(req.Information.Description)
		}

		infoBytes, err := json.Marshal(infoMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal information: %w", err)
		}
		informationJSON = string(infoBytes)

		// Create ISIC code (trimmed)
		_, err = s.buildingRepo.FirstOrCreateIsicCode(ctx, strings.TrimSpace(req.Information.ActivityLine))
		if err != nil {
			return nil, fmt.Errorf("failed to create ISIC code: %w", err)
		}
	}
	// If activity_line not provided, informationJSON remains empty string (will preserve existing if not updating)

	// 8. Recalculate construction end date using updated satisfaction
	// Duration in hours: buildingModel.required_satisfaction * 288000 / launched_satisfaction
	constructionDurationHours := requiredSatisfaction * 288000.0 / launchedSatisfaction

	// Parse start date from existing building
	// The date comes from database in MySQL datetime format: "2006-01-02 15:04:05"
	var constructionStartDate time.Time
	if existingBuilding.ConstructionStartDate != "" {
		dateFormats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05",
			time.RFC3339,
		}
		parsed := false
		for _, format := range dateFormats {
			if t, err := time.Parse(format, existingBuilding.ConstructionStartDate); err == nil {
				constructionStartDate = t
				parsed = true
				break
			}
		}
		if !parsed {
			// If we can't parse, use current time (fallback)
			constructionStartDate = time.Now()
		}
	} else {
		constructionStartDate = time.Now()
	}

	// Convert hours to seconds for time.Duration
	constructionEndDate := constructionStartDate.Add(time.Duration(constructionDurationHours * 3600) * time.Second)

	// 9. Preserve existing bubble diameter (don't recalculate on update)
	existingBubbleDiameter, _ := strconv.ParseFloat(existingBuilding.BubbleDiameter, 64)

	// 10. Update building (preserve existing bubble diameter)
	updatedBuilding, err := s.buildingRepo.UpdateBuilding(ctx, req.FeatureId, buildingModelIDStr,
		req.LaunchedSatisfaction, req.Rotation, req.Position, informationJSON,
		constructionEndDate, existingBubbleDiameter)
	if err != nil {
		return nil, fmt.Errorf("failed to update building: %w", err)
	}

	// Format dates to Jalali
	dateFormats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	if updatedBuilding.ConstructionStartDate != "" {
		for _, format := range dateFormats {
			if t, err := time.Parse(format, updatedBuilding.ConstructionStartDate); err == nil {
				updatedBuilding.ConstructionStartDate = helpers.FormatJalaliDateTime(t)
				break
			}
		}
	}
	if updatedBuilding.ConstructionEndDate != "" {
		for _, format := range dateFormats {
			if t, err := time.Parse(format, updatedBuilding.ConstructionEndDate); err == nil {
				updatedBuilding.ConstructionEndDate = helpers.FormatJalaliDateTime(t)
				break
			}
		}
	}
	if updatedBuilding.LaunchedSatisfaction != "" {
		if sat, err := strconv.ParseFloat(updatedBuilding.LaunchedSatisfaction, 64); err == nil {
			updatedBuilding.LaunchedSatisfaction = fmt.Sprintf("%.4f", sat)
		}
	}

	return updatedBuilding, nil
}

// DestroyBuilding removes a building from a feature and refunds invested satisfaction
// buildingModelID is the string model_id from 3D API
func (s *BuildingService) DestroyBuilding(ctx context.Context, featureID uint64, buildingModelID string) error {
	buildingModelIDStr := strings.TrimSpace(buildingModelID)
	// Check ownership
	feature, _, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return fmt.Errorf("feature not found: %w", err)
	}

	// Get user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf("unauthorized: authentication required")
	}

	if feature.OwnerID != user.UserID {
		return fmt.Errorf("unauthorized: user does not own this feature")
	}

	// Get building to retrieve launched_satisfaction for refund
	building, err := s.buildingRepo.FindBuildingByFeatureAndModel(ctx, featureID, buildingModelIDStr)
	if err != nil {
		return fmt.Errorf("failed to find building: %w", err)
	}
	if building == nil {
		return fmt.Errorf("building not found")
	}

	// Parse launched_satisfaction for refund
	launchedSat, err := strconv.ParseFloat(building.LaunchedSatisfaction, 64)
	if err != nil {
		// Log error but continue with deletion
		fmt.Printf("Warning: failed to parse launched_satisfaction for refund: %v\n", err)
		launchedSat = 0
	}

	// Delete building first
	if err := s.buildingRepo.DeleteBuilding(ctx, featureID, buildingModelIDStr); err != nil {
		return fmt.Errorf("failed to delete building: %w", err)
	}

	// Reactivate hourly profits when building is destroyed
	if err := s.hourlyProfitRepo.ActivateProfitsForFeature(ctx, featureID); err != nil {
		// Log error but don't fail - building already deleted
		fmt.Printf("Warning: failed to reactivate profits: %v\n", err)
	}

	// Refund satisfaction to wallet
	if launchedSat > 0 && s.commercialClient != nil {
		if err := s.commercialClient.AddBalance(ctx, user.UserID, "satisfaction", launchedSat); err != nil {
			// Log error but don't fail - building already deleted
			fmt.Printf("Warning: failed to refund satisfaction: %v\n", err)
		}
	}

	return nil
}
