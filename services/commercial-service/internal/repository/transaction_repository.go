package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"metarang/commercial-service/internal/models"
	"metarang/shared/pkg/helpers"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *models.Transaction) error
	Update(ctx context.Context, transaction *models.Transaction) error
	FindByID(ctx context.Context, id string) (*models.Transaction, error)
	FindLatestByUserID(ctx context.Context, userID uint64) (*models.Transaction, error)
	FindByUserID(ctx context.Context, userID uint64, filters map[string]interface{}) ([]*models.Transaction, error)
}

type transactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, transaction *models.Transaction) error {
	query := `
		INSERT INTO transactions (id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		transaction.ID, transaction.UserID, transaction.Asset, transaction.Amount,
		transaction.Action, transaction.Status, transaction.Token, transaction.RefID,
		transaction.PayableType, transaction.PayableID, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

func (r *transactionRepository) Update(ctx context.Context, transaction *models.Transaction) error {
	query := `
		UPDATE transactions
		SET user_id = ?, asset = ?, amount = ?, action = ?, status = ?, token = ?, ref_id = ?, payable_type = ?, payable_id = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		transaction.UserID,
		transaction.Asset,
		transaction.Amount,
		transaction.Action,
		transaction.Status,
		transaction.Token,
		transaction.RefID,
		transaction.PayableType,
		transaction.PayableID,
		time.Now(),
		transaction.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

func (r *transactionRepository) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	query := `
		SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at
		FROM transactions
		WHERE id = ?
	`
	transaction := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&transaction.ID, &transaction.UserID, &transaction.Asset, &transaction.Amount,
		&transaction.Action, &transaction.Status, &transaction.Token, &transaction.RefID,
		&transaction.PayableType, &transaction.PayableID,
		&transaction.CreatedAt, &transaction.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	return transaction, nil
}

func (r *transactionRepository) FindLatestByUserID(ctx context.Context, userID uint64) (*models.Transaction, error) {
	query := `
		SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at
		FROM transactions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	transaction := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&transaction.ID, &transaction.UserID, &transaction.Asset, &transaction.Amount,
		&transaction.Action, &transaction.Status, &transaction.Token, &transaction.RefID,
		&transaction.PayableType, &transaction.PayableID,
		&transaction.CreatedAt, &transaction.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find latest transaction: %w", err)
	}
	return transaction, nil
}

func (r *transactionRepository) FindByUserID(ctx context.Context, userID uint64, filters map[string]interface{}) ([]*models.Transaction, error) {
	query := `
		SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at
		FROM transactions
		WHERE user_id = ?
	`
	args := []interface{}{userID}

	if search, ok := filters["search"].(string); ok && search != "" {
		query += " AND id = ?"
		args = append(args, search)
	}

	if startDateTime, ok := filters["start_date_time"].(string); ok && startDateTime != "" {
		if start, err := helpers.ParseJalaliDateTime(startDateTime); err == nil {
			query += " AND DATE(created_at) >= DATE(?)"
			args = append(args, start)
		}
	}

	if endDateTime, ok := filters["end_date_time"].(string); ok && endDateTime != "" {
		if end, err := helpers.ParseJalaliDateTime(endDateTime); err == nil {
			query += " AND DATE(created_at) <= DATE(?)"
			args = append(args, end)
		}
	}

	if statuses, ok := filters["status"].([]int32); ok && len(statuses) > 0 {
		placeholders := strings.Repeat("?,", len(statuses))
		query += " AND status IN (" + placeholders[:len(placeholders)-1] + ")"
		for _, status := range statuses {
			args = append(args, status)
		}
	}

	if action, ok := filters["action"].(string); ok && action != "" {
		query += " AND action = ?"
		args = append(args, action)
	}

	if asset, ok := filters["asset"].(string); ok && asset != "" {
		assets := splitCSV(asset)
		if len(assets) == 1 {
			query += " AND asset = ?"
			args = append(args, assets[0])
		} else if len(assets) > 1 {
			placeholders := strings.Repeat("?,", len(assets))
			query += " AND asset IN (" + placeholders[:len(placeholders)-1] + ")"
			for _, value := range assets {
				args = append(args, value)
			}
		}
	}

	if txType, ok := filters["type"].(string); ok && txType != "" {
		types := splitCSV(txType)
		payableTypes := make([]string, 0, len(types))
		for _, value := range types {
			if payableType := transactionTypeToPayableType(value); payableType != "" {
				payableTypes = append(payableTypes, payableType)
			}
		}
		if len(payableTypes) == 1 {
			query += " AND payable_type = ?"
			args = append(args, payableTypes[0])
		} else if len(payableTypes) > 1 {
			placeholders := strings.Repeat("?,", len(payableTypes))
			query += " AND payable_type IN (" + placeholders[:len(placeholders)-1] + ")"
			for _, value := range payableTypes {
				args = append(args, value)
			}
		}
	}

	query += " ORDER BY created_at DESC"

	perPage := 15
	if value, ok := filters["per_page"].(int); ok && value > 0 {
		perPage = value
	}

	page := 1
	if value, ok := filters["page"].(int); ok && value > 0 {
		page = value
	}

	offset := (page - 1) * perPage
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", perPage+1, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transactions []*models.Transaction
	for rows.Next() {
		transaction := &models.Transaction{}
		err := rows.Scan(
			&transaction.ID, &transaction.UserID, &transaction.Asset, &transaction.Amount,
			&transaction.Action, &transaction.Status, &transaction.Token, &transaction.RefID,
			&transaction.PayableType, &transaction.PayableID,
			&transaction.CreatedAt, &transaction.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func transactionTypeToPayableType(txType string) string {
	switch strings.ToLower(strings.TrimSpace(txType)) {
	case "trade":
		return `App\Models\Trade`
	case "order":
		return `App\Models\Order`
	default:
		return ""
	}
}
