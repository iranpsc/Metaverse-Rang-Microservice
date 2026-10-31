package service

import (
	"context"
	"fmt"
	"math"
)

// AssetRatePort returns rials per asset unit from commercial-service.
type AssetRatePort interface {
	GetPriceByAsset(context.Context, string) (float64, error)
}

func PSCRate(ctx context.Context, rates AssetRatePort) (float64, error) {
	if rates == nil {
		return 0, fmt.Errorf("psc rate client not configured")
	}
	rate, err := rates.GetPriceByAsset(ctx, "psc")
	if err != nil {
		return 0, fmt.Errorf("psc rate: %w", err)
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, fmt.Errorf("psc rate must be positive and finite")
	}
	return rate, nil
}

// RialsToPSC rejects malformed amounts and overflow before JSON serialization
// or a wallet credit. A positive rate alone does not guarantee a finite result.
func RialsToPSC(rials int64, rate float64) (float64, error) {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, fmt.Errorf("psc rate must be positive and finite")
	}
	if rials < 0 {
		return 0, fmt.Errorf("prize rial amount must not be negative")
	}
	amount := float64(rials) / rate
	if math.IsInf(amount, 0) || (rials > 0 && amount == 0) {
		return 0, fmt.Errorf("converted PSC prize is outside the supported range")
	}
	return amount, nil
}
