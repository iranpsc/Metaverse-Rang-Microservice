package service

import (
	"context"
	"fmt"
	"time"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/repository"
)

const (
	adultAgeYears = 18
	// hoursPerYear matches the existing age check (365.25 days, including leap years).
	hoursPerYear = 365.25 * 24
)

type OrderPolicy interface {
	CanBuyFromStore(ctx context.Context, userID uint64) (bool, error)
	CanGetBonus(ctx context.Context, userID uint64, asset string) (bool, error)
}

type orderPolicy struct {
	eligibilityRepo repository.EligibilityRepository
	firstOrderRepo  repository.FirstOrderRepository
}

func NewOrderPolicy(eligibilityRepo repository.EligibilityRepository, firstOrderRepo repository.FirstOrderRepository) OrderPolicy {
	return &orderPolicy{
		eligibilityRepo: eligibilityRepo,
		firstOrderRepo:  firstOrderRepo,
	}
}

// CanBuyFromStore blocks buyers under 18 unless child permissions are verified and the BFR flag is set.
// A missing birthdate is treated as an adult.
func (p *orderPolicy) CanBuyFromStore(ctx context.Context, userID uint64) (bool, error) {
	birthdate, err := p.eligibilityRepo.GetUserBirthdate(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check user age: %w", err)
	}
	if birthdate == nil || ageInYears(*birthdate) >= adultAgeYears {
		return true, nil
	}

	verified, bfr, found, err := p.eligibilityRepo.GetChildPermissions(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check permissions: %w", err)
	}
	return found && verified && bfr, nil
}

func ageInYears(birthdate time.Time) float64 {
	return time.Since(birthdate).Hours() / hoursPerYear
}

// CanGetBonus is true only when the user has no first-order record and the asset is not IRR.
func (p *orderPolicy) CanGetBonus(ctx context.Context, userID uint64, asset string) (bool, error) {
	if asset == constants.AssetIRR {
		return false, nil
	}

	count, err := p.firstOrderRepo.Count(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check first order: %w", err)
	}

	return count == 0, nil
}
