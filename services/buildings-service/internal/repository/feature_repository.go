package repository

import (
	"context"
	"database/sql"

	"metarang/buildings-service/internal/models"
)

type FeatureRepository struct {
	db *sql.DB
}

func NewFeatureRepository(db *sql.DB) *FeatureRepository {
	return &FeatureRepository{db: db}
}

// FindByID retrieves a feature by ID with its properties.
func (r *FeatureRepository) FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
	feature := &models.Feature{}
	properties := &models.FeatureProperties{}

	query := `
		SELECT f.id, f.owner_id, f.map_id, f.type, f.created_at, f.updated_at,
		       fp.id as prop_id, fp.feature_id, fp.karbari, fp.rgb, fp.owner, fp.label, fp.address,
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
		&properties.Owner, &properties.Label, &properties.Address, &properties.Area, &properties.Density,
		&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
		&properties.CreatedAt, &properties.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	return feature, properties, nil
}
