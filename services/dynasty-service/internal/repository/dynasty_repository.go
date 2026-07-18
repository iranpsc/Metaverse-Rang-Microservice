// Package repository provides data access for the dynasty service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/dynasty-service/internal/models"
)

type DynastyRepository struct {
	db *sql.DB
}

func NewDynastyRepository(db *sql.DB) *DynastyRepository {
	return &DynastyRepository{db: db}
}

// CreateDynasty creates a new dynasty
func (r *DynastyRepository) CreateDynasty(ctx context.Context, dynasty *models.Dynasty) error {
	query := `INSERT INTO dynasties (user_id, feature_id, created_at, updated_at) 
	          VALUES (?, ?, NOW(), NOW())`

	result, err := r.db.ExecContext(ctx, query, dynasty.UserID, dynasty.FeatureID)
	if err != nil {
		return fmt.Errorf("failed to create dynasty: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get dynasty ID: %w", err)
	}

	dynasty.ID = uint64(id)
	return nil
}

// GetDynastyByID retrieves a dynasty by ID
func (r *DynastyRepository) GetDynastyByID(ctx context.Context, id uint64) (*models.Dynasty, error) {
	query := `SELECT id, user_id, feature_id, created_at, updated_at 
	          FROM dynasties WHERE id = ?`

	var dynasty models.Dynasty
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dynasty.ID,
		&dynasty.UserID,
		&dynasty.FeatureID,
		&dynasty.CreatedAt,
		&dynasty.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty: %w", err)
	}

	return &dynasty, nil
}

// GetDynastyByUserID retrieves a dynasty by user ID
func (r *DynastyRepository) GetDynastyByUserID(ctx context.Context, userID uint64) (*models.Dynasty, error) {
	query := `SELECT id, user_id, feature_id, created_at, updated_at 
	          FROM dynasties WHERE user_id = ?`

	var dynasty models.Dynasty
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&dynasty.ID,
		&dynasty.UserID,
		&dynasty.FeatureID,
		&dynasty.CreatedAt,
		&dynasty.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty: %w", err)
	}

	return &dynasty, nil
}

// UpdateDynastyFeature updates the feature associated with a dynasty
func (r *DynastyRepository) UpdateDynastyFeature(ctx context.Context, dynastyID, featureID uint64) error {
	query := `UPDATE dynasties SET feature_id = ?, updated_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, featureID, dynastyID)
	if err != nil {
		return fmt.Errorf("failed to update dynasty feature: %w", err)
	}

	return nil
}

// GetFeatureDetails retrieves feature details for dynasty response
func (r *DynastyRepository) GetFeatureDetails(ctx context.Context, featureID uint64) (map[string]interface{}, error) {
	query := `
		SELECT 
			f.id,
			fp.id as properties_id,
			fp.area,
			fp.density,
			fp.stability
		FROM features f
		JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.id = ?
	`

	var (
		id           uint64
		propertiesID string
		area         string
		density      string
		stability    string
	)

	err := r.db.QueryRowContext(ctx, query, featureID).Scan(
		&id, &propertiesID, &area, &density, &stability,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get feature details: %w", err)
	}

	return map[string]interface{}{
		"id":            id,
		"properties_id": propertiesID,
		"area":          area,
		"density":       density,
		"stability":     stability,
	}, nil
}

// GetUserFeatures retrieves user's features excluding dynasty feature
func (r *DynastyRepository) GetUserFeatures(ctx context.Context, userID, excludeFeatureID uint64) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			f.id,
			fp.id as properties_id,
			fp.area,
			fp.density,
			fp.stability,
			fp.karbari
		FROM features f
		JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.user_id = ? AND f.id != ? AND fp.karbari = 'm'
	`

	rows, err := r.db.QueryContext(ctx, query, userID, excludeFeatureID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user features: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var features []map[string]interface{}
	for rows.Next() {
		var (
			id           uint64
			propertiesID string
			area         string
			density      string
			stability    string
			karbari      string
		)

		if err := rows.Scan(&id, &propertiesID, &area, &density, &stability, &karbari); err != nil {
			return nil, fmt.Errorf("failed to scan feature: %w", err)
		}

		features = append(features, map[string]interface{}{
			"id":            id,
			"properties_id": propertiesID,
			"area":          area,
			"density":       density,
			"stability":     stability,
		})
	}

	return features, nil
}

// GetUserProfilePhoto retrieves user's latest profile photo
func (r *DynastyRepository) GetUserProfilePhoto(ctx context.Context, userID uint64) (*string, error) {
	query := `
		SELECT url FROM images 
		WHERE imageable_type = 'App\\Models\\User' 
		AND imageable_id = ? 
		ORDER BY id DESC LIMIT 1
	`

	var url string
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile photo: %w", err)
	}

	return &url, nil
}

// GetDynastyMessage retrieves a dynasty message by type
func (r *DynastyRepository) GetDynastyMessage(ctx context.Context, messageType string) (string, error) {
	query := `SELECT message FROM dynasty_messages WHERE type = ? LIMIT 1`

	var message string
	err := r.db.QueryRowContext(ctx, query, messageType).Scan(&message)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get dynasty message: %w", err)
	}

	return message, nil
}

// GetVariableRate retrieves a variable rate by asset name (e.g. "psc").
func (r *DynastyRepository) GetVariableRate(ctx context.Context, asset string) (float64, error) {
	query := `SELECT price FROM variables WHERE asset = ? LIMIT 1`

	var price float64
	err := r.db.QueryRowContext(ctx, query, asset).Scan(&price)
	if err == sql.ErrNoRows {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get variable rate: %w", err)
	}
	if price == 0 {
		return 1, nil
	}

	return price, nil
}

// CheckFeatureHasPendingRequest checks if feature has pending join requests
func (r *DynastyRepository) CheckFeatureHasPendingRequest(ctx context.Context, featureID uint64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM join_requests jr
			INNER JOIN dynasties d ON d.user_id = jr.from_user
			WHERE d.feature_id = ? AND jr.status = 0
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, featureID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check feature pending requests: %w", err)
	}

	return exists, nil
}

// GetFeaturePenaltyData returns karbari + stability used for dynasty feature-change penalties.
func (r *DynastyRepository) GetFeaturePenaltyData(ctx context.Context, featureID uint64) (karbari string, stability float64, err error) {
	const q = `
		SELECT fp.karbari, fp.stability
		FROM feature_properties fp
		WHERE fp.feature_id = ?
		LIMIT 1
	`
	err = r.db.QueryRowContext(ctx, q, featureID).Scan(&karbari, &stability)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("failed to get feature penalty data: %w", err)
	}
	return karbari, stability, nil
}

// CreateDebt inserts debt row in debts table and sets the color-specific column.
func (r *DynastyRepository) CreateDebt(ctx context.Context, userID uint64, color string, amount float64, reason string) error {
	colorColumn := "yellow"
	switch color {
	case "red":
		colorColumn = "red"
	case "blue":
		colorColumn = "blue"
	case "yellow":
		colorColumn = "yellow"
	}
	q := fmt.Sprintf(`
		INSERT INTO debts (user_id, %s, reason, status, created_at, updated_at)
		VALUES (?, ?, ?, 1, NOW(), NOW())
	`, colorColumn)
	if _, err := r.db.ExecContext(ctx, q, userID, amount, reason); err != nil {
		return fmt.Errorf("failed to create debt: %w", err)
	}
	return nil
}

// LockFeature inserts row into locked_features to prevent immediate reuse.
func (r *DynastyRepository) LockFeature(ctx context.Context, featureID uint64, reason string, until time.Time, status int) error {
	const q = `
		INSERT INTO locked_features (feature_id, reason, until, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`
	if _, err := r.db.ExecContext(ctx, q, featureID, reason, until, status); err != nil {
		return fmt.Errorf("failed to lock feature: %w", err)
	}
	return nil
}

// SetFeatureLabel updates feature_properties.label for given feature.
func (r *DynastyRepository) SetFeatureLabel(ctx context.Context, featureID uint64, label string) error {
	const q = `UPDATE feature_properties SET label = ?, updated_at = NOW() WHERE feature_id = ?`
	if _, err := r.db.ExecContext(ctx, q, label, featureID); err != nil {
		return fmt.Errorf("failed to update feature label: %w", err)
	}
	return nil
}
