package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"metargb/features-service/internal/models"
)

type GeometryRepository struct {
	db *sql.DB
}

func NewGeometryRepository(db *sql.DB) *GeometryRepository {
	return &GeometryRepository{db: db}
}

// GetByFeatureID retrieves geometry data for a feature
func (r *GeometryRepository) GetByFeatureID(ctx context.Context, featureID uint64) (*models.Geometry, error) {
	geometry := &models.Geometry{}

	query := `
		SELECT g.id, g.type, g.created_at, g.updated_at
		FROM geometries g
		WHERE g.feature_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, featureID).Scan(
		&geometry.ID, &geometry.Type, &geometry.CreatedAt, &geometry.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return geometry, nil
}

// GetCoordinatesByFeatureID retrieves coordinates for a feature as "x,y" strings
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
	defer rows.Close()

	coordinates := []string{}
	for rows.Next() {
		var x, y float64
		if err := rows.Scan(&x, &y); err != nil {
			continue
		}
		// Format as "x,y" string
		coordinates = append(coordinates, formatCoordinate(x, y))
	}

	return coordinates, nil
}

func formatCoordinate(x, y float64) string {
	return fmt.Sprintf("%.6f,%.6f", x, y)
}

func parseCoordString(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// GetCoordinatesWithIDs retrieves coordinates for a feature with IDs
func (r *GeometryRepository) GetCoordinatesWithIDs(ctx context.Context, featureID uint64) ([]*models.Coordinate, error) {
	query := `
		SELECT c.id, c.geometry_id, c.x, c.y
		FROM coordinates c
		INNER JOIN geometries g ON g.id = c.geometry_id
		WHERE g.feature_id = ?
		ORDER BY c.id
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coordinates := []*models.Coordinate{}
	for rows.Next() {
		coord := &models.Coordinate{}
		var x, y string
		if err := rows.Scan(&coord.ID, &coord.GeometryID, &x, &y); err != nil {
			continue
		}
		coord.X = parseCoordString(x)
		coord.Y = parseCoordString(y)
		coordinates = append(coordinates, coord)
	}

	return coordinates, nil
}
