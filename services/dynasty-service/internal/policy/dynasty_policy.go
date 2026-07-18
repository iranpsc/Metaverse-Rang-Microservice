// Package policy encodes authorization rules for the dynasty service.
package policy

import (
	"context"
	"fmt"

	"metarang/dynasty-service/internal/repository"
)

// DynastyPolicy enforces dynasty business rules
type DynastyPolicy struct {
	dynastyRepo    *repository.DynastyRepository
	featuresClient interface{} // Will be *client.FeaturesClient when injected
	authClient     interface{} // Will be *client.AuthClient when injected
}

func NewDynastyPolicy(dynastyRepo *repository.DynastyRepository) *DynastyPolicy {
	return &DynastyPolicy{
		dynastyRepo: dynastyRepo,
	}
}

// SetClients sets the gRPC clients (called after initialization)
func (p *DynastyPolicy) SetClients(featuresClient, authClient interface{}) {
	p.featuresClient = featuresClient
	p.authClient = authClient
}

// CanCreateDynasty checks if user can create a dynasty
// Implements DynastyPolicy::create from Laravel
func (p *DynastyPolicy) CanCreateDynasty(
	ctx context.Context,
	userID, featureID uint64,
) (bool, string, error) {
	// 1. Check if user is verified (KYC)
	// TODO: Would call Auth service to check KYC status
	/*
		isVerified, err := p.authClient.CheckKYCVerified(ctx, &auth.CheckKYCRequest{
			UserId: userID,
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to check KYC: %w", err)
		}
		if !isVerified {
			return false, "باید احراز هویت را کامل کنید", nil
		}
	*/

	// 2. Check if user doesn't have dynasty already
	dynasty, err := p.dynastyRepo.GetDynastyByUserID(ctx, userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to check existing dynasty: %w", err)
	}
	if dynasty != nil {
		return false, "شما قبلا سلسله تاسیس کرده اید", nil
	}

	// 3. Check feature is maskoni (residential, karbari = 'm') and user owns it
	// 4. Check feature has no pending requests
	// Note: These checks would be performed via Features service client
	// For now, we check via repository (direct DB access)
	hasPending, err := p.dynastyRepo.CheckFeatureHasPendingRequest(ctx, featureID)
	if err != nil {
		return false, "", fmt.Errorf("failed to check pending requests: %w", err)
	}
	if hasPending {
		return false, "این ملک درخواست در انتظار دارد", nil
	}

	return true, "", nil
}

// CanUpdateDynastyFeature checks if user can update dynasty feature
// Implements DynastyPolicy::update from Laravel
func (p *DynastyPolicy) CanUpdateDynastyFeature(
	ctx context.Context,
	userID, dynastyID, featureID uint64,
) (bool, string, error) {
	// Get dynasty
	dynasty, err := p.dynastyRepo.GetDynastyByID(ctx, dynastyID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return false, "سلسله یافت نشد", nil
	}

	// Check user owns the dynasty
	if dynasty.UserID != userID {
		return false, "شما مالک این سلسله نیستید", nil
	}

	// Check not the same feature
	if dynasty.FeatureID == featureID {
		return false, "این ملک هم اکنون ملک سلسله شماست", nil
	}

	// TODO: Check feature has no pending requests
	/*
		hasPending, err := p.featureClient.HasPendingRequests(ctx, &features.CheckPendingRequest{
			FeatureId: featureID,
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to check pending requests: %w", err)
		}
		if hasPending {
			return false, "این ملک درخواست در انتظار دارد", nil
		}

		// Check user owns the feature
		feature, err := p.featureClient.GetFeature(ctx, &features.GetFeatureRequest{
			FeatureId: featureID,
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to get feature: %w", err)
		}
		if feature.OwnerId != userID {
			return false, "شما مالک این ملک نیستید", nil
		}
	*/

	return true, "", nil
}
