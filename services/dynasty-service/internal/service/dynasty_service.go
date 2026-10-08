// Package service implements business logic for the dynasty service.
package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/validation"
)

type DynastyService struct {
	dynastyRepo             *repository.DynastyRepository
	familyRepo              *repository.FamilyRepository
	prizeRepo               *repository.PrizeRepository
	notificationServiceAddr string
	notifier                NotificationPort
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

// SetNotifier attaches the notification client used for dynasty lifecycle messages.
func (s *DynastyService) SetNotifier(notifier NotificationPort) {
	s.notifier = notifier
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

	if err := s.authorizeFeatureForDynasty(ctx, userID, featureID, true); err != nil {
		return nil, nil, err
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

	s.notifyDynasty(ctx, userID, "dynasty_created", "سلسله شما تاسیس شد.", "سلسله شما تاسیس شد.", map[string]string{
		"feature_id": fmt.Sprintf("%d", featureID),
	})

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

	if err := s.authorizeFeatureForDynasty(ctx, userID, featureID, false); err != nil {
		return err
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

	s.notifyDynasty(ctx, userID, "dynasty_feature_changed", "ملک بنای سلسله جایگزین شد.", "ملک بنای سلسله جایگزین شد.", map[string]string{
		"feature_id": fmt.Sprintf("%d", featureID),
	})

	return nil
}

// authorizeFeatureForDynasty enforces DynastyPolicy create/update rules against the selected feature.
// requireResidential is true for create (karbari must be maskoni) and false for feature replacement.
func (s *DynastyService) authorizeFeatureForDynasty(ctx context.Context, userID, featureID uint64, requireResidential bool) error {
	if requireResidential {
		verified, err := s.dynastyRepo.UserIsVerified(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to check user verification: %w", err)
		}
		if !verified {
			return &validation.ValidationError{Message: "باید احراز هویت را کامل کنید", Code: 403}
		}
	}

	ownerID, karbari, err := s.dynastyRepo.GetFeatureEligibility(ctx, featureID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return &validation.ValidationError{Message: "شما مالک این ملک نیستید", Code: 403}
	}
	if requireResidential && karbari != "m" {
		return &validation.ValidationError{Message: "این ملک مسکونی نیست", Code: 403}
	}

	hasPending, err := s.dynastyRepo.CheckFeatureHasPendingRequest(ctx, featureID)
	if err != nil {
		return fmt.Errorf("failed to check pending requests: %w", err)
	}
	if hasPending {
		return &validation.ValidationError{Message: "این ملک درخواست در انتظار دارد", Code: 403}
	}
	return nil
}

func (s *DynastyService) notifyDynasty(ctx context.Context, userID uint64, notificationType, title, message string, data map[string]string) {
	if s.notifier == nil {
		return
	}
	sendSMS := false
	if s.dynastyRepo != nil {
		verifiedPhone, err := s.dynastyRepo.UserHasVerifiedPhone(ctx, userID)
		if err != nil {
			log.Printf("Warning: dynasty notification phone lookup failed for user %d: %v", userID, err)
		}
		sendSMS = verifiedPhone
	}
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	if err := s.notifier.SendNotification(notifyCtx, userID, notificationType, title, message, data, sendSMS, false); err != nil {
		log.Printf("Warning: failed to send %s notification to user %d: %v", notificationType, userID, err)
	}
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

// GetFeatureDetails retrieves dynasty feature details, including polygon coordinates.
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
