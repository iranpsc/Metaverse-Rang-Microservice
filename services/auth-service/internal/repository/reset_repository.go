package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/auth-service/internal/models"
)

type ResetRepository interface {
	Create(ctx context.Context, reset *models.Reset) error
	MarkVerified(ctx context.Context, id uint64) error
	CountVerifiedByUserAndType(ctx context.Context, userID uint64, resetType string) (int, error)
	Delete(ctx context.Context, id uint64) error
}

type resetRepository struct {
	db *sql.DB
}

func NewResetRepository(db *sql.DB) ResetRepository {
	return &resetRepository{db: db}
}

func (r *resetRepository) Create(ctx context.Context, reset *models.Reset) error {
	if reset == nil {
		return fmt.Errorf("reset is required")
	}

	now := time.Now()
	if reset.CreatedAt.IsZero() {
		reset.CreatedAt = now
	}
	if reset.UpdatedAt.IsZero() {
		reset.UpdatedAt = now
	}

	query := `
		INSERT INTO resets (user_id, type, value, verified, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		reset.UserID,
		reset.Type,
		reset.Value,
		reset.Verified,
		reset.CreatedAt,
		reset.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create reset: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	reset.ID = uint64(id)
	return nil
}

func (r *resetRepository) MarkVerified(ctx context.Context, id uint64) error {
	query := `UPDATE resets SET verified = 1, updated_at = ? WHERE id = ?`
	if _, err := r.db.ExecContext(ctx, query, time.Now(), id); err != nil {
		return fmt.Errorf("failed to mark reset verified: %w", err)
	}
	return nil
}

func (r *resetRepository) CountVerifiedByUserAndType(ctx context.Context, userID uint64, resetType string) (int, error) {
	query := `
		SELECT COUNT(*) FROM resets
		WHERE user_id = ? AND type = ? AND verified = 1
	`
	var count int
	if err := r.db.QueryRowContext(ctx, query, userID, resetType).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count verified resets: %w", err)
	}
	return count, nil
}

func (r *resetRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM resets WHERE id = ?`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to delete reset: %w", err)
	}
	return nil
}
