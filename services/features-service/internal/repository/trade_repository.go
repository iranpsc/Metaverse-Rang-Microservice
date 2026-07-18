package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/models"
)

type TradeRepository struct {
	db *sql.DB
}

func NewTradeRepository(db *sql.DB) *TradeRepository {
	return &TradeRepository{db: db}
}

// Create creates a new trade record
func (r *TradeRepository) Create(ctx context.Context, featureID, buyerID, sellerID uint64, irrAmount, pscAmount float64) (uint64, error) {
	query := `
		INSERT INTO trades (feature_id, buyer_id, seller_id, irr_amount, psc_amount, date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, featureID, buyerID, sellerID, irrAmount, pscAmount)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return uint64(id), err
}

// GetLatestForFeature gets the most recent trade for a feature
func (r *TradeRepository) GetLatestForFeature(ctx context.Context, featureID uint64) (*models.Trade, error) {
	trade := &models.Trade{}

	query := `
		SELECT id, feature_id, buyer_id, seller_id, irr_amount, psc_amount, date, created_at, updated_at
		FROM trades
		WHERE feature_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, featureID).Scan(
		&trade.ID, &trade.FeatureID, &trade.BuyerID, &trade.SellerID,
		&trade.IRRAmount, &trade.PSCAmount, &trade.Date,
		&trade.CreatedAt, &trade.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return trade, err
}

// GetLatestForFeatureWithSeller gets the most recent trade for a feature with seller information
func (r *TradeRepository) GetLatestForFeatureWithSeller(ctx context.Context, featureID uint64) (*models.Trade, *SellerInfo, error) {
	trade := &models.Trade{}
	seller := &SellerInfo{}

	query := `
		SELECT 
			t.id, t.feature_id, t.buyer_id, t.seller_id, t.irr_amount, t.psc_amount, t.date, t.created_at, t.updated_at,
			u.id as seller_user_id, u.name as seller_name, u.code as seller_code
		FROM trades t
		LEFT JOIN users u ON t.seller_id = u.id
		WHERE t.feature_id = ?
		ORDER BY t.created_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, featureID).Scan(
		&trade.ID, &trade.FeatureID, &trade.BuyerID, &trade.SellerID,
		&trade.IRRAmount, &trade.PSCAmount, &trade.Date,
		&trade.CreatedAt, &trade.UpdatedAt,
		&seller.ID, &seller.Name, &seller.Code,
	)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	}

	return trade, seller, err
}

// SellerInfo represents seller information from a trade
type SellerInfo struct {
	ID   uint64
	Name string
	Code string
}

// GetLatestUnderpricedForSeller gets the most recent underpriced trade for a seller.
func (r *TradeRepository) GetLatestUnderpricedForSeller(ctx context.Context, sellerID, featureID uint64) (*models.Trade, error) {
	trade := &models.Trade{}

	// Get latest trade where seller sold feature that was underpriced (< 100%)
	query := `
		SELECT t.id, t.feature_id, t.buyer_id, t.seller_id, t.irr_amount, t.psc_amount, t.date, t.created_at, t.updated_at
		FROM trades t
		INNER JOIN sell_feature_requests sfr ON t.feature_id = sfr.feature_id AND t.seller_id = sfr.seller_id
		WHERE t.seller_id = ? AND t.feature_id = ? AND sfr.limit < 100
		ORDER BY t.created_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, sellerID, featureID).Scan(
		&trade.ID, &trade.FeatureID, &trade.BuyerID, &trade.SellerID,
		&trade.IRRAmount, &trade.PSCAmount, &trade.Date,
		&trade.CreatedAt, &trade.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No underpriced trade found
	}

	return trade, err
}

// IsWithin24Hours checks if trade was created within last 24 hours
func (r *TradeRepository) IsWithin24Hours(trade *models.Trade) bool {
	if trade == nil {
		return false
	}
	return time.Since(trade.CreatedAt).Hours() < 24
}

// GetTimeRemaining returns remaining time until 24-hour lock expires
func (r *TradeRepository) GetTimeRemaining(trade *models.Trade) (hours int, minutes int) {
	if trade == nil {
		return 0, 0
	}

	lockExpiry := trade.CreatedAt.Add(24 * time.Hour)
	remaining := time.Until(lockExpiry)

	if remaining < 0 {
		return 0, 0
	}

	hours = int(remaining.Hours())
	minutes = int(remaining.Minutes()) % 60
	return hours, minutes
}

// FindSystemUserID returns the ID of the RGB system user (code = hm-2000000).
func (r *TradeRepository) FindSystemUserID(ctx context.Context) (uint64, error) {
	var id uint64
	err := r.db.QueryRowContext(ctx, "SELECT id FROM users WHERE code = ? LIMIT 1", constants.RGBUserCode).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("find system user: %w", err)
	}
	return id, nil
}

// ListByFeatureWithDetails loads trades for a feature with buyer info and color withdraw transactions.
// Results are ordered by created_at DESC, id DESC.
func (r *TradeRepository) ListByFeatureWithDetails(ctx context.Context, featureID uint64) ([]models.TradeHistoryTrade, error) {
	query := `
		SELECT
			t.id, t.feature_id, t.buyer_id, t.seller_id,
			COALESCE(t.irr_amount, 0), COALESCE(t.psc_amount, 0),
			t.date, t.created_at,
			COALESCE(buyer.code, ''), COALESCE(buyer.name, '')
		FROM trades t
		LEFT JOIN users buyer ON t.buyer_id = buyer.id
		WHERE t.feature_id = ?
		ORDER BY t.created_at DESC, t.id DESC
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, fmt.Errorf("list trades for feature %d: %w", featureID, err)
	}
	defer func() { _ = rows.Close() }()

	trades := make([]models.TradeHistoryTrade, 0)
	tradeIDs := make([]uint64, 0)
	tradeIndex := make(map[uint64]int)

	for rows.Next() {
		var trade models.TradeHistoryTrade
		var date, createdAt sql.NullTime
		if err := rows.Scan(
			&trade.ID, &trade.FeatureID, &trade.BuyerID, &trade.SellerID,
			&trade.IRRAmount, &trade.PSCAmount,
			&date, &createdAt,
			&trade.BuyerCode, &trade.BuyerName,
		); err != nil {
			return nil, fmt.Errorf("scan trade: %w", err)
		}
		trade.Date = date
		trade.CreatedAt = createdAt
		trade.Transactions = []models.TradeHistoryTransaction{}
		tradeIndex[trade.ID] = len(trades)
		trades = append(trades, trade)
		tradeIDs = append(tradeIDs, trade.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trades: %w", err)
	}

	if len(tradeIDs) == 0 {
		return trades, nil
	}

	txQuery := `
		SELECT payable_id, asset, amount, action
		FROM transactions
		WHERE payable_type = ?
		  AND payable_id IN (` + placeholders(len(tradeIDs)) + `)
		  AND action = 'withdraw'
		  AND asset IN ('red', 'blue', 'yellow')
	`
	args := make([]interface{}, 0, len(tradeIDs)+1)
	args = append(args, `App\Models\Trade`)
	for _, id := range tradeIDs {
		args = append(args, id)
	}

	txRows, err := r.db.QueryContext(ctx, txQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list trade transactions: %w", err)
	}
	defer func() { _ = txRows.Close() }()

	for txRows.Next() {
		var payableID uint64
		var tx models.TradeHistoryTransaction
		if err := txRows.Scan(&payableID, &tx.Asset, &tx.Amount, &tx.Action); err != nil {
			return nil, fmt.Errorf("scan trade transaction: %w", err)
		}
		if idx, ok := tradeIndex[payableID]; ok {
			trades[idx].Transactions = append(trades[idx].Transactions, tx)
		}
	}
	if err := txRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade transactions: %w", err)
	}

	return trades, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*2-1)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '?')
	}
	return string(b)
}
