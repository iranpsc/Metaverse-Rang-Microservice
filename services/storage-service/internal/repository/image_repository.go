// Package repository provides data access for the storage service.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/storage-service/internal/models"
)

type ImageRepository struct {
	db *sql.DB
}

func NewImageRepository(db *sql.DB) *ImageRepository {
	return &ImageRepository{db: db}
}

// CreateImage creates a new image record
func (r *ImageRepository) CreateImage(ctx context.Context, image *models.Image) error {
	query := `
		INSERT INTO images (imageable_type, imageable_id, url, type, created_at, updated_at) 
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		image.ImageableType,
		image.ImageableID,
		image.URL,
		image.Type,
	)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get image ID: %w", err)
	}

	image.ID = uint64(id)
	return nil
}

// GetImages retrieves images for a specific entity
func (r *ImageRepository) GetImages(ctx context.Context, imageableType string, imageableID uint64, imageType string) ([]*models.Image, error) {
	query := "SELECT id, imageable_type, imageable_id, url, type, created_at, updated_at FROM images WHERE imageable_type = ? AND imageable_id = ?"
	args := []interface{}{imageableType, imageableID}

	// Optional type filter
	if imageType != "" {
		query += " AND type = ?"
		args = append(args, imageType)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
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
			&image.Type,
			&image.CreatedAt,
			&image.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan image: %w", err)
		}
		images = append(images, &image)
	}

	return images, nil
}

// GetImageByID retrieves an image by ID
func (r *ImageRepository) GetImageByID(ctx context.Context, id uint64) (*models.Image, error) {
	query := "SELECT id, imageable_type, imageable_id, url, type, created_at, updated_at FROM images WHERE id = ?"

	var image models.Image
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&image.ID,
		&image.ImageableType,
		&image.ImageableID,
		&image.URL,
		&image.Type,
		&image.CreatedAt,
		&image.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	return &image, nil
}

// DeleteImage deletes an image record
func (r *ImageRepository) DeleteImage(ctx context.Context, id uint64) error {
	query := "DELETE FROM images WHERE id = ?"

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	return nil
}
