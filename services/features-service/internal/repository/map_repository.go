package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/features-service/internal/models"
)

type MapRepository struct {
	db *sql.DB
}

func NewMapRepository(db *sql.DB) *MapRepository {
	return &MapRepository{db: db}
}

// FindAll retrieves all maps from the database
func (r *MapRepository) FindAll(ctx context.Context) ([]*models.Map, error) {
	query := `
		SELECT id, name, karbari, publish_date, publisher_name, polygon_count,
		       total_area, first_id, last_id, status, fileName,
		       central_point_coordinates, border_coordinates, polygon_area,
		       polygon_address, polygon_color
		FROM maps
		ORDER BY id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query maps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	maps := []*models.Map{}
	for rows.Next() {
		m := &models.Map{}
		err := rows.Scan(
			&m.ID, &m.Name, &m.Karbari, &m.PublishDate, &m.PublisherName,
			&m.PolygonCount, &m.TotalArea, &m.FirstID, &m.LastID, &m.Status,
			&m.FileName, &m.CentralPointCoordinates, &m.BorderCoordinates,
			&m.PolygonArea, &m.PolygonAddress, &m.PolygonColor,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan map: %w", err)
		}
		maps = append(maps, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating maps: %w", err)
	}

	return maps, nil
}

// FindByID retrieves a single map by ID
func (r *MapRepository) FindByID(ctx context.Context, id uint64) (*models.Map, error) {
	query := `
		SELECT id, name, karbari, publish_date, publisher_name, polygon_count,
		       total_area, first_id, last_id, status, fileName,
		       central_point_coordinates, border_coordinates, polygon_area,
		       polygon_address, polygon_color
		FROM maps
		WHERE id = ?
	`

	m := &models.Map{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.Name, &m.Karbari, &m.PublishDate, &m.PublisherName,
		&m.PolygonCount, &m.TotalArea, &m.FirstID, &m.LastID, &m.Status,
		&m.FileName, &m.CentralPointCoordinates, &m.BorderCoordinates,
		&m.PolygonArea, &m.PolygonAddress, &m.PolygonColor,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find map: %w", err)
	}

	return m, nil
}

// FindFeaturesByMapID retrieves all features for a map with their owner_id and karbari
func (r *MapRepository) FindFeaturesByMapID(ctx context.Context, mapID uint64) ([]*models.MapFeature, error) {
	query := `
		SELECT f.id, f.owner_id, COALESCE(fp.karbari, '') as karbari
		FROM features f
		LEFT JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.map_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, mapID)
	if err != nil {
		return nil, fmt.Errorf("failed to query features: %w", err)
	}
	defer func() { _ = rows.Close() }()

	features := []*models.MapFeature{}
	for rows.Next() {
		f := &models.MapFeature{}
		err := rows.Scan(&f.ID, &f.OwnerID, &f.Karbari)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feature: %w", err)
		}
		features = append(features, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating features: %w", err)
	}

	return features, nil
}
