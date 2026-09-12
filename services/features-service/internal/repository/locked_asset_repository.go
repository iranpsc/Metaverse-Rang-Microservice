package repository

import (
	"context"
	"database/sql"

	"metarang/features-service/internal/models"
)

type LockedAssetRepository struct {
	db *sql.DB
}

func NewLockedAssetRepository(db *sql.DB) *LockedAssetRepository {
	return &LockedAssetRepository{db: db}
}

// Create locks assets for a buy request
func (r *LockedAssetRepository) Create(ctx context.Context, userID, buyRequestID, featureID uint64, psc, irr float64) (uint64, error) {
	query := `
		INSERT INTO locked_assets (user_id, buy_feature_request_id, feature_id, psc, irr, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, userID, buyRequestID, featureID, psc, irr)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return uint64(id), err
}

// GetByBuyRequestID retrieves locked assets for a buy request
func (r *LockedAssetRepository) GetByBuyRequestID(ctx context.Context, buyRequestID uint64) (*models.LockedAsset, error) {
	asset := &models.LockedAsset{}

	query := `
		SELECT id, buy_feature_request_id, feature_id, psc, irr, created_at, updated_at
		FROM locked_assets
		WHERE buy_feature_request_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, buyRequestID).Scan(
		&asset.ID, &asset.BuyFeatureRequestID, &asset.FeatureID,
		&asset.PSC, &asset.IRR, &asset.CreatedAt, &asset.UpdatedAt,
	)

	return asset, err
}

// Delete removes locked assets (after acceptance or cancellation)
func (r *LockedAssetRepository) Delete(ctx context.Context, buyRequestID uint64) error {
	query := "DELETE FROM locked_assets WHERE buy_feature_request_id = ?"
	_, err := r.db.ExecContext(ctx, query, buyRequestID)
	return err
}

// DeleteAllForFeature deletes all locked assets for a feature's buy requests
func (r *LockedAssetRepository) DeleteAllForFeature(ctx context.Context, featureID uint64) error {
	query := "DELETE FROM locked_assets WHERE feature_id = ?"
	_, err := r.db.ExecContext(ctx, query, featureID)
	return err
}

// DeleteWithTx deletes within a transaction
func (r *LockedAssetRepository) DeleteWithTx(ctx context.Context, tx *sql.Tx, buyRequestID uint64) error {
	query := "DELETE FROM locked_assets WHERE buy_feature_request_id = ?"
	_, err := tx.ExecContext(ctx, query, buyRequestID)
	return err
}
