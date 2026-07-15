package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type UserVariableRepository interface {
	GetReferralProfitLimit(ctx context.Context, userID uint64) (float64, error)
	GetWithdrawProfit(ctx context.Context, userID uint64) (int, error)
	Create(ctx context.Context, userID uint64) error
}

type userVariableRepository struct {
	db *sql.DB
}

func NewUserVariableRepository(db *sql.DB) UserVariableRepository {
	return &userVariableRepository{db: db}
}

// GetReferralProfitLimit gets the referral_profit limit for a user
// Laravel: $user->variables->referral_profit
func (r *userVariableRepository) GetReferralProfitLimit(ctx context.Context, userID uint64) (float64, error) {
	query := `
		SELECT referral_profit
		FROM user_variables
		WHERE user_id = ?
		LIMIT 1
	`

	var limit float64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&limit)
	if err == sql.ErrNoRows {
		// Default limit if not found
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get referral profit limit: %w", err)
	}

	return limit, nil
}

// GetWithdrawProfit gets the withdraw_profit (days) for a user
// Laravel: $user->variables->withdraw_profit
func (r *userVariableRepository) GetWithdrawProfit(ctx context.Context, userID uint64) (int, error) {
	query := `
		SELECT withdraw_profit
		FROM user_variables
		WHERE user_id = ?
		LIMIT 1
	`

	var days int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&days)
	if err == sql.ErrNoRows {
		// Default value
		return 7, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get withdraw profit: %w", err)
	}

	return days, nil
}

// Create inserts default user_variables for a newly registered user.
// Defaults match Laravel migration: referral_profit=15000000, data_storage=0, withdraw_profit=10.
// Idempotent if a row already exists.
func (r *userVariableRepository) Create(ctx context.Context, userID uint64) error {
	var existingID uint64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM user_variables WHERE user_id = ? LIMIT 1`, userID).Scan(&existingID)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing user variables: %w", err)
	}

	query := `
		INSERT INTO user_variables (user_id, referral_profit, data_storage, withdraw_profit, created_at, updated_at)
		VALUES (?, 15000000, 0, 10, NOW(), NOW())
	`
	_, err = r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to create user variables: %w", err)
	}
	return nil
}
