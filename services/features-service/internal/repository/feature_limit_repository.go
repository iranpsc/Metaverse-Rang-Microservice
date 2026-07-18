package repository

import (
	"context"
	"database/sql"
	"time"

	"metarang/features-service/internal/models"
)

type FeatureLimitRepository struct {
	db *sql.DB
}

func NewFeatureLimitRepository(db *sql.DB) *FeatureLimitRepository {
	return &FeatureLimitRepository{db: db}
}

// GetLimitationByPropertyID checks if a feature property is within a limited campaign
// Implements Laravel's getLimitation() logic from BuyFeatureController and FeaturePolicy
func (r *FeatureLimitRepository) GetLimitationByPropertyID(ctx context.Context, propertyID string) (*models.FeatureLimit, error) {
	limit := &models.FeatureLimit{}

	query := `
		SELECT id, title, start_date, end_date, start_id, end_id,
		       price_limit, verified_kyc_limit, under_18_limit, more_than_18_limit,
		       dynasty_owner_limit, individual_buy_limit, individual_buy_count, expired,
		       created_at, updated_at
		FROM feature_limits
		WHERE expired = 0
		  AND start_date <= NOW()
		  AND end_date >= NOW()
		  AND start_id <= ?
		  AND end_id >= ?
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, propertyID, propertyID).Scan(
		&limit.ID, &limit.Title, &limit.StartDate, &limit.EndDate,
		&limit.StartID, &limit.EndID, &limit.PriceLimit, &limit.VerifiedKYCLimit,
		&limit.Under18Limit, &limit.MoreThan18Limit, &limit.DynastyOwnerLimit,
		&limit.IndividualBuyLimit, &limit.IndividualBuyCount, &limit.Expired,
		&limit.CreatedAt, &limit.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No limitation found
	}

	return limit, err
}

// CountLimitedPurchases counts how many times a user has purchased from this limitation
func (r *FeatureLimitRepository) CountLimitedPurchases(ctx context.Context, userID, limitID uint64) (int, error) {
	var count int

	query := `
		SELECT COUNT(*)
		FROM limited_feature_purchases
		WHERE user_id = ? AND feature_limit_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, userID, limitID).Scan(&count)
	return count, err
}

// TrackLimitedPurchase records a purchase from a limited feature campaign
func (r *FeatureLimitRepository) TrackLimitedPurchase(ctx context.Context, userID, limitID, featureID uint64) error {
	query := `
		INSERT INTO limited_feature_purchases (user_id, feature_limit_id, feature_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, userID, limitID, featureID)
	return err
}

// GetActiveLimitations retrieves all active feature limitations
func (r *FeatureLimitRepository) GetActiveLimitations(ctx context.Context) ([]*models.FeatureLimit, error) {
	query := `
		SELECT id, title, start_date, end_date, start_id, end_id,
		       price_limit, verified_kyc_limit, under_18_limit, more_than_18_limit,
		       dynasty_owner_limit, individual_buy_limit, individual_buy_count, expired,
		       created_at, updated_at
		FROM feature_limits
		WHERE expired = 0
		  AND start_date <= ?
		  AND end_date >= ?
	`

	now := time.Now()
	rows, err := r.db.QueryContext(ctx, query, now, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	limits := []*models.FeatureLimit{}
	for rows.Next() {
		limit := &models.FeatureLimit{}
		if err := rows.Scan(
			&limit.ID, &limit.Title, &limit.StartDate, &limit.EndDate,
			&limit.StartID, &limit.EndID, &limit.PriceLimit, &limit.VerifiedKYCLimit,
			&limit.Under18Limit, &limit.MoreThan18Limit, &limit.DynastyOwnerLimit,
			&limit.IndividualBuyLimit, &limit.IndividualBuyCount, &limit.Expired,
			&limit.CreatedAt, &limit.UpdatedAt,
		); err != nil {
			continue
		}
		limits = append(limits, limit)
	}

	return limits, nil
}
