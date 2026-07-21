package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/financial-service/internal/models"
)

type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	FindByID(ctx context.Context, id uint64) (*models.Order, error)
	FindByIDWithUser(ctx context.Context, id uint64) (*models.Order, *models.User, error)
	Update(ctx context.Context, order *models.Order) error
	UpdateWithTx(ctx context.Context, tx *sql.Tx, order *models.Order) error
	Delete(ctx context.Context, id uint64) error
}

type orderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	query := `
		INSERT INTO orders (user_id, asset, amount, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		order.UserID, order.Asset, order.Amount, order.Status, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	order.ID = uint64(id)

	return nil
}

func (r *orderRepository) FindByID(ctx context.Context, id uint64) (*models.Order, error) {
	query := `
		SELECT id, user_id, asset, amount, status, created_at, updated_at
		FROM orders
		WHERE id = ?
	`
	order := &models.Order{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID, &order.UserID, &order.Asset, &order.Amount,
		&order.Status, &order.CreatedAt, &order.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}
	return order, nil
}

func (r *orderRepository) FindByIDWithUser(ctx context.Context, id uint64) (*models.Order, *models.User, error) {
	query := `
		SELECT o.id, o.user_id, o.asset, o.amount, o.status, o.created_at, o.updated_at,
		       u.id, u.name, u.email, u.phone, k.birthdate
		FROM orders o
		INNER JOIN users u ON o.user_id = u.id
		LEFT JOIN kycs k ON u.id = k.user_id
		WHERE o.id = ?
	`
	order := &models.Order{}
	user := &models.User{}
	var phone sql.NullString
	var birthdate sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID, &order.UserID, &order.Asset, &order.Amount,
		&order.Status, &order.CreatedAt, &order.UpdatedAt,
		&user.ID, &user.Name, &user.Email, &phone, &birthdate,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find order with user: %w", err)
	}

	if phone.Valid {
		user.Phone = phone.String
	}
	if birthdate.Valid {
		user.Birthdate = &birthdate.Time
	}

	return order, user, nil
}

func (r *orderRepository) Update(ctx context.Context, order *models.Order) error {
	query := `
		UPDATE orders
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, order.Status, time.Now(), order.ID)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}
	return nil
}

func (r *orderRepository) UpdateWithTx(ctx context.Context, tx *sql.Tx, order *models.Order) error {
	query := `
		UPDATE orders
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.ExecContext(ctx, query, order.Status, time.Now(), order.ID)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}
	return nil
}

func (r *orderRepository) Delete(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM orders WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}
	return nil
}
