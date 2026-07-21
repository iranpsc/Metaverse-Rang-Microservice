package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/financial-service/internal/models"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *models.Payment) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, payment *models.Payment) error
}

type paymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	return r.create(ctx, r.db, payment)
}

func (r *paymentRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, payment *models.Payment) error {
	return r.create(ctx, tx, payment)
}

func (r *paymentRepository) create(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}, payment *models.Payment) error {
	query := `
		INSERT INTO payments (user_id, ref_id, card_pan, gateway, amount, product, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := exec.ExecContext(ctx, query,
		payment.UserID, payment.RefID, payment.CardPan, payment.Gateway,
		payment.Amount, payment.Product, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	payment.ID = uint64(id)

	return nil
}
