package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/financial-service/internal/models"
)

type TransactionRepository interface {
	Create(ctx context.Context, transaction *models.Transaction) error
	Update(ctx context.Context, transaction *models.Transaction) error
	UpdateWithTx(ctx context.Context, tx *sql.Tx, transaction *models.Transaction) error
	FindByID(ctx context.Context, id string) (*models.Transaction, error)
	FindByPayable(ctx context.Context, payableType string, payableID uint64) (*models.Transaction, error)
	Delete(ctx context.Context, id string) error
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
	return r.update(ctx, r.db, transaction)
}

func (r *transactionRepository) UpdateWithTx(ctx context.Context, tx *sql.Tx, transaction *models.Transaction) error {
	return r.update(ctx, tx, transaction)
}

func (r *transactionRepository) update(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}, transaction *models.Transaction) error {
	query := `
		UPDATE transactions
		SET status = ?, ref_id = ?, token = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := exec.ExecContext(ctx, query,
		transaction.Status,
		transaction.RefID,
		transaction.Token,
		time.Now(),
		transaction.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

func (r *transactionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
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

func (r *transactionRepository) FindByPayable(ctx context.Context, payableType string, payableID uint64) (*models.Transaction, error) {
	query := `
		SELECT id, user_id, asset, amount, action, status, token, ref_id, payable_type, payable_id, created_at, updated_at
		FROM transactions
		WHERE payable_type = ? AND payable_id = ?
		LIMIT 1
	`
	transaction := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, query, payableType, payableID).Scan(
		&transaction.ID, &transaction.UserID, &transaction.Asset, &transaction.Amount,
		&transaction.Action, &transaction.Status, &transaction.Token, &transaction.RefID,
		&transaction.PayableType, &transaction.PayableID,
		&transaction.CreatedAt, &transaction.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction by payable: %w", err)
	}
	return transaction, nil
}
