package service

import (
	"context"
	"fmt"
	"time"

	"metarang/financial-service/internal/repository"
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

// CanBuyFromStore checks if user can buy from store
// Laravel: UserPolicy::buyFromStore
// Rule: Blocks users under 18 unless permissions are verified and BFR flag is set; adults pass automatically
func (p *orderPolicy) CanBuyFromStore(ctx context.Context, userID uint64) (bool, error) {
	birthdate, err := p.eligibilityRepo.GetUserBirthdate(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check user age: %w", err)
	}

	if birthdate == nil {
		return true, nil
	}

	age := time.Since(*birthdate).Hours() / (365.25 * 24)
	if age >= 18 {
		return true, nil
	}

	verified, bfr, found, err := p.eligibilityRepo.GetChildPermissions(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check permissions: %w", err)
	}

	if found && verified && bfr {
		return true, nil
	}

	return false, nil
}

// CanGetBonus checks if user can get first order bonus
// Laravel: OrderPolicy::canGetBonus
// Rule: Returns true only when user has never logged a firstOrder record and asset is not 'irr'
func (p *orderPolicy) CanGetBonus(ctx context.Context, userID uint64, asset string) (bool, error) {
	if asset == "irr" {
		return false, nil
	}

	count, err := p.firstOrderRepo.Count(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check first order: %w", err)
	}

	return count == 0, nil
}
