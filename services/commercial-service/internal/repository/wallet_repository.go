package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"metarang/commercial-service/internal/models"
)

type WalletRepository interface {
	FindByUserID(ctx context.Context, userID uint64) (*models.Wallet, error)
	Create(ctx context.Context, userID uint64) (*models.Wallet, error)
	Update(ctx context.Context, wallet *models.Wallet) error
	DeductBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error
	AddBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error
	LockBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal, reason string) error
	UnlockBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error
}

type walletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) FindByUserID(ctx context.Context, userID uint64) (*models.Wallet, error) {
	query := `
		SELECT id, user_id, psc, irr, red, blue, yellow, satisfaction, effect, created_at, updated_at
		FROM wallets
		WHERE user_id = ?
	`
	wallet := &models.Wallet{}

	var psc, irr, red, blue, yellow, satisfaction, effect string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&wallet.ID, &wallet.UserID, &psc, &irr, &red, &blue, &yellow,
		&satisfaction, &effect, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// Parse decimal values - handle empty strings as zero
	parseDecimal := func(s string) decimal.Decimal {
		if s == "" {
			return decimal.Zero
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return decimal.Zero
		}
		return d
	}

	wallet.PSC = parseDecimal(psc)
	wallet.IRR = parseDecimal(irr)
	wallet.Red = parseDecimal(red)
	wallet.Blue = parseDecimal(blue)
	wallet.Yellow = parseDecimal(yellow)
	wallet.Satisfaction = parseDecimal(satisfaction)
	wallet.Effect = parseDecimal(effect)

	return wallet, nil
}

// Create inserts a new wallet for the user with Laravel-compatible defaults.
// Idempotent: if a wallet already exists for the user, it is returned unchanged.
func (r *walletRepository) Create(ctx context.Context, userID uint64) (*models.Wallet, error) {
	existing, err := r.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now()
	query := `
		INSERT INTO wallets (user_id, psc, irr, red, blue, yellow, satisfaction, effect, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		userID,
		"0",
		"0",
		"0",
		"0",
		"0",
		"0.10",
		"0",
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get created wallet id: %w", err)
	}

	return &models.Wallet{
		ID:           uint64(id),
		UserID:       userID,
		PSC:          decimal.Zero,
		IRR:          decimal.Zero,
		Red:          decimal.Zero,
		Blue:         decimal.Zero,
		Yellow:       decimal.Zero,
		Satisfaction: decimal.NewFromFloat(0.10),
		Effect:       decimal.Zero,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *walletRepository) Update(ctx context.Context, wallet *models.Wallet) error {
	query := `
		UPDATE wallets
		SET psc = ?, irr = ?, red = ?, blue = ?, yellow = ?, satisfaction = ?, effect = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		wallet.PSC.String(), wallet.IRR.String(), wallet.Red.String(),
		wallet.Blue.String(), wallet.Yellow.String(), wallet.Satisfaction.String(),
		wallet.Effect.String(), time.Now(), wallet.ID)
	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}
	return nil
}

func (r *walletRepository) DeductBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error {
	query := fmt.Sprintf(`
		UPDATE wallets
		SET %s = %s - ?, updated_at = ?
		WHERE user_id = ? AND %s >= ?
	`, asset, asset, asset)

	result, err := r.db.ExecContext(ctx, query, amount.String(), time.Now(), userID, amount.String())
	if err != nil {
		return fmt.Errorf("failed to deduct balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("insufficient balance")
	}

	return nil
}

func (r *walletRepository) AddBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error {
	query := fmt.Sprintf(`
		UPDATE wallets
		SET %s = %s + ?, updated_at = ?
		WHERE user_id = ?
	`, asset, asset)

	_, err := r.db.ExecContext(ctx, query, amount.String(), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to add balance: %w", err)
	}

	return nil
}

func (r *walletRepository) LockBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal, reason string) error {
	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Deduct from wallet
	query := fmt.Sprintf(`
		UPDATE wallets
		SET %s = %s - ?, updated_at = ?
		WHERE user_id = ? AND %s >= ?
	`, asset, asset, asset)

	result, err := tx.ExecContext(ctx, query, amount.String(), time.Now(), userID, amount.String())
	if err != nil {
		return fmt.Errorf("failed to deduct for lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("insufficient balance to lock")
	}

	// Create locked asset record
	lockQuery := `
		INSERT INTO locked_assets (user_id, asset, amount, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, lockQuery, userID, asset, amount.String(), reason, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create locked asset: %w", err)
	}

	return tx.Commit()
}

func (r *walletRepository) UnlockBalance(ctx context.Context, userID uint64, asset string, amount decimal.Decimal) error {
	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Add back to wallet
	query := fmt.Sprintf(`
		UPDATE wallets
		SET %s = %s + ?, updated_at = ?
		WHERE user_id = ?
	`, asset, asset)

	_, err = tx.ExecContext(ctx, query, amount.String(), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to add unlocked balance: %w", err)
	}

	// Delete locked asset record
	unlockQuery := `
		DELETE FROM locked_assets
		WHERE user_id = ? AND asset = ? AND amount = ?
		LIMIT 1
	`
	_, err = tx.ExecContext(ctx, unlockQuery, userID, asset, amount.String())
	if err != nil {
		return fmt.Errorf("failed to delete locked asset: %w", err)
	}

	return tx.Commit()
}
