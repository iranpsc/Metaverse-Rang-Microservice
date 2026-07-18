// Package models defines domain types for the commercial service.
package models

// TransactionDTO represents the formatted transaction response
// Matches Laravel's TransactionResource exactly
type TransactionDTO struct {
	ID     string `json:"id"`     // VARCHAR PK like TR-xxxxx
	Type   string `json:"type"`   // polymorphic type
	Asset  string `json:"asset"`  // psc, irr, red, blue, yellow
	Amount string `json:"amount"` // formatted amount
	Action string `json:"action"` // deposit, withdraw
	Status int32  `json:"status"` // 0=pending, 1=success, etc.
	Date   string `json:"date"`   // Jalali format: Y/m/d
	Time   string `json:"time"`   // Jalali format: H:i:s
}
