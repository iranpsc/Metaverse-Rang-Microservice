// Package constants holds shared domain constants for the financial service.
package constants

const (
	OrderPayableType   = "App\\Models\\Order"
	OptionPayableType  = "App\\Models\\Option"
	SadadGateway       = "sadad"
	OrderStatusPending = int32(-138)
	StatusSuccess      = int32(0)
	StatusUnknown      = int32(-1)
	// TransactionStatusPending is the deposit row written before the bank callback.
	// It is not OrderStatusPending; a successful callback replaces it with StatusSuccess.
	TransactionStatusPending = int32(1)

	TransactionActionDeposit = "deposit"
	FirstOrderBonusRate      = 0.5
	MinOrderAmount           = int32(1)
	MinStoreCodes            = 2
	MinStoreCodeLength       = 2

	AssetIRR = "irr"
)

var ValidOrderAssets = []string{"psc", AssetIRR, "red", "blue", "yellow"}
