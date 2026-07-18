// Package repository provides data access for the training service.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/training-service/internal/models"
)

// CategoryRepositoryInterface defines the interface for category repository operations.
type CategoryRepositoryInterface interface {
	GetCategories(ctx context.Context, page, perPage int32) ([]*models.VideoCategory, int32, error)
	GetCategoryByID(ctx context.Context, categoryID uint64) (*models.VideoCategory, error)
	GetCategoryBySlug(ctx context.Context, slug string) (*models.VideoCategory, error)
	GetSubCategoriesByCategoryID(ctx context.Context, categoryID uint64) ([]*models.VideoSubCategory, error)
	GetSubCategoryByID(ctx context.Context, subCategoryID uint64) (*models.VideoSubCategory, error)
	GetSubCategoryBySlugs(ctx context.Context, categorySlug, subCategorySlug string) (*models.VideoSubCategory, error)
	GetCategoryStats(ctx context.Context, categoryID uint64) (*models.CategoryStats, error)
	GetSubCategoryStats(ctx context.Context, subCategoryID uint64) (*models.SubCategoryStats, error)
	GetSubCategoryStatsByCategoryID(ctx context.Context, categoryID uint64) (map[uint64]*models.SubCategoryStats, error)
}

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetCategories retrieves paginated categories with stats
func (r *CategoryRepository) GetCategories(ctx context.Context, page, perPage int32) ([]*models.VideoCategory, int32, error) {
	query := `
		SELECT id, name, slug, description, image, icon, created_at, updated_at
		FROM video_categories
		ORDER BY (
			SELECT COUNT(*)
			FROM interactions i
			INNER JOIN videos vid ON vid.id = i.likeable_id
			INNER JOIN video_sub_categories vsc ON vsc.id = vid.video_sub_category_id
			WHERE i.likeable_type = 'App\\Models\\Video' AND i.liked = 1 AND vsc.video_category_id = video_categories.id
		) DESC
	`
	countQuery := "SELECT COUNT(*) FROM video_categories"

	// Get total count
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count categories: %w", err)
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += " LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var categories []*models.VideoCategory
	for rows.Next() {
		var category models.VideoCategory
		if err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Slug,
			&category.Description,
			&category.Image,
			&category.Icon,
			&category.CreatedAt,
			&category.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, total, nil
}

// GetCategoryByID retrieves a category by ID
func (r *CategoryRepository) GetCategoryByID(ctx context.Context, categoryID uint64) (*models.VideoCategory, error) {
	query := `
		SELECT id, name, slug, description, image, icon, created_at, updated_at
		FROM video_categories
		WHERE id = ?
	`

	var category models.VideoCategory
	err := r.db.QueryRowContext(ctx, query, categoryID).Scan(
		&category.ID,
		&category.Name,
		&category.Slug,
		&category.Description,
		&category.Image,
		&category.Icon,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	return &category, nil
}

// GetCategoryBySlug retrieves a category by slug with subcategories
func (r *CategoryRepository) GetCategoryBySlug(ctx context.Context, slug string) (*models.VideoCategory, error) {
	query := `
		SELECT id, name, slug, description, image, icon, created_at, updated_at
		FROM video_categories
		WHERE slug = ?
	`

	var category models.VideoCategory
	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&category.ID,
		&category.Name,
		&category.Slug,
		&category.Description,
		&category.Image,
		&category.Icon,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	return &category, nil
}

// GetSubCategoriesByCategoryID retrieves all subcategories for a category
func (r *CategoryRepository) GetSubCategoriesByCategoryID(ctx context.Context, categoryID uint64) ([]*models.VideoSubCategory, error) {
	query := `
		SELECT id, video_category_id, name, slug, description, image, icon, created_at, updated_at
		FROM video_sub_categories
		WHERE video_category_id = ?
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subcategories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var subCategories []*models.VideoSubCategory
	for rows.Next() {
		var subCategory models.VideoSubCategory
		if err := rows.Scan(
			&subCategory.ID,
			&subCategory.VideoCategoryID,
			&subCategory.Name,
			&subCategory.Slug,
			&subCategory.Description,
			&subCategory.Image,
			&subCategory.Icon,
			&subCategory.CreatedAt,
			&subCategory.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan subcategory: %w", err)
		}
		subCategories = append(subCategories, &subCategory)
	}

	return subCategories, nil
}

// GetSubCategoryByID retrieves a subcategory by ID
func (r *CategoryRepository) GetSubCategoryByID(ctx context.Context, subCategoryID uint64) (*models.VideoSubCategory, error) {
	query := `
		SELECT id, video_category_id, name, slug, description, image, icon, created_at, updated_at
		FROM video_sub_categories
		WHERE id = ?
	`

	var subCategory models.VideoSubCategory
	err := r.db.QueryRowContext(ctx, query, subCategoryID).Scan(
		&subCategory.ID,
		&subCategory.VideoCategoryID,
		&subCategory.Name,
		&subCategory.Slug,
		&subCategory.Description,
		&subCategory.Image,
		&subCategory.Icon,
		&subCategory.CreatedAt,
		&subCategory.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subcategory: %w", err)
	}

	return &subCategory, nil
}

// GetSubCategoryBySlugs retrieves a subcategory by category and subcategory slugs
func (r *CategoryRepository) GetSubCategoryBySlugs(ctx context.Context, categorySlug, subCategorySlug string) (*models.VideoSubCategory, error) {
	query := `
		SELECT vsc.id, vsc.video_category_id, vsc.name, vsc.slug, vsc.description, vsc.image, vsc.icon, vsc.created_at, vsc.updated_at
		FROM video_sub_categories vsc
		INNER JOIN video_categories vc ON vc.id = vsc.video_category_id
		WHERE vc.slug = ? AND vsc.slug = ?
	`

	var subCategory models.VideoSubCategory
	err := r.db.QueryRowContext(ctx, query, categorySlug, subCategorySlug).Scan(
		&subCategory.ID,
		&subCategory.VideoCategoryID,
		&subCategory.Name,
		&subCategory.Slug,
		&subCategory.Description,
		&subCategory.Image,
		&subCategory.Icon,
		&subCategory.CreatedAt,
		&subCategory.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subcategory: %w", err)
	}

	return &subCategory, nil
}

// GetCategoryStats retrieves statistics for a category
func (r *CategoryRepository) GetCategoryStats(ctx context.Context, categoryID uint64) (*models.CategoryStats, error) {
	stats := &models.CategoryStats{}

	// Get videos count (through subcategories)
	videoQuery := `
		SELECT COUNT(*) 
		FROM videos v
		INNER JOIN video_sub_categories vsc ON vsc.id = v.video_sub_category_id
		WHERE vsc.video_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, videoQuery, categoryID).Scan(&stats.VideosCount); err != nil {
		return nil, fmt.Errorf("failed to get videos count: %w", err)
	}

	// Get views count
	viewQuery := `
		SELECT COUNT(*) 
		FROM views v
		INNER JOIN videos vid ON vid.id = v.viewable_id
		INNER JOIN video_sub_categories vsc ON vsc.id = vid.video_sub_category_id
		WHERE v.viewable_type = 'App\\Models\\Video' AND vsc.video_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, viewQuery, categoryID).Scan(&stats.ViewsCount); err != nil {
		return nil, fmt.Errorf("failed to get views count: %w", err)
	}

	// Get likes count
	likeQuery := `
		SELECT COUNT(*) 
		FROM interactions i
		INNER JOIN videos vid ON vid.id = i.likeable_id
		INNER JOIN video_sub_categories vsc ON vsc.id = vid.video_sub_category_id
		WHERE i.likeable_type = 'App\\Models\\Video' AND i.liked = 1 AND vsc.video_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, likeQuery, categoryID).Scan(&stats.LikesCount); err != nil {
		return nil, fmt.Errorf("failed to get likes count: %w", err)
	}

	// Get dislikes count
	dislikeQuery := `
		SELECT COUNT(*) 
		FROM interactions i
		INNER JOIN videos vid ON vid.id = i.likeable_id
		INNER JOIN video_sub_categories vsc ON vsc.id = vid.video_sub_category_id
		WHERE i.likeable_type = 'App\\Models\\Video' AND i.liked = 0 AND vsc.video_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, dislikeQuery, categoryID).Scan(&stats.DislikesCount); err != nil {
		return nil, fmt.Errorf("failed to get dislikes count: %w", err)
	}

	return stats, nil
}

// GetSubCategoryStats retrieves statistics for a subcategory
func (r *CategoryRepository) GetSubCategoryStats(ctx context.Context, subCategoryID uint64) (*models.SubCategoryStats, error) {
	stats := &models.SubCategoryStats{}

	// Get videos count
	videoQuery := "SELECT COUNT(*) FROM videos WHERE video_sub_category_id = ?"
	if err := r.db.QueryRowContext(ctx, videoQuery, subCategoryID).Scan(&stats.VideosCount); err != nil {
		return nil, fmt.Errorf("failed to get videos count: %w", err)
	}

	// Get views count
	viewQuery := `
		SELECT COUNT(*) 
		FROM views v
		INNER JOIN videos vid ON vid.id = v.viewable_id
		WHERE v.viewable_type = 'App\\Models\\Video' AND vid.video_sub_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, viewQuery, subCategoryID).Scan(&stats.ViewsCount); err != nil {
		return nil, fmt.Errorf("failed to get views count: %w", err)
	}

	// Get likes count
	likeQuery := `
		SELECT COUNT(*) 
		FROM interactions i
		INNER JOIN videos vid ON vid.id = i.likeable_id
		WHERE i.likeable_type = 'App\\Models\\Video' AND i.liked = 1 AND vid.video_sub_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, likeQuery, subCategoryID).Scan(&stats.LikesCount); err != nil {
		return nil, fmt.Errorf("failed to get likes count: %w", err)
	}

	// Get dislikes count
	dislikeQuery := `
		SELECT COUNT(*) 
		FROM interactions i
		INNER JOIN videos vid ON vid.id = i.likeable_id
		WHERE i.likeable_type = 'App\\Models\\Video' AND i.liked = 0 AND vid.video_sub_category_id = ?
	`
	if err := r.db.QueryRowContext(ctx, dislikeQuery, subCategoryID).Scan(&stats.DislikesCount); err != nil {
		return nil, fmt.Errorf("failed to get dislikes count: %w", err)
	}

	return stats, nil
}

// GetSubCategoryStatsByCategoryID retrieves statistics for all subcategories in a category
func (r *CategoryRepository) GetSubCategoryStatsByCategoryID(ctx context.Context, categoryID uint64) (map[uint64]*models.SubCategoryStats, error) {
	statsMap := make(map[uint64]*models.SubCategoryStats)

	// Get all subcategories for this category
	subCategories, err := r.GetSubCategoriesByCategoryID(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	// Get stats for each subcategory
	for _, subCategory := range subCategories {
		stats, err := r.GetSubCategoryStats(ctx, subCategory.ID)
		if err != nil {
			return nil, err
		}
		statsMap[subCategory.ID] = stats
	}

	return statsMap, nil
}
