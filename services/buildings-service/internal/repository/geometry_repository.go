package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type GeometryRepository struct {
	db *sql.DB
}

func NewGeometryRepository(db *sql.DB) *GeometryRepository {
	return &GeometryRepository{db: db}
}

// GetCoordinatesByFeatureID retrieves coordinates for a feature as "x,y" strings.
func (r *GeometryRepository) GetCoordinatesByFeatureID(ctx context.Context, featureID uint64) ([]string, error) {
	query := `
		SELECT c.x, c.y
		FROM coordinates c
		INNER JOIN geometries g ON g.id = c.geometry_id
		WHERE g.feature_id = ?
		ORDER BY c.id
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	coordinates := []string{}
	for rows.Next() {
		var x, y float64
		if err := rows.Scan(&x, &y); err != nil {
			return nil, fmt.Errorf("failed to scan coordinates: %w", err)
		}
		coordinates = append(coordinates, fmt.Sprintf("%.6f,%.6f", x, y))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating coordinates: %w", err)
	}

	return coordinates, nil
}
