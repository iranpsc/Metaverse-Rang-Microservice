package constants

const (
	Maskoni  = "m"
	Tejari   = "t"
	Amozeshi = "a"
)

// DefaultKarbariFilterCodes are used when no karbari query filter is provided.
var DefaultKarbariFilterCodes = []string{"a", "m", "t", "g", "s", "b", "e", "n"}

// KarbariCoefficients maps karbari types to their pricing coefficients.
var KarbariCoefficients = map[string]float64{
	Amozeshi: 0.3,
	Tejari:   0.2,
	Maskoni:  0.1,
}

// GetKarbariCoefficient returns the coefficient for a karbari.
func GetKarbariCoefficient(karbari string) float64 {
	if coef, ok := KarbariCoefficients[karbari]; ok {
		return coef
	}
	return 1.0
}
