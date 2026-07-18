package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type ImageRepository struct {
	db *sql.DB
}

func NewImageRepository(db *sql.DB) *ImageRepository {
	return &ImageRepository{db: db}
}

// GetImagesByFeatureID retrieves all images for a feature
// Uses polymorphic relationship: imageable_type = 'App\\Models\\Feature'
func (r *ImageRepository) GetImagesByFeatureID(ctx context.Context, featureID uint64) ([]*Image, error) {
	query := `
		SELECT id, url
		FROM images
		WHERE imageable_type = 'App\\Models\\Feature' AND imageable_id = ?
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	images := []*Image{}
	for rows.Next() {
		img := &Image{}
		if err := rows.Scan(&img.ID, &img.URL); err != nil {
			continue
		}
		images = append(images, img)
	}

	return images, nil
}

// Image represents a feature image
type Image struct {
	ID  uint64
	URL string
}

// CreateImage creates a new image record for a feature
// imageable_type = 'App\\Models\\Feature', imageable_id = featureID
func (r *ImageRepository) CreateImage(ctx context.Context, featureID uint64, url string) (*Image, error) {
	query := `
		INSERT INTO images (imageable_type, imageable_id, url, created_at, updated_at)
		VALUES ('App\\Models\\Feature', ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, featureID, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create image: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get image ID: %w", err)
	}

	return &Image{
		ID:  uint64(id),
		URL: url,
	}, nil
}

// DeleteImage deletes an image record
// Verifies that the image belongs to the feature before deletion
func (r *ImageRepository) DeleteImage(ctx context.Context, featureID, imageID uint64) error {
	query := `
		DELETE FROM images
		WHERE id = ? AND imageable_type = 'App\\Models\\Feature' AND imageable_id = ?
	`

	result, err := r.db.ExecContext(ctx, query, imageID, featureID)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("image not found or does not belong to feature")
	}

	return nil
}

// GetImageByID retrieves an image by ID and verifies it belongs to the feature
func (r *ImageRepository) GetImageByID(ctx context.Context, featureID, imageID uint64) (*Image, error) {
	query := `
		SELECT id, url
		FROM images
		WHERE id = ? AND imageable_type = 'App\\Models\\Feature' AND imageable_id = ?
	`

	img := &Image{}
	err := r.db.QueryRowContext(ctx, query, imageID, featureID).Scan(&img.ID, &img.URL)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	return img, nil
}
