package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type VariableRepository interface {
	GetRate(ctx context.Context, asset string) (float64, error)
	GetAllRates(ctx context.Context) (map[string]float64, error)
}

type variableRepository struct {
	db *sql.DB
}

func NewVariableRepository(db *sql.DB) VariableRepository {
	return &variableRepository{db: db}
}

// GetRate retrieves the rate for a specific asset
// Laravel equivalent: Variable::getRate('psc')
func (r *variableRepository) GetRate(ctx context.Context, asset string) (float64, error) {
	query := `
		SELECT price
		FROM variables
		WHERE asset = ?
		LIMIT 1
	`

	var price float64
	err := r.db.QueryRowContext(ctx, query, asset).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("variable not found: %s", asset)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get variable rate: %w", err)
	}

	return price, nil
}

// GetAllRates retrieves all rates at once for efficiency
func (r *variableRepository) GetAllRates(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT asset, price
		FROM variables
		WHERE asset IN ('psc', 'red', 'blue', 'yellow')
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all rates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rates := make(map[string]float64)
	for rows.Next() {
		var asset string
		var price float64
		if err := rows.Scan(&asset, &price); err != nil {
			return nil, fmt.Errorf("failed to scan rate: %w", err)
		}
		rates[asset] = price
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return rates, nil
}
