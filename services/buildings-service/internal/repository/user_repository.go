package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserRepository reads lightweight user metadata.
type UserRepository interface {
	GetUserCreatedAt(ctx context.Context, userID uint64) (time.Time, error)
}

type userRepository struct {
	db *sql.DB
}

// NewUserRepository creates a MySQL-backed user metadata repository.
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetUserCreatedAt(ctx context.Context, userID uint64) (time.Time, error) {
	query := `SELECT created_at FROM users WHERE id = ? LIMIT 1`
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&createdAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get user created_at: %w", err)
	}
	return createdAt, nil
}
