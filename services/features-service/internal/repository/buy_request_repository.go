package repository

import (
	"context"
	"database/sql"

	"metarang/features-service/internal/models"
)

type BuyRequestRepository struct {
	db *sql.DB
}

func NewBuyRequestRepository(db *sql.DB) *BuyRequestRepository {
	return &BuyRequestRepository{db: db}
}

// Create creates a new buy feature request
func (r *BuyRequestRepository) Create(ctx context.Context, buyerID, sellerID, featureID uint64, note string, pricePSC, priceIRR float64) (uint64, error) {
	query := `
		INSERT INTO buy_feature_requests (buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, requested_grace_period, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, NULL, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, buyerID, sellerID, featureID, note, pricePSC, priceIRR)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return uint64(id), err
}

// FindByID retrieves a buy request by ID (excluding soft-deleted)
func (r *BuyRequestRepository) FindByID(ctx context.Context, id uint64) (*models.BuyFeatureRequest, error) {
	request := &models.BuyFeatureRequest{}

	query := `
		SELECT id, buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, requested_grace_period, created_at, updated_at
		FROM buy_feature_requests
		WHERE id = ? AND deleted_at IS NULL
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&request.ID, &request.BuyerID, &request.SellerID, &request.FeatureID,
		&request.Note, &request.PricePSC, &request.PriceIRR, &request.Status,
		&request.RequestedGracePeriod, &request.CreatedAt, &request.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return request, err
}

// SoftDelete soft deletes a buy request
func (r *BuyRequestRepository) SoftDelete(ctx context.Context, id uint64) error {
	query := "UPDATE buy_feature_requests SET deleted_at = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// CancelAllForFeature cancels (soft deletes) all buy requests for a feature
// Implements Laravel's cancelBuyRequests() logic
func (r *BuyRequestRepository) CancelAllForFeature(ctx context.Context, featureID uint64) error {
	query := "UPDATE buy_feature_requests SET deleted_at = NOW() WHERE feature_id = ? AND deleted_at IS NULL"
	_, err := r.db.ExecContext(ctx, query, featureID)
	return err
}

// GetAllForFeature gets all non-deleted buy requests for a feature
func (r *BuyRequestRepository) GetAllForFeature(ctx context.Context, featureID uint64) ([]*models.BuyFeatureRequest, error) {
	query := `
		SELECT id, buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, requested_grace_period, created_at, updated_at
		FROM buy_feature_requests
		WHERE feature_id = ? AND deleted_at IS NULL
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	requests := []*models.BuyFeatureRequest{}
	for rows.Next() {
		req := &models.BuyFeatureRequest{}
		if err := rows.Scan(
			&req.ID, &req.BuyerID, &req.SellerID, &req.FeatureID,
			&req.Note, &req.PricePSC, &req.PriceIRR, &req.Status,
			&req.RequestedGracePeriod, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			continue
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// UpdateStatus updates the status of a buy request
func (r *BuyRequestRepository) UpdateStatus(ctx context.Context, id uint64, status int) error {
	query := "UPDATE buy_feature_requests SET status = ?, updated_at = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

// UpdateStatusWithTx updates the status within a transaction
func (r *BuyRequestRepository) UpdateStatusWithTx(ctx context.Context, tx *sql.Tx, id uint64, status int) error {
	query := "UPDATE buy_feature_requests SET status = ?, updated_at = NOW() WHERE id = ?"
	_, err := tx.ExecContext(ctx, query, status, id)
	return err
}

// SoftDeleteWithTx soft deletes within a transaction
func (r *BuyRequestRepository) SoftDeleteWithTx(ctx context.Context, tx *sql.Tx, id uint64) error {
	query := "UPDATE buy_feature_requests SET deleted_at = NOW() WHERE id = ?"
	_, err := tx.ExecContext(ctx, query, id)
	return err
}

// ListByBuyerID retrieves all buy requests for a buyer (excluding soft-deleted)
func (r *BuyRequestRepository) ListByBuyerID(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error) {
	query := `
		SELECT id, buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, requested_grace_period, created_at, updated_at
		FROM buy_feature_requests
		WHERE buyer_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, buyerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	requests := []*models.BuyFeatureRequest{}
	for rows.Next() {
		req := &models.BuyFeatureRequest{}
		if err := rows.Scan(
			&req.ID, &req.BuyerID, &req.SellerID, &req.FeatureID,
			&req.Note, &req.PricePSC, &req.PriceIRR, &req.Status,
			&req.RequestedGracePeriod, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			continue
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// ListBySellerID retrieves all buy requests received by a seller (excluding soft-deleted)
func (r *BuyRequestRepository) ListBySellerID(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error) {
	query := `
		SELECT id, buyer_id, seller_id, feature_id, note, price_psc, price_irr, status, requested_grace_period, created_at, updated_at
		FROM buy_feature_requests
		WHERE seller_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	requests := []*models.BuyFeatureRequest{}
	for rows.Next() {
		req := &models.BuyFeatureRequest{}
		if err := rows.Scan(
			&req.ID, &req.BuyerID, &req.SellerID, &req.FeatureID,
			&req.Note, &req.PricePSC, &req.PriceIRR, &req.Status,
			&req.RequestedGracePeriod, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			continue
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// Delete hard deletes a buy request (used for reject/delete operations)
func (r *BuyRequestRepository) Delete(ctx context.Context, id uint64) error {
	query := "DELETE FROM buy_feature_requests WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateGracePeriod updates the requested_grace_period field
func (r *BuyRequestRepository) UpdateGracePeriod(ctx context.Context, id uint64, gracePeriod sql.NullTime) error {
	query := "UPDATE buy_feature_requests SET requested_grace_period = ?, updated_at = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, gracePeriod, id)
	return err
}

// HasPendingRequest checks if buyer has a pending request for the feature
func (r *BuyRequestRepository) HasPendingRequest(ctx context.Context, buyerID, featureID uint64) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM buy_feature_requests 
		WHERE buyer_id = ? AND feature_id = ? AND status = 0 AND deleted_at IS NULL
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, buyerID, featureID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
