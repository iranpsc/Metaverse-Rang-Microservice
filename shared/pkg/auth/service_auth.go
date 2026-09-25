package auth

import (
	"crypto/subtle"
	"os"
)

// ServiceTokenMetadataKey is the gRPC metadata key for inter-service authentication.
const ServiceTokenMetadataKey = "x-service-token"

// InternalServiceMethods require a valid service token (not public user auth bypass).
var InternalServiceMethods = map[string]struct{}{
	"/commercial.WalletService/CreateWallet":              {},
	"/commercial.WalletService/AddBalance":                {},
	"/commercial.WalletService/DeductBalance":             {},
	"/commercial.WalletService/LockBalance":               {},
	"/commercial.WalletService/UnlockBalance":             {},
	"/commercial.UserVariableService/CreateUserVariables": {},
	"/commercial.TransactionService/CreateTransaction":    {},
	"/financial.WalletService/AddBalance":                 {},
	"/financial.WalletService/DeductBalance":              {},
	"/financial.WalletService/LockBalance":                {},
	"/financial.WalletService/UnlockBalance":              {},
	// Citizen building stats trust caller-supplied user_id and karbari filters.
	// The public HTTP handler applies privacy in-process. Network callers must
	// present the service token.
	"/features.CitizenBuildingsService/GetCitizenBuildingSummary": {},
	"/features.CitizenBuildingsService/GetCitizenBuildingChart":   {},
	"/features.CitizenBuildingsService/ListCitizenBuildings":      {},
}

// RequiresServiceAuth reports whether a gRPC method must be called with a service token.
func RequiresServiceAuth(fullMethod string) bool {
	_, ok := InternalServiceMethods[fullMethod]
	return ok
}

// ServiceSecretFromEnv returns the shared secret used for service-to-service calls.
func ServiceSecretFromEnv() string {
	return os.Getenv("INTERNAL_SERVICE_SECRET")
}

// ValidateServiceToken compares the provided token with the configured service secret.
func ValidateServiceToken(token string) bool {
	secret := ServiceSecretFromEnv()
	if secret == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}
