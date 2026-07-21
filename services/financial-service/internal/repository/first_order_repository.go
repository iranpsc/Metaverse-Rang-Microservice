// Package repository provides data access for the financial service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/financial-service/internal/models"
)

type FirstOrderRepository interface {
	Create(ctx context.Context, firstOrder *models.FirstOrder) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, firstOrder *models.FirstOrder) error
	Count(ctx context.Context, userID uint64) (int, error)
}

type firstOrderRepository struct {
	db *sql.DB
}

func NewFirstOrderRepository(db *sql.DB) FirstOrderRepository {
	return &firstOrderRepository{db: db}
}

func (r *firstOrderRepository) Create(ctx context.Context, firstOrder *models.FirstOrder) error {
	return r.create(ctx, r.db, firstOrder)
}

func (r *firstOrderRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, firstOrder *models.FirstOrder) error {
	return r.create(ctx, tx, firstOrder)
}

func (r *firstOrderRepository) create(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}, firstOrder *models.FirstOrder) error {
	query := `
		INSERT INTO first_orders (user_id, type, amount, date, bonus, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := exec.ExecContext(ctx, query,
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
