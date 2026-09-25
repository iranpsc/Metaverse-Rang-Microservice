package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"metarang/buildings-service/internal/constants"
	"metarang/buildings-service/internal/models"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"
	"metarang/shared/pkg/helpers"
)

var (
	positionFormatRegex = regexp.MustCompile(`^(-?\d+(\.\d+)?),\s*(-?\d+(\.\d+)?)$`)
	postalCodeRegex     = regexp.MustCompile(`^[0-9]{10}$`)
)

var mysqlDateTimeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

func parseMySQLDateTime(value string) (time.Time, bool) {
	for _, format := range mysqlDateTimeFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func constructionEndFromStart(start time.Time, requiredSatisfaction, launchedSatisfaction float64) time.Time {
	hours := requiredSatisfaction * 288000.0 / launchedSatisfaction
	return start.Add(time.Duration(hours*3600) * time.Second)
}

func formatBuildingDates(building *pb.Building) {
	if building == nil {
		return
	}
	if building.ConstructionStartDate != "" {
		if t, ok := parseMySQLDateTime(building.ConstructionStartDate); ok {
			building.ConstructionStartDate = helpers.FormatJalaliDateTime(t)
		}
	}
	if building.ConstructionEndDate != "" {
		if t, ok := parseMySQLDateTime(building.ConstructionEndDate); ok {
			building.ConstructionEndDate = helpers.FormatJalaliDateTime(t)
		}
	}
	if building.LaunchedSatisfaction != "" {
		if sat, err := strconv.ParseFloat(building.LaunchedSatisfaction, 64); err == nil {
			building.LaunchedSatisfaction = fmt.Sprintf("%.4f", sat)
		}
	}
}

func validateBuildPlacement(rotation, position string) error {
	if _, err := strconv.ParseFloat(rotation, 64); err != nil {
		return fmt.Errorf("invalid rotation: %w", err)
	}
	if !positionFormatRegex.MatchString(position) {
		return fmt.Errorf("invalid position format: expected 'x,y'")
	}
	return nil
}

func trimBuildingInformation(info *pb.BuildingInformation) *pb.BuildingInformation {
	if info == nil {
		return nil
	}
	return &pb.BuildingInformation{
		ActivityLine: strings.TrimSpace(info.ActivityLine),
		Name:         strings.TrimSpace(info.Name),
		Address:      strings.TrimSpace(info.Address),
		PostalCode:   strings.TrimSpace(info.PostalCode),
		Website:      strings.TrimSpace(info.Website),
		Description:  strings.TrimSpace(info.Description),
	}
}

func (s *BuildingService) ownedFeature(ctx context.Context, featureID uint64) (*models.Feature, *models.FeatureProperties, *auth.UserContext, error) {
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("feature not found: %w", err)
	}
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unauthorized: authentication required")
	}
	if feature.OwnerID != user.UserID {
		return nil, nil, nil, fmt.Errorf("unauthorized: user does not own this feature")
	}
	return feature, properties, user, nil
}

// requiredSatisfactionForFeature prices construction from the feature itself.
// The value must not be read from building_models, because that row is shared
// across every feature that uses the same 3D model.
func requiredSatisfactionForFeature(properties *models.FeatureProperties) (float64, error) {
	if properties == nil {
		return 0, fmt.Errorf("feature properties not found")
	}
	density := properties.Density
	if density < 0 {
		return 0, fmt.Errorf("invalid required_satisfaction")
	}
	if density == 0 {
		density = 1
	}
	coeff := constants.GetKarbariCoefficient(properties.Karbari)
	required := properties.Area * coeff * float64(density) * 0.1 / 100.0
	if required < 0 || math.IsNaN(required) || math.IsInf(required, 0) {
		return 0, fmt.Errorf("invalid required_satisfaction")
	}
	return required, nil
}

func validateLaunchedAmount(launchedRaw string, required float64) (float64, error) {
	launched, err := strconv.ParseFloat(strings.TrimSpace(launchedRaw), 64)
	if err != nil || launched <= 0 || math.IsNaN(launched) || math.IsInf(launched, 0) {
		return 0, fmt.Errorf("invalid launched_satisfaction")
	}
	if launched < required {
		return 0, fmt.Errorf("launched_satisfaction must be at least %f", required)
	}
	return launched, nil
}

func (s *BuildingService) loadBuildingModel(ctx context.Context, buildingModelID string) (*pb.BuildingModel, error) {
	modelID := strings.TrimSpace(buildingModelID)
	model, err := s.buildingRepo.FindBuildingModelByModelID(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to find building model: %w", err)
	}
	if model == nil {
		return nil, fmt.Errorf("building model not found")
	}
	return model, nil
}

func (s *BuildingService) walletSatisfaction(ctx context.Context, userID uint64) (float64, error) {
	if s.commercialClient == nil {
		return 0, fmt.Errorf("commercial client not available")
	}
	wallet, err := s.commercialClient.GetWallet(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get wallet: %w", err)
	}
	balance, err := strconv.ParseFloat(wallet.Satisfaction, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid wallet satisfaction: %w", err)
	}
	return balance, nil
}

func (s *BuildingService) chargeSatisfaction(ctx context.Context, userID uint64, amount float64) error {
	if amount <= 0 {
		return nil
	}
	balance, err := s.walletSatisfaction(ctx, userID)
	if err != nil {
		return err
	}
	if amount > balance {
		return fmt.Errorf("insufficient satisfaction: required %f, available %f", amount, balance)
	}
	if err := s.commercialClient.DeductBalance(ctx, userID, "satisfaction", amount); err != nil {
		return fmt.Errorf("failed to deduct satisfaction: %w", err)
	}
	return nil
}

func (s *BuildingService) refundSatisfaction(ctx context.Context, userID uint64, amount float64) error {
	if amount <= 0 {
		return nil
	}
	if s.commercialClient == nil {
		return fmt.Errorf("commercial client not available")
	}
	if err := s.commercialClient.AddBalance(ctx, userID, "satisfaction", amount); err != nil {
		return fmt.Errorf("failed to refund satisfaction: %w", err)
	}
	return nil
}

func (s *BuildingService) prepareInformationJSON(ctx context.Context, info *pb.BuildingInformation) (string, error) {
	if info == nil || strings.TrimSpace(info.ActivityLine) == "" {
		return "", nil
	}
	trimmed := trimBuildingInformation(info)
	if err := s.ValidateBuildingInformation(trimmed); err != nil {
		return "", fmt.Errorf("invalid building information: %w", err)
	}
	informationJSON, err := marshalBuildingInformation(trimmed)
	if err != nil {
		return "", err
	}
	if _, err := s.buildingRepo.FirstOrCreateIsicCode(ctx, trimmed.ActivityLine); err != nil {
		return "", fmt.Errorf("failed to create ISIC code: %w", err)
	}
	return informationJSON, nil
}

func parseExistingConstructionStart(existing *pb.Building) time.Time {
	if existing == nil || existing.ConstructionStartDate == "" {
		return time.Now()
	}
	if t, ok := parseMySQLDateTime(existing.ConstructionStartDate); ok {
		return t
	}
	return time.Now()
}
