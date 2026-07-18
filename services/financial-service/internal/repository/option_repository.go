package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/financial-service/internal/models"
)

type OptionRepository interface {
	FindByCodes(ctx context.Context, codes []string) ([]*models.Option, error)
}

type optionRepository struct {
	db *sql.DB
}

func NewOptionRepository(db *sql.DB) OptionRepository {
	return &optionRepository{db: db}
}

func (r *optionRepository) FindByCodes(ctx context.Context, codes []string) ([]*models.Option, error) {
	if len(codes) == 0 {
		return []*models.Option{}, nil
	}

	// Build query with placeholders
	query := `
		SELECT id, code, asset, amount, note, created_at, updated_at
		FROM options
		WHERE code IN (`

	args := make([]interface{}, len(codes))
	for i, code := range codes {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = code
	}
	query += ")"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query options: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var options []*models.Option
	for rows.Next() {
		option := &models.Option{}
		var note sql.NullString
		err := rows.Scan(
			&option.ID, &option.Code, &option.Asset, &option.Amount,
			&note, &option.CreatedAt, &option.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan option: %w", err)
		}
		if note.Valid {
			option.Note = &note.String
		}
		options = append(options, option)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return options, nil
}
