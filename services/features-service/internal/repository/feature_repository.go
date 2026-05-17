package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"metargb/features-service/internal/models"
)

type FeatureRepository struct {
	db *sql.DB
}

func NewFeatureRepository(db *sql.DB) *FeatureRepository {
	return &FeatureRepository{db: db}
}

// FindByID retrieves a feature by ID with its properties
func (r *FeatureRepository) FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
	feature := &models.Feature{}
	properties := &models.FeatureProperties{}

	query := `
		SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at,
		       fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label,
		       fp.area, fp.density, fp.stability, fp.price_psc, fp.price_irr, fp.minimum_price_percentage,
		       fp.created_at as prop_created_at, fp.updated_at as prop_updated_at
		FROM features f
		LEFT JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.id = ?
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&feature.ID, &feature.OwnerID, &feature.MapID, &feature.Type,
		&feature.CreatedAt, &feature.UpdatedAt,
		&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
		&properties.Owner, &properties.Label, &properties.Area, &properties.Density,
		&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
		&properties.CreatedAt, &properties.UpdatedAt,
	)

	if err != nil {
		return nil, nil, err
	}

	return feature, properties, nil
}

// FindByBoundingBox implements Laravel's FeatureRepository@all logic
// Points format: ["minX,minY", "maxX,minY", "maxX,maxY", "minX,maxY"]
func (r *FeatureRepository) FindByBoundingBox(ctx context.Context, points []string, loadBuildings bool) ([]*models.Feature, error) {
	if len(points) != 4 {
		return nil, fmt.Errorf("expected 4 points, got %d", len(points))
	}

	// Parse points
	parsePoint := func(point string) (float64, float64, error) {
		parts := strings.Split(point, ",")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid point format: %s", point)
		}
		x, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, 0, err
		}
		y, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, 0, err
		}
		return x, y, nil
	}

	// Extract bounds
	minX, minY, err := parsePoint(points[0])
	if err != nil {
		return nil, err
	}
	maxX, maxY, err := parsePoint(points[2])
	if err != nil {
		return nil, err
	}

	// Query coordinates table for features within bounds
	// Matches Laravel: whereBetween('x', [minX, maxX])->whereBetween('y', [minY, maxY])
	query := `
		SELECT DISTINCT c.geometry_id
		FROM coordinates c
		WHERE c.x BETWEEN ? AND ?
		  AND c.y BETWEEN ? AND ?
	`

	rows, err := r.db.QueryContext(ctx, query, minX, maxX, minY, maxY)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	geometryIDs := []uint64{}
	for rows.Next() {
		var geoID uint64
		if err := rows.Scan(&geoID); err != nil {
			continue
		}
		geometryIDs = append(geometryIDs, geoID)
	}

	if len(geometryIDs) == 0 {
		return []*models.Feature{}, nil
	}

	// Convert to string for IN clause
	idStrs := make([]string, len(geometryIDs))
	for i, id := range geometryIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}

	// Load features with properties
	// Join with geometries table since geometries have feature_id (not features.geometry_id)
	featureQuery := `
		SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at,
		       fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label,
		       fp.area, fp.density, fp.stability, fp.price_psc, fp.price_irr, fp.minimum_price_percentage,
		       fp.created_at as prop_created_at, fp.updated_at as prop_updated_at
		FROM features f
		INNER JOIN geometries g ON g.feature_id = f.id
		LEFT JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE g.id IN (` + strings.Join(idStrs, ",") + `)
	`

	featureRows, err := r.db.QueryContext(ctx, featureQuery)
	if err != nil {
		return nil, err
	}
	defer featureRows.Close()

	features := []*models.Feature{}
	for featureRows.Next() {
		feature := &models.Feature{}
		properties := &models.FeatureProperties{}
		if err := featureRows.Scan(
			&feature.ID, &feature.OwnerID,
			&feature.MapID, &feature.Type, &feature.CreatedAt, &feature.UpdatedAt,
			&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
			&properties.Owner, &properties.Label, &properties.Area, &properties.Density,
			&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
			&properties.CreatedAt, &properties.UpdatedAt,
		); err != nil {
			continue
		}
		// Store properties reference (we'll need to handle this differently)
		features = append(features, feature)
	}

	return features, nil
}

// FindByBoundingBoxWithProperties returns features with their properties
func (r *FeatureRepository) FindByBoundingBoxWithProperties(ctx context.Context, points []string) ([]*models.Feature, []*models.FeatureProperties, error) {
	if len(points) != 4 {
		return nil, nil, fmt.Errorf("expected 4 points, got %d", len(points))
	}

	// Parse points
	parsePoint := func(point string) (float64, float64, error) {
		parts := strings.Split(point, ",")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid point format: %s", point)
		}
		x, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, 0, err
		}
		y, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, 0, err
		}
		return x, y, nil
	}

	// Extract bounds
	minX, minY, err := parsePoint(points[0])
	if err != nil {
		return nil, nil, err
	}
	maxX, maxY, err := parsePoint(points[2])
	if err != nil {
		return nil, nil, err
	}

	// Query coordinates table for features within bounds
	query := `
		SELECT DISTINCT c.geometry_id
		FROM coordinates c
		WHERE c.x BETWEEN ? AND ?
		  AND c.y BETWEEN ? AND ?
	`

	rows, err := r.db.QueryContext(ctx, query, minX, maxX, minY, maxY)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	geometryIDs := []uint64{}
	for rows.Next() {
		var geoID uint64
		if err := rows.Scan(&geoID); err != nil {
			continue
		}
		geometryIDs = append(geometryIDs, geoID)
	}

	if len(geometryIDs) == 0 {
		return []*models.Feature{}, []*models.FeatureProperties{}, nil
	}

	// Convert to string for IN clause
	idStrs := make([]string, len(geometryIDs))
	for i, id := range geometryIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}

	// Load features with properties
	// Join with geometries table since geometries have feature_id (not features.geometry_id)
	featureQuery := `
		SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at,
		       fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label,
		       fp.area, fp.density, fp.stability, fp.price_psc, fp.price_irr, fp.minimum_price_percentage,
		       fp.created_at as prop_created_at, fp.updated_at as prop_updated_at
		FROM features f
		INNER JOIN geometries g ON g.feature_id = f.id
		LEFT JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE g.id IN (` + strings.Join(idStrs, ",") + `)
	`

	featureRows, err := r.db.QueryContext(ctx, featureQuery)
	if err != nil {
		return nil, nil, err
	}
	defer featureRows.Close()

	features := []*models.Feature{}
	propertiesList := []*models.FeatureProperties{}
	for featureRows.Next() {
		feature := &models.Feature{}
		properties := &models.FeatureProperties{}
		if err := featureRows.Scan(
			&feature.ID, &feature.OwnerID,
			&feature.MapID, &feature.Type, &feature.CreatedAt, &feature.UpdatedAt,
			&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
			&properties.Owner, &properties.Label, &properties.Area, &properties.Density,
			&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
			&properties.CreatedAt, &properties.UpdatedAt,
		); err != nil {
			continue
		}
		features = append(features, feature)
		propertiesList = append(propertiesList, properties)
	}

	return features, propertiesList, nil
}

// FindByOwner retrieves all features owned by a user
func (r *FeatureRepository) FindByOwner(ctx context.Context, ownerID uint64) ([]*models.Feature, error) {
	query := `
		SELECT id, owner_id, map_id, type, created_at, updated_at
		FROM features
		WHERE owner_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	features := []*models.Feature{}
	for rows.Next() {
		feature := &models.Feature{}
		if err := rows.Scan(
			&feature.ID, &feature.OwnerID,
			&feature.MapID, &feature.Type, &feature.CreatedAt, &feature.UpdatedAt,
		); err != nil {
			continue
		}
		features = append(features, feature)
	}

	return features, nil
}

// UpdateOwner transfers ownership
func (r *FeatureRepository) UpdateOwner(ctx context.Context, featureID, newOwnerID uint64) error {
	query := "UPDATE features SET owner_id = ?, updated_at = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, newOwnerID, featureID)
	return err
}

// UpdateOwnerWithTx transfers ownership within a transaction
func (r *FeatureRepository) UpdateOwnerWithTx(ctx context.Context, tx *sql.Tx, featureID, newOwnerID uint64) error {
	query := "UPDATE features SET owner_id = ?, updated_at = NOW() WHERE id = ?"
	_, err := tx.ExecContext(ctx, query, newOwnerID, featureID)
	return err
}

// IsLocked checks if a feature is locked
func (r *FeatureRepository) IsLocked(ctx context.Context, featureID uint64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM locked_features
			WHERE feature_id = ? AND status = 0
		)
	`

	var locked bool
	err := r.db.QueryRowContext(ctx, query, featureID).Scan(&locked)
	return locked, err
}

// HasPendingBuyRequests checks if feature has pending buy requests
func (r *FeatureRepository) HasPendingBuyRequests(ctx context.Context, featureID uint64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM buy_feature_requests
			WHERE feature_id = ? AND deleted_at IS NULL
		)
	`

	var hasPending bool
	err := r.db.QueryRowContext(ctx, query, featureID).Scan(&hasPending)
	return hasPending, err
}

// FindByOwnerPaginated retrieves features owned by a user with pagination (5 per page)
// Returns features with their properties eager-loaded
// NOTE: features table does NOT have geometry_id column - geometries table has feature_id instead
func (r *FeatureRepository) FindByOwnerPaginated(ctx context.Context, ownerID uint64, page int) ([]*models.Feature, []*models.FeatureProperties, error) {
	if page < 1 {
		page = 1
	}
	perPage := 5
	offset := (page - 1) * perPage

	// Query does NOT select f.geometry_id because features table doesn't have that column
	query := `SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at, fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label, fp.area, fp.density, fp.stability, fp.price_psc, fp.price_irr, fp.minimum_price_percentage, fp.created_at as prop_created_at, fp.updated_at as prop_updated_at FROM features f LEFT JOIN feature_properties fp ON f.id = fp.feature_id WHERE f.owner_id = ? ORDER BY f.id ASC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, ownerID, perPage, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	features := []*models.Feature{}
	propertiesList := []*models.FeatureProperties{}
	for rows.Next() {
		feature := &models.Feature{}
		properties := &models.FeatureProperties{}
		if err := rows.Scan(
			&feature.ID, &feature.OwnerID,
			&feature.MapID, &feature.Type, &feature.CreatedAt, &feature.UpdatedAt,
			&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
			&properties.Owner, &properties.Label, &properties.Area, &properties.Density,
			&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
			&properties.CreatedAt, &properties.UpdatedAt,
		); err != nil {
			continue
		}
		features = append(features, feature)
		propertiesList = append(propertiesList, properties)
	}

	return features, propertiesList, nil
}

// FindByOwnerAndFeatureID retrieves a feature that belongs to a specific owner
// Used for scoped route bindings
func (r *FeatureRepository) FindByOwnerAndFeatureID(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
	feature := &models.Feature{}
	properties := &models.FeatureProperties{}

	query := `
		SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at,
		       fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label,
		       fp.area, fp.density, fp.stability, fp.price_psc, fp.price_irr, fp.minimum_price_percentage,
		       fp.created_at as prop_created_at, fp.updated_at as prop_updated_at
		FROM features f
		LEFT JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.id = ? AND f.owner_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, featureID, ownerID).Scan(
		&feature.ID, &feature.OwnerID, &feature.MapID, &feature.Type,
		&feature.CreatedAt, &feature.UpdatedAt,
		&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
		&properties.Owner, &properties.Label, &properties.Area, &properties.Density,
		&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
		&properties.CreatedAt, &properties.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil, nil // Not found
	}
	if err != nil {
		return nil, nil, err
	}

	return feature, properties, nil
}
