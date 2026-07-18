// Package service implements business logic for the dynasty service.
package service

import (
	"context"
	"fmt"
	"time"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
)

type DynastyService struct {
	dynastyRepo             *repository.DynastyRepository
	familyRepo              *repository.FamilyRepository
	prizeRepo               *repository.PrizeRepository
	notificationServiceAddr string
}

func NewDynastyService(
	dynastyRepo *repository.DynastyRepository,
	familyRepo *repository.FamilyRepository,
	prizeRepo *repository.PrizeRepository,
	notificationServiceAddr string,
) *DynastyService {
	return &DynastyService{
		dynastyRepo:             dynastyRepo,
		familyRepo:              familyRepo,
		prizeRepo:               prizeRepo,
		notificationServiceAddr: notificationServiceAddr,
	}
}

// CreateDynasty creates a new dynasty for a user
func (s *DynastyService) CreateDynasty(ctx context.Context, userID, featureID uint64) (*models.Dynasty, *models.Family, error) {
	// Check if user already has a dynasty
	existing, err := s.dynastyRepo.GetDynastyByUserID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check existing dynasty: %w", err)
	}
	if existing != nil {
		return nil, nil, fmt.Errorf("user already has a dynasty")
	}

	// Create dynasty
	dynasty := &models.Dynasty{
		UserID:    userID,
		FeatureID: featureID,
	}
	if err := s.dynastyRepo.CreateDynasty(ctx, dynasty); err != nil {
		return nil, nil, fmt.Errorf("failed to create dynasty: %w", err)
	}

	// Create family
	family, err := s.familyRepo.CreateFamily(ctx, dynasty.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create family: %w", err)
	}

	// Add user as owner family member
	member := &models.FamilyMember{
		FamilyID:     family.ID,
		UserID:       userID,
		Relationship: "owner",
	}
	if err := s.familyRepo.CreateFamilyMember(ctx, member); err != nil {
		return nil, nil, fmt.Errorf("failed to add owner to family: %w", err)
	}

	// TODO: Send notification via gRPC call to notification service
	// This would be implemented once notification service is ready

	return dynasty, family, nil
}

// GetDynastyByID retrieves a dynasty by ID
func (s *DynastyService) GetDynastyByID(ctx context.Context, id uint64) (*models.Dynasty, error) {
	dynasty, err := s.dynastyRepo.GetDynastyByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return nil, fmt.Errorf("dynasty not found")
	}

	return dynasty, nil
}

// GetDynastyByUserID retrieves a dynasty by user ID
func (s *DynastyService) GetDynastyByUserID(ctx context.Context, userID uint64) (*models.Dynasty, error) {
	dynasty, err := s.dynastyRepo.GetDynastyByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty: %w", err)
	}

	return dynasty, nil
}

// UpdateDynastyFeature updates the feature associated with a dynasty
func (s *DynastyService) UpdateDynastyFeature(ctx context.Context, dynastyID, featureID, userID uint64) error {
	// Get dynasty to verify ownership
	dynasty, err := s.dynastyRepo.GetDynastyByID(ctx, dynastyID)
	if err != nil {
		return fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return fmt.Errorf("dynasty not found")
	}
	if dynasty.UserID != userID {
		return fmt.Errorf("unauthorized: user does not own this dynasty")
	}
	if dynasty.FeatureID == featureID {
		return fmt.Errorf("feature is already dynasty feature")
	}

	// Apply penalty rules if changing within 30 days.
	if time.Since(dynasty.UpdatedAt) < 30*24*time.Hour {
		karbari, stability, err := s.dynastyRepo.GetFeaturePenaltyData(ctx, dynasty.FeatureID)
		if err != nil {
			return fmt.Errorf("failed to get feature penalty data: %w", err)
		}
		colorType := featureColorByKarbari(karbari)
		debtAmount := stability * 0.01
		if debtAmount > 0 {
			if err := s.dynastyRepo.CreateDebt(ctx, userID, colorType, debtAmount, "update-dynasty-feature"); err != nil {
				return fmt.Errorf("failed to create debt: %w", err)
			}
		}
		if err := s.dynastyRepo.LockFeature(ctx, dynasty.FeatureID, "dynasty-feature-change", time.Now().AddDate(0, 1, 0), 0); err != nil {
			return fmt.Errorf("failed to lock feature: %w", err)
		}
		if err := s.dynastyRepo.SetFeatureLabel(ctx, dynasty.FeatureID, "locked"); err != nil {
			return fmt.Errorf("failed to set feature label: %w", err)
		}
	}

	// Update dynasty feature
	if err := s.dynastyRepo.UpdateDynastyFeature(ctx, dynastyID, featureID); err != nil {
		return fmt.Errorf("failed to update dynasty feature: %w", err)
	}

	return nil
}

func featureColorByKarbari(karbari string) string {
	switch karbari {
	case "m":
		return "yellow"
	case "t":
		return "red"
	case "a":
		return "blue"
	default:
		return "yellow"
	}
}

// GetFeatureDetails retrieves feature details for dynasty
func (s *DynastyService) GetFeatureDetails(ctx context.Context, featureID uint64) (map[string]interface{}, error) {
	return s.dynastyRepo.GetFeatureDetails(ctx, featureID)
}

// GetUserFeatures retrieves user's features
func (s *DynastyService) GetUserFeatures(ctx context.Context, userID, excludeFeatureID uint64) ([]map[string]interface{}, error) {
	return s.dynastyRepo.GetUserFeatures(ctx, userID, excludeFeatureID)
}

// GetUserProfilePhoto retrieves user's profile photo
func (s *DynastyService) GetUserProfilePhoto(ctx context.Context, userID uint64) (*string, error) {
	return s.dynastyRepo.GetUserProfilePhoto(ctx, userID)
}

// GetFamilyByDynastyID retrieves family by dynasty ID
func (s *DynastyService) GetFamilyByDynastyID(ctx context.Context, dynastyID uint64) (*models.Family, error) {
	return s.familyRepo.GetFamilyByDynastyID(ctx, dynastyID)
}

// GetFamilyMemberCount retrieves the count of family members
func (s *DynastyService) GetFamilyMemberCount(ctx context.Context, familyID uint64) (int32, error) {
	return s.familyRepo.GetFamilyMemberCount(ctx, familyID)
}

// GetIntroductionPrizes retrieves introduction prizes (for users without dynasty)
func (s *DynastyService) GetIntroductionPrizes(ctx context.Context) ([]*models.DynastyPrize, error) {
	return s.prizeRepo.GetAllDynastyPrizes(ctx)
}

// GetVariableRate retrieves a variable rate by asset name.
func (s *DynastyService) GetVariableRate(ctx context.Context, asset string) (float64, error) {
	return s.dynastyRepo.GetVariableRate(ctx, asset)
}
