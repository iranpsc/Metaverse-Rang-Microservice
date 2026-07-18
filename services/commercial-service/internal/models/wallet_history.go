package models

// AllWalletAssets names match Laravel WalletAsset::ALL.
var AllWalletAssets = []string{"psc", "irr", "red", "blue", "yellow", "satisfaction", "effect"}

// ColorAssets are color tokens that support withdraw spending.
var ColorAssets = []string{"red", "blue", "yellow"}

// WalletHistorySummaryCard is one asset card in the summary response.
type WalletHistorySummaryCard struct {
	Asset             string
	CurrentBalance    float64
	PeriodIncome      float64
	PeriodSpending    float64
	GrowthPercent     float64
	Direction         string
	PrivacyRestricted bool
}

// WalletChartPoint is a labeled amount for income or spending timelines.
type WalletChartPoint struct {
	Label  string
	Amount float64
}

// WalletAssetChart holds income and spending series for one asset.
type WalletAssetChart struct {
	Income   []WalletChartPoint
	Spending []WalletChartPoint
}

// WalletBalance holds current balances from the wallets table.
type WalletBalance struct {
	PSC          float64
	IRR          float64
	Red          float64
	Blue         float64
	Yellow       float64
	Satisfaction float64
	Effect       float64
}

// BalanceFor returns the balance for a named asset.
func (b *WalletBalance) BalanceFor(asset string) float64 {
	switch asset {
	case "psc":
		return b.PSC
	case "irr":
		return b.IRR
	case "red":
		return b.Red
	case "blue":
		return b.Blue
	case "yellow":
		return b.Yellow
	case "satisfaction":
		return b.Satisfaction
	case "effect":
		return b.Effect
	default:
		return 0
	}
}
