package constants

// FeatureIndicators contains all feature status codes and types
// Ported from app/Helpers/FeatureIndicators.php

// Feature types (karbari)
const (
	Maskoni     = "m"
	Tejari      = "t"
	Amozeshi    = "a"
	FazaSabz    = "s"
	Farhangi    = "f"
	Parking     = "p"
	Mazhabi     = "z"
	Nemayeshgah = "n"
	Gardeshgari = "g"
	Edari       = "e"
	Behdashti   = "b"
)

// Maskoni (Residential/Yellow) status codes
const (
	MaskoniSoldAndPriced      = "a"
	MaskoniSoldAndNotPriced   = "b"
	MaskoniNotPriced          = "c"
	MaskoniPriced             = "d"
	MaskoniPreBought          = "e"
	MaskoniNotAllowedToBeSold = "f"
	MaskoniTradingLimited     = "g"
	MaskoniInConstruction     = "aa"
	MaskoniHasBuilding        = "bb"
	MaskoniHasDynasty         = "cc"
)

// Tejari (Commercial/Red) status codes
const (
	TejariSoldAndPriced      = "h"
	TejariSoldAndNotPriced   = "i"
	TejariNotPriced          = "j"
	TejariPriced             = "k"
	TejariPreBought          = "l"
	TejariNotAllowedToBeSold = "m"
	TejariTradingLimited     = "n"
	TejariInConstruction     = "hh"
	TejariHasBuilding        = "ii"
	TejariHasUnion           = "jj"
)

// Amozeshi (Educational/Blue) status codes
const (
	AmozeshiSoldAndPriced       = "o"
	AmozeshiSoldAndNotPriced    = "p"
	AmozeshiNotPriced           = "q"
	AmozeshiPriced              = "r"
	AmoozeshiPreBought          = "ss"
	AmoozeshiNotAllowedToBeSold = "tt"
	AmoozeshiTradingLimited     = "uu"
	AmoozeshiInConstruction     = "oo"
	AmoozeshiHasBuilding        = "pp"
)

// Color names
const (
	ColorMaskoni  = "yellow"
	ColorTejari   = "red"
	ColorAmozeshi = "blue"
)

// Persian color names
const (
	ColorMaskoniPersian  = "زرد"
	ColorTejariPersian   = "قرمز"
	ColorAmozeshiPersian = "آبی"
)

// Persian karbari titles
const (
	TitleMaskoni     = "مسکونی"
	TitleTejari      = "تجاری"
	TitleAmozeshi    = "آموزشی"
	TitleEdari       = "اداری"
	TitleBehdashti   = "بهداشتی"
	TitleFazaSabz    = "فضای سبز"
	TitleFarhangi    = "فرهنگی"
	TitleParking     = "پارکینگ"
	TitleMazhabi     = "مذهبی"
	TitleNemayeshgah = "نمایشگاه"
	TitleGardeshgari = "گردشگری"
)

// KarbariCoefficients maps karbari types to their pricing coefficients.
var KarbariCoefficients = map[string]float64{
	Amozeshi: 0.3,
	Tejari:   0.2,
	Maskoni:  0.1,
}

// GetColor returns the color asset (blue/red/yellow) based on karbari
func GetColor(karbari string) string {
	switch karbari {
	case Amozeshi:
		return ColorAmozeshi
	case Tejari:
		return ColorTejari
	case Maskoni:
		return ColorMaskoni
	default:
		return ""
	}
}

// GetColorPersian returns the Persian color name based on karbari
func GetColorPersian(karbari string) string {
	switch karbari {
	case Amozeshi:
		return ColorAmozeshiPersian
	case Tejari:
		return ColorTejariPersian
	case Maskoni:
		return ColorMaskoniPersian
	default:
		return ""
	}
}

// GetKarbariTitle returns the Persian title for karbari
func GetKarbariTitle(karbari string) string {
	switch karbari {
	case Amozeshi:
		return TitleAmozeshi
	case Tejari:
		return TitleTejari
	case Maskoni:
		return TitleMaskoni
	case Edari:
		return TitleEdari
	case Behdashti:
		return TitleBehdashti
	case FazaSabz:
		return TitleFazaSabz
	case Farhangi:
		return TitleFarhangi
	case Parking:
		return TitleParking
	case Mazhabi:
		return TitleMazhabi
	case Nemayeshgah:
		return TitleNemayeshgah
	case Gardeshgari:
		return TitleGardeshgari
	default:
		return ""
	}
}

// ChangeStatusToSoldAndPriced returns the sold-and-priced status based on karbari
func ChangeStatusToSoldAndPriced(karbari string) string {
	switch karbari {
	case Maskoni:
		return MaskoniSoldAndPriced
	case Tejari:
		return TejariSoldAndPriced
	case Amozeshi:
		return AmozeshiSoldAndPriced
	default:
		return ""
	}
}

// ChangeStatusToSoldAndNotPriced returns the sold-and-not-priced status based on karbari
func ChangeStatusToSoldAndNotPriced(karbari string) string {
	switch karbari {
	case Maskoni:
		return MaskoniSoldAndNotPriced
	case Tejari:
		return TejariSoldAndNotPriced
	case Amozeshi:
		return AmozeshiSoldAndNotPriced
	default:
		return ""
	}
}

// GetKarbariCoefficient returns the coefficient for a karbari
func GetKarbariCoefficient(karbari string) float64 {
	if coef, ok := KarbariCoefficients[karbari]; ok {
		return coef
	}
	return 1.0
}

// IsLimitedFeature checks if a feature has limited trading status
func IsLimitedFeature(rgb string) bool {
	return rgb == MaskoniTradingLimited ||
		rgb == TejariTradingLimited ||
		rgb == AmoozeshiTradingLimited
}

// IsNotAllowedToBeSold checks if a feature cannot be sold
func IsNotAllowedToBeSold(rgb string) bool {
	return rgb == MaskoniNotAllowedToBeSold ||
		rgb == TejariNotAllowedToBeSold ||
		rgb == AmoozeshiNotAllowedToBeSold
}

// IsSoldAndNotPriced checks if a feature is sold but not priced
func IsSoldAndNotPriced(rgb string) bool {
	return rgb == MaskoniSoldAndNotPriced ||
		rgb == TejariSoldAndNotPriced ||
		rgb == AmozeshiSoldAndNotPriced ||
		rgb == MaskoniNotPriced ||
		rgb == TejariNotPriced ||
		rgb == AmozeshiNotPriced
}
