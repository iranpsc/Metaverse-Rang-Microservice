package models

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrFeatureNotFound = errors.New("feature not found")
	ErrNotFeatureOwner = errors.New("not the feature owner")
)

const (
	TradeHistoryTypeTrade   = "trade"
	TradeHistoryTypeGenesis = "genesis"

	TradeHistoryPriceCurrency = "currency"
	TradeHistoryPriceColor    = "color"

	SystemOwnerLabel    = "متارنگ سیستم"
	TradeHistoryPerPage = 10
)

// TradeHistoryTrade is a trade row with buyer info and linked color transactions.
type TradeHistoryTrade struct {
	ID           uint64
	FeatureID    uint64
	BuyerID      uint64
	SellerID     uint64
	Type         string
	IRRAmount    float64
	PSCAmount    float64
	Date         sql.NullTime
	CreatedAt    sql.NullTime
	BuyerCode    string
	BuyerName    string
	Transactions []TradeHistoryTransaction
}

// TradeHistoryTransaction is a morph-linked transaction used for color purchases.
type TradeHistoryTransaction struct {
	Asset  string
	Amount float64
	Action string
}

// TradeHistoryDateTime is the Shamsi date/time payload.
type TradeHistoryDateTime struct {
	Date      string
	MonthName string
	Year      int
	Time      string
	Formatted string
}

// TradeHistoryPrice is the price discriminator payload.
type TradeHistoryPrice struct {
	Type        string
	PricePSC    *int64
	PriceIRR    *int64
	Color       *string
	ColorName   *string
	ColorAmount *int64
}

// TradeHistoryItem is one ownership timeline entry.
type TradeHistoryItem struct {
	ID               *uint64
	Type             string
	ParticipantCode  *string
	ParticipantLabel string
	DateTime         TradeHistoryDateTime
	Price            TradeHistoryPrice
}

// TradeHistoryPage is a paginated trade history result.
type TradeHistoryPage struct {
	Items       []TradeHistoryItem
	CurrentPage int
	PerPage     int
	Total       int
	LastPage    int
	From        *int
	To          *int
	Path        string
}

// TradeTimestamp resolves the Gregorian timestamp for a trade (created_at, else date).
func (t TradeHistoryTrade) TradeTimestamp(fallback time.Time) time.Time {
	if t.CreatedAt.Valid {
		return t.CreatedAt.Time
	}
	if t.Date.Valid {
		d := t.Date.Time
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	}
	return fallback
}
