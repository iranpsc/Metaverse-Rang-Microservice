package service

import (
	"context"
	"fmt"
	"time"

	"metarang/dynasty-service/internal/repository"
)

// DynastyServiceEnhanced provides enhanced dynasty operations with business logic
type DynastyServiceEnhanced struct {
	dynastyRepo *repository.DynastyRepository
	familyRepo  *repository.FamilyRepository

	// gRPC clients (to be injected)
	// featureClient   features.FeatureServiceClient
	// commercialClient commercial.CommercialServiceClient
	// notificationClient notification.NotificationServiceClient
}

func NewDynastyServiceEnhanced(
	dynastyRepo *repository.DynastyRepository,
	familyRepo *repository.FamilyRepository,
) *DynastyServiceEnhanced {
	return &DynastyServiceEnhanced{
		dynastyRepo: dynastyRepo,
		familyRepo:  familyRepo,
	}
}

// UpdateDynastyFeature updates dynasty feature with debt and locking logic
func (s *DynastyServiceEnhanced) UpdateDynastyFeature(
	ctx context.Context,
	dynastyID, featureID, userID uint64,
) error {
	// 1. Get dynasty to verify ownership
	dynasty, err := s.dynastyRepo.GetDynastyByID(ctx, dynastyID)
	if err != nil {
		return fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return fmt.Errorf("dynasty not found")
	}

	// 2. Verify ownership
	if dynasty.UserID != userID {
		return fmt.Errorf("unauthorized: user does not own this dynasty")
	}

	// 3. Get current feature ID
	currentFeatureID := dynasty.FeatureID

	// 4. Check if same feature
	if currentFeatureID == featureID {
		return fmt.Errorf("feature is already the dynasty feature")
	}

	// 5. CRITICAL: Check if updated within last 30 days
	daysSinceLastUpdate := time.Since(dynasty.UpdatedAt).Hours() / 24

	if daysSinceLastUpdate < 30 {
		// PENALTY SYSTEM: Within 30 days of last update

		// TODO: Get feature properties to calculate debt
		// This would call Features service
		/*
			featureProps, err := s.featureClient.GetFeatureProperties(ctx, &features.GetFeaturePropertiesRequest{
				FeatureId: currentFeatureID,
			})
			if err != nil {
				return fmt.Errorf("failed to get feature properties: %w", err)
			}

			// Get feature color based on karbari
			// m = yellow, t = red, a = blue
			colorType := getFeatureColor(featureProps.Karbari)

			// Calculate debt: 1% of feature stability
			debtAmount := featureProps.Stability * 0.01

			// 6. Create debt via Commercial Service
			err = s.commercialClient.CreateDebt(ctx, &commercial.CreateDebtRequest{
				UserId: userID,
				Color:  colorType,
				Amount: debtAmount,
				Reason: "update-dynasty-feature",
			})
			if err != nil {
				return fmt.Errorf("failed to create debt: %w", err)
			}

			// 7. Lock the old feature for 1 month
			unlockDate := time.Now().AddDate(0, 1, 0) // 1 month from now
			err = s.featureClient.LockFeature(ctx, &features.LockFeatureRequest{
				FeatureId: currentFeatureID,
				Reason:    "dynasty-feature-change",
				Until:     unlockDate.Unix(),
				Status:    0, // 0 = locked
			})
			if err != nil {
				return fmt.Errorf("failed to lock feature: %w", err)
			}

			// 8. Update feature label to 'locked'
			err = s.featureClient.UpdateFeatureLabel(ctx, &features.UpdateFeatureLabelRequest{
				FeatureId: currentFeatureID,
				Label:     "locked",
			})
			if err != nil {
				return fmt.Errorf("failed to update feature label: %w", err)
			}
		*/

		// Log that penalties would be applied
		fmt.Printf("PENALTY: Would create debt and lock feature %d for user %d\n", currentFeatureID, userID)
	}

	// 9. Update dynasty feature
	if err := s.dynastyRepo.UpdateDynastyFeature(ctx, dynastyID, featureID); err != nil {
		return fmt.Errorf("failed to update dynasty feature: %w", err)
	}

	// 10. Send notification
	// TODO: Call Notifications service
	/*
		err = s.notificationClient.SendNotification(ctx, &notification.SendRequest{
			UserId: userID,
			Type:   "DynastyFeatureChangedNotification",
			Data: map[string]string{
				"feature_id": fmt.Sprintf("%d", featureID),
			},
		})
	*/

	return nil
}

// CalculateFeatureProfitIncrease calculates profit increase from stability
// Implements Laravel: (stability / 10000 - 1) if stability > 10000, else 0
func (s *DynastyServiceEnhanced) CalculateFeatureProfitIncrease(stability float64) string {
	if stability > 10000 {
		increase := (stability / 10000) - 1
		return fmt.Sprintf("%.3f", increase)
	}
	return "0"
}
