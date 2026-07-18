package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/features-service/internal/models"
)

type PropertiesRepository struct {
	db *sql.DB
}

func NewPropertiesRepository(db *sql.DB) *PropertiesRepository {
	return &PropertiesRepository{db: db}
}

// GetByFeatureID retrieves properties for a feature
func (r *PropertiesRepository) GetByFeatureID(ctx context.Context, featureID uint64) (*models.FeatureProperties, error) {
	properties := &models.FeatureProperties{}

	query := `
		SELECT id, feature_id, karbari, rgb, owner, label, address, area, density, stability,
		       price_psc, price_irr, minimum_price_percentage, created_at, updated_at
		FROM feature_properties
		WHERE feature_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, featureID).Scan(
		&properties.ID, &properties.FeatureID, &properties.Karbari, &properties.RGB,
		&properties.Owner, &properties.Label, &properties.Address, &properties.Area, &properties.Density,
		&properties.Stability, &properties.PricePSC, &properties.PriceIRR, &properties.MinimumPricePercentage,
		&properties.CreatedAt, &properties.UpdatedAt,
	)

	return properties, err
}

// Update updates feature properties
// Implements Laravel's FeatureProperties->update()
func (r *PropertiesRepository) Update(ctx context.Context, featureID uint64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	setParts := []string{}
	args := []interface{}{}

	for key, value := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = ?", key))
		args = append(args, value)
	}

	args = append(args, featureID)
	query := "UPDATE feature_properties SET " + joinStrings(setParts, ", ") + ", updated_at = NOW() WHERE feature_id = ?"

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// UpdatePricing updates pricing fields
// Implements Laravel's FeatureController@updateFeature logic
func (r *PropertiesRepository) UpdatePricing(ctx context.Context, featureID uint64, pricePSC, priceIRR string, minPercentage int) error {
	query := `
		UPDATE feature_properties
		SET price_psc = ?, price_irr = ?, minimum_price_percentage = ?, updated_at = NOW()
		WHERE feature_id = ?
	`

	_, err := r.db.ExecContext(ctx, query, pricePSC, priceIRR, minPercentage, featureID)
	return err
}

// UpdateStatus updates RGB status and label
func (r *PropertiesRepository) UpdateStatus(ctx context.Context, featureID uint64, rgb, owner, label string, minPercentage int) error {
	query := `
		UPDATE feature_properties
		SET rgb = ?, owner = ?, label = ?, minimum_price_percentage = ?, updated_at = NOW()
		WHERE feature_id = ?
	`

	_, err := r.db.ExecContext(ctx, query, rgb, owner, label, minPercentage, featureID)
	return err
}

// UpdateStatusWithTx updates RGB status within a transaction
func (r *PropertiesRepository) UpdateStatusWithTx(ctx context.Context, tx *sql.Tx, featureID uint64, rgb, owner, label string, minPercentage int) error {
	query := `
		UPDATE feature_properties
		SET rgb = ?, owner = ?, label = ?, minimum_price_percentage = ?, updated_at = NOW()
		WHERE feature_id = ?
	`
	_, err := tx.ExecContext(ctx, query, rgb, owner, label, minPercentage, featureID)
	return err
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
