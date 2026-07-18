package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"metarang/features-service/internal/models"
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
	defer func() { _ = rows.Close() }()

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

func uint64IDsToCSV(ids []uint64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ",")
}

// GetByFeatureIDs loads geometries keyed by feature_id.
func (r *GeometryRepository) GetByFeatureIDs(ctx context.Context, featureIDs []uint64) (map[uint64]*models.Geometry, error) {
	result := make(map[uint64]*models.Geometry, len(featureIDs))
	if len(featureIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT g.id, g.feature_id, g.type, g.created_at, g.updated_at
		FROM geometries g
		WHERE g.feature_id IN (` + uint64IDsToCSV(featureIDs) + `)
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		geometry := &models.Geometry{}
		if err := rows.Scan(
			&geometry.ID, &geometry.FeatureID, &geometry.Type, &geometry.CreatedAt, &geometry.UpdatedAt,
		); err != nil {
			continue
		}
		result[geometry.FeatureID] = geometry
	}

	return result, nil
}

// GetCoordinatesByFeatureIDs loads coordinates keyed by feature_id.
func (r *GeometryRepository) GetCoordinatesByFeatureIDs(ctx context.Context, featureIDs []uint64) (map[uint64][]*models.Coordinate, error) {
	result := make(map[uint64][]*models.Coordinate, len(featureIDs))
	if len(featureIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT g.feature_id, c.id, c.geometry_id, c.x, c.y
		FROM coordinates c
		INNER JOIN geometries g ON g.id = c.geometry_id
		WHERE g.feature_id IN (` + uint64IDsToCSV(featureIDs) + `)
		ORDER BY g.feature_id, c.id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		coord := &models.Coordinate{}
		var featureID uint64
		var x, y string
		if err := rows.Scan(&featureID, &coord.ID, &coord.GeometryID, &x, &y); err != nil {
			continue
		}
		coord.X = parseCoordString(x)
		coord.Y = parseCoordString(y)
		result[featureID] = append(result[featureID], coord)
	}

	return result, nil
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
	defer func() { _ = rows.Close() }()

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
