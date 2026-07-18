package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/repository"
	"metarang/shared/pkg/logger"
)

// FeaturePricingService handles feature pricing updates
// Implements Laravel's FeatureController@updateFeature logic (lines 77-105)
type FeaturePricingService struct {
	featureRepo    *repository.FeatureRepository
	propertiesRepo *repository.PropertiesRepository
	db             *sql.DB
	log            *logger.Logger
}

func NewFeaturePricingService(
	featureRepo *repository.FeatureRepository,
	propertiesRepo *repository.PropertiesRepository,
	db *sql.DB,
	log *logger.Logger,
) *FeaturePricingService {
	return &FeaturePricingService{
		featureRepo:    featureRepo,
		propertiesRepo: propertiesRepo,
		db:             db,
		log:            log,
	}
}

// UpdateFeaturePricing updates feature pricing based on minimum_price_percentage
// Implements Laravel's FeatureController@updateFeature (lines 77-105)
func (s *FeaturePricingService) UpdateFeaturePricing(ctx context.Context, featureID, userID uint64, minimumPricePercentage int) error {
	// Load feature
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return fmt.Errorf("feature not found: %w", err)
	}

	// Verify ownership
	if feature.OwnerID != userID {
		return fmt.Errorf("unauthorized: not the owner")
	}

	// Check if user is under 18
	isUnder18, err := s.isUserUnder18(ctx, userID)
	if err != nil {
		s.log.Error("Failed to check user age", "error", err)
		isUnder18 = false
	}

	// Validate minimum_price_percentage
	// Under-18 users: minimum 110%
	// Regular users: minimum 80%
	minAllowed := constants.DefaultPublicPricingLimit
	if isUnder18 {
		minAllowed = constants.DefaultUnder18PricingLimit
	}

	if minimumPricePercentage < minAllowed {
		return fmt.Errorf("حداقل درصد قیمت مجاز برای شما %d%% می\u200cباشد", minAllowed)
	}

	// Calculate pricing
	// Formula from Laravel:
	// totalPrice = stability × colorRate × percentage / 100
	// price_psc = (totalPrice × 0.5) / pscRate
	// price_irr = totalPrice × 0.5

	color := constants.GetColor(properties.Karbari)
	colorRate := s.getVariableRate(ctx, color)
	pscRate := s.getVariableRate(ctx, "psc")

	totalPrice := properties.Stability * colorRate * float64(minimumPricePercentage) / 100.0

	// Split 50/50 between PSC and IRR
	pricePSC := (totalPrice * 0.5) / pscRate
	priceIRR := totalPrice * 0.5

	// Convert to strings (stored as VARCHAR in database)
	pricePSCStr := fmt.Sprintf("%.10f", pricePSC)
	priceIRRStr := fmt.Sprintf("%.10f", priceIRR)

	// Update properties
	if err := s.propertiesRepo.UpdatePricing(ctx, featureID, pricePSCStr, priceIRRStr, minimumPricePercentage); err != nil {
		return fmt.Errorf("failed to update pricing: %w", err)
	}

	s.log.Info("Feature pricing updated",
		"feature_id", featureID,
		"percentage", minimumPricePercentage,
		"price_psc", pricePSCStr,
		"price_irr", priceIRRStr,
	)

	return nil
}

// UpdateFeatureLabel updates the label field of a feature
func (s *FeaturePricingService) UpdateFeatureLabel(ctx context.Context, featureID, userID uint64, label string) error {
	// Load feature
	feature, _, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return fmt.Errorf("feature not found: %w", err)
	}

	// Verify ownership
	if feature.OwnerID != userID {
		return fmt.Errorf("unauthorized: not the owner")
	}

	// Update label
	updates := map[string]interface{}{
		"label": label,
	}

	if err := s.propertiesRepo.Update(ctx, featureID, updates); err != nil {
		return fmt.Errorf("failed to update label: %w", err)
	}

	return nil
}

// GetFeaturePriceInfo calculates price information for display
func (s *FeaturePricingService) GetFeaturePriceInfo(ctx context.Context, featureID uint64) (map[string]interface{}, error) {
	_, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	color := constants.GetColor(properties.Karbari)
	colorRate := s.getVariableRate(ctx, color)

	// Calculate stability value in IRR
	stabilityValueIRR := properties.Stability * colorRate

	// Parse current prices
	pricePSC := parseStringToFloat(properties.PricePSC)
	priceIRR := parseStringToFloat(properties.PriceIRR)
	pscRate := s.getVariableRate(ctx, "psc")

	// Calculate current price in IRR
	currentPriceIRR := priceIRR + (pricePSC * pscRate)

	// Calculate percentage of stability
	percentage := 0.0
	if stabilityValueIRR > 0 {
		percentage = (currentPriceIRR / stabilityValueIRR) * 100
	}

	return map[string]interface{}{
		"stability":           properties.Stability,
		"stability_value_irr": stabilityValueIRR,
		"price_psc":           pricePSC,
		"price_irr":           priceIRR,
		"current_price_irr":   currentPriceIRR,
		"percentage":          percentage,
		"min_percentage":      properties.MinimumPricePercentage,
		"color":               color,
		"karbari":             properties.Karbari,
	}, nil
}

// Utility methods

func (s *FeaturePricingService) isUserUnder18(ctx context.Context, userID uint64) (bool, error) {
	var birthdate sql.NullTime
	err := s.db.QueryRowContext(ctx, "SELECT birthdate FROM kycs WHERE user_id = ?", userID).Scan(&birthdate)
	if err != nil || !birthdate.Valid {
		return false, nil
	}

	// Calculate age correctly
	now := time.Now()
	age := float64(now.Sub(birthdate.Time).Hours()) / (365.25 * 24)
	return age < 18, nil
}

func (s *FeaturePricingService) getVariableRate(ctx context.Context, asset string) float64 {
	var rate float64
	query := "SELECT value FROM variables WHERE `key` = ?"
	if err := s.db.QueryRowContext(ctx, query, asset).Scan(&rate); err != nil {
		return 1.0 // Default
	}
	return rate
}

func parseStringToFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
