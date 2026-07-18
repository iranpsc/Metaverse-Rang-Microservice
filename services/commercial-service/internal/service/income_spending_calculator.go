// Package service implements business logic for the commercial service.
package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"metarang/commercial-service/internal/repository"
	"metarang/shared/pkg/period"
)

// BucketAmount is a labeled amount for chart series.
type BucketAmount struct {
	Label  string
	Amount float64
}

// IncomeCalculator aggregates period income for wallet assets.
type IncomeCalculator struct {
	repo repository.WalletHistoryRepository
}

func NewIncomeCalculator(repo repository.WalletHistoryRepository) *IncomeCalculator {
	return &IncomeCalculator{repo: repo}
}

func (c *IncomeCalculator) CalcIncome(ctx context.Context, userID uint64, asset string, start, end time.Time) (float64, error) {
	deposit, err := c.repo.SumDeposits(ctx, userID, asset, start, end)
	if err != nil {
		return 0, fmt.Errorf("deposit income: %w", err)
	}
	hourly, err := c.repo.SumHourlyProfits(ctx, userID, asset, start, end)
	if err != nil {
		return 0, fmt.Errorf("hourly profit income: %w", err)
	}
	tradeSell, err := c.repo.SumTradeSells(ctx, userID, asset, start, end)
	if err != nil {
		return 0, fmt.Errorf("trade sell income: %w", err)
	}
	referral := 0.0
	if asset == "psc" {
		referral, err = c.repo.SumReferralBonuses(ctx, userID, start, end)
		if err != nil {
			return 0, fmt.Errorf("referral income: %w", err)
		}
	}
	firstOrder, err := c.repo.SumFirstOrderBonuses(ctx, userID, asset, start, end)
	if err != nil {
		return 0, fmt.Errorf("first order income: %w", err)
	}
	level, err := c.repo.SumLevelRewards(ctx, userID, asset, start, end)
	if err != nil {
		return 0, fmt.Errorf("level reward income: %w", err)
	}
	return deposit + hourly + tradeSell + referral + firstOrder + level, nil
}

func (c *IncomeCalculator) CalcIncomeBuckets(ctx context.Context, userID uint64, asset string, buckets []period.PeriodBucket) ([]BucketAmount, error) {
	out := make([]BucketAmount, 0, len(buckets))
	for _, bucket := range buckets {
		amount, err := c.CalcIncome(ctx, userID, asset, bucket.Start, bucket.End)
		if err != nil {
			return nil, err
		}
		out = append(out, BucketAmount{Label: bucket.Label, Amount: round2(amount)})
	}
	return out, nil
}

// SpendingCalculator aggregates period spending for wallet assets.
type SpendingCalculator struct {
	repo repository.WalletHistoryRepository
}

func NewSpendingCalculator(repo repository.WalletHistoryRepository) *SpendingCalculator {
	return &SpendingCalculator{repo: repo}
}

func (c *SpendingCalculator) CalcSpending(ctx context.Context, userID uint64, asset string, start, end time.Time) (float64, error) {
	tradeBuy := 0.0
	var err error
	if asset == "psc" || asset == "irr" {
		tradeBuy, err = c.repo.SumTradeBuys(ctx, userID, asset, start, end)
		if err != nil {
			return 0, fmt.Errorf("trade buy spending: %w", err)
		}
	}
	featurePurchase := 0.0
	if asset == "psc" || asset == "irr" {
		featurePurchase, err = c.repo.SumFeaturePurchaseWithdrawals(ctx, userID, asset, start, end)
		if err != nil {
			return 0, fmt.Errorf("feature purchase spending: %w", err)
		}
	}
	colorWithdraw := 0.0
	if isColorAsset(asset) {
		colorWithdraw, err = c.repo.SumWithdrawals(ctx, userID, asset, start, end)
		if err != nil {
			return 0, fmt.Errorf("color withdraw spending: %w", err)
		}
	}
	building := 0.0
	if asset == "satisfaction" {
		building, err = c.repo.SumBuildingSatisfaction(ctx, userID, start, end)
		if err != nil {
			return 0, fmt.Errorf("building satisfaction spending: %w", err)
		}
	}
	return tradeBuy + featurePurchase + colorWithdraw + building, nil
}

func (c *SpendingCalculator) CalcSpendingBuckets(ctx context.Context, userID uint64, asset string, buckets []period.PeriodBucket) ([]BucketAmount, error) {
	out := make([]BucketAmount, 0, len(buckets))
	for _, bucket := range buckets {
		amount, err := c.CalcSpending(ctx, userID, asset, bucket.Start, bucket.End)
		if err != nil {
			return nil, err
		}
		out = append(out, BucketAmount{Label: bucket.Label, Amount: round2(amount)})
	}
	return out, nil
}

func isColorAsset(asset string) bool {
	return asset == "red" || asset == "blue" || asset == "yellow"
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
