// Package constants provides configuration and domain constants for the features service.
package constants

// RGB System Configuration Constants
// Matches config/rgb.php

const (
	// RGBFee is the marketplace fee (5%)
	// Buyer pays: price + (price * 0.05) = 105%
	// Seller receives: price - (price * 0.05) = 95%
	// Platform receives: (price * 0.05) * 2 = 10%
	RGBFee = 0.05

	// RGBUserCode is the system user code
	RGBUserCode = "hm-2000000"

	// DefaultPublicPricingLimit is the default minimum pricing percentage for regular users
	DefaultPublicPricingLimit = 80

	// DefaultUnder18PricingLimit is the default minimum pricing percentage for under-18 users
	DefaultUnder18PricingLimit = 110

	// HourlyProfitCalculationRate is the profit increment rate per 3 hours
	// Formula: stability * 0.000041666
	// This gives approximately 1% of stability per day (0.000041666 * 8 * 3 hours)
	HourlyProfitCalculationRate = 0.000041666

	// HourlyProfitCalculationInterval is the interval for profit calculation (3 hours)
	HourlyProfitCalculationIntervalHours = 3

	// UnderpricedLockDurationHours is the lock duration after selling below 100% (24 hours)
	UnderpricedLockDurationHours = 24
)

// CalculateBuyerCharge calculates the amount buyer pays (price + fee)
func CalculateBuyerCharge(price float64) float64 {
	return price + (price * RGBFee)
}

// CalculateSellerPayment calculates the amount seller receives (price - fee)
func CalculateSellerPayment(price float64) float64 {
	return price - (price * RGBFee)
}

// CalculatePlatformFee calculates the total fee for platform (fee * 2)
func CalculatePlatformFee(price float64) float64 {
	return (price * RGBFee) * 2
}

// CalculateFee calculates the fee amount for a given price
func CalculateFee(price float64) float64 {
	return price * RGBFee
}
