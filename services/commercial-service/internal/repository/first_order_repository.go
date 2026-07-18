// Package repository provides data access for the commercial service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/commercial-service/internal/models"
)

type FirstOrderRepository interface {
	Create(ctx context.Context, firstOrder *models.FirstOrder) error
	HasFirstOrder(ctx context.Context, userID uint64, orderType string) (bool, error)
	Count(ctx context.Context, userID uint64) (int, error)
}

type firstOrderRepository struct {
	db *sql.DB
}

func NewFirstOrderRepository(db *sql.DB) FirstOrderRepository {
	return &firstOrderRepository{db: db}
}

// Create creates a first order record
// Laravel: $user->firstOrder()->create([...])
func (r *firstOrderRepository) Create(ctx context.Context, firstOrder *models.FirstOrder) error {
	query := `
		INSERT INTO first_orders (user_id, type, amount, date, bonus, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query,
		firstOrder.UserID,
		firstOrder.Type,
		firstOrder.Amount,
		firstOrder.Date,
		firstOrder.Bonus,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create first order: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		firstOrder.ID = uint64(id)
	}

	return nil
}

// HasFirstOrder checks if user has already gotten first order bonus for this type
func (r *firstOrderRepository) HasFirstOrder(ctx context.Context, userID uint64, orderType string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM first_orders
		WHERE user_id = ? AND type = ?
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, orderType).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check first order: %w", err)
	}

	return exists, nil
}

// Count returns the total number of first orders for a user
func (r *firstOrderRepository) Count(ctx context.Context, userID uint64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM first_orders
		WHERE user_id = ?
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count first orders: %w", err)
	}

	return count, nil
}
