// Package constants holds shared domain constants for the financial service.
package constants

const (
	OrderPayableType   = "App\\Models\\Order"
	OptionPayableType  = "App\\Models\\Option"
	SadadGateway       = "sadad"
	OrderStatusPending = int32(-138)
	StatusSuccess      = int32(0)
	StatusUnknown      = int32(-1)

	TransactionActionDeposit = "deposit"
	FirstOrderBonusRate      = 0.5
	MinStoreCodes            = 2
	MinStoreCodeLength       = 2
)

var ValidOrderAssets = []string{"psc", "irr", "red", "blue", "yellow"}
