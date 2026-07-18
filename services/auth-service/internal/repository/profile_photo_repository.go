package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/auth-service/internal/models"
)

type ProfilePhotoRepository interface {
	// Create creates a new profile photo record
	Create(ctx context.Context, userID uint64, url string) (*models.Image, error)
	// FindByUserID finds all profile photos for a user, ordered by creation time
	FindByUserID(ctx context.Context, userID uint64) ([]*models.Image, error)
	// FindByID finds a profile photo by ID
	FindByID(ctx context.Context, id uint64) (*models.Image, error)
	// Delete deletes a profile photo by ID
	Delete(ctx context.Context, id uint64) error
	// CheckOwnership checks if a profile photo belongs to a user
	CheckOwnership(ctx context.Context, id uint64, userID uint64) (bool, error)
}

type profilePhotoRepository struct {
	db *sql.DB
}

func NewProfilePhotoRepository(db *sql.DB) ProfilePhotoRepository {
	return &profilePhotoRepository{db: db}
}

// Create creates a new profile photo record with polymorphic relation to User
func (r *profilePhotoRepository) Create(ctx context.Context, userID uint64, url string) (*models.Image, error) {
	query := `
		INSERT INTO images (imageable_type, imageable_id, url, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, "App\\Models\\User", userID, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile photo: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get profile photo ID: %w", err)
	}

	image := &models.Image{
		ID:            uint64(id),
		ImageableType: "App\\Models\\User",
		ImageableID:   userID,
		URL:           url,
	}

	return image, nil
}

// FindByUserID finds all profile photos for a user, ordered by creation time (oldest first)
func (r *profilePhotoRepository) FindByUserID(ctx context.Context, userID uint64) ([]*models.Image, error) {
	query := `
		SELECT id, imageable_type, imageable_id, url, created_at, updated_at
		FROM images
		WHERE imageable_type = ? AND imageable_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, "App\\Models\\User", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find profile photos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []*models.Image
	for rows.Next() {
		var image models.Image
		if err := rows.Scan(
			&image.ID,
			&image.ImageableType,
			&image.ImageableID,
			&image.URL,
			&image.CreatedAt,
			&image.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan profile photo: %w", err)
		}
		images = append(images, &image)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profile photos: %w", err)
	}

	return images, nil
}

// FindByID finds a profile photo by ID
func (r *profilePhotoRepository) FindByID(ctx context.Context, id uint64) (*models.Image, error) {
	query := `
		SELECT id, imageable_type, imageable_id, url, created_at, updated_at
		FROM images
		WHERE id = ?
	`

	var image models.Image
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&image.ID,
		&image.ImageableType,
		&image.ImageableID,
		&image.URL,
		&image.CreatedAt,
		&image.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find profile photo: %w", err)
	}

	return &image, nil
}

// Delete deletes a profile photo by ID
func (r *profilePhotoRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM images WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete profile photo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("profile photo not found")
	}

	return nil
}

// CheckOwnership checks if a profile photo belongs to a user
func (r *profilePhotoRepository) CheckOwnership(ctx context.Context, id uint64, userID uint64) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM images
		WHERE id = ? AND imageable_type = ? AND imageable_id = ?
	`

	var owns bool
	err := r.db.QueryRowContext(ctx, query, id, "App\\Models\\User", userID).Scan(&owns)
	if err != nil {
		return false, fmt.Errorf("failed to check ownership: %w", err)
	}

	return owns, nil
}
