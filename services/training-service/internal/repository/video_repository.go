package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metargb/training-service/internal/models"
)

// VideoRepositoryInterface defines the interface for video repository operations
type VideoRepositoryInterface interface {
	GetVideos(ctx context.Context, page, perPage int32, categoryID, subCategoryID *uint64) ([]*models.Video, int32, error)
	GetVideoBySlug(ctx context.Context, slug string) (*models.Video, error)
	GetVideoByFileName(ctx context.Context, fileName string) (*models.Video, error)
	SearchVideos(ctx context.Context, searchTerm string, page, perPage int32) ([]*models.Video, int32, error)
	GetVideoStats(ctx context.Context, videoID uint64) (*models.VideoStats, error)
	GetUserInteraction(ctx context.Context, videoID, userID uint64) (*bool, error)
	IncrementView(ctx context.Context, videoID uint64, ipAddress string) error
	AddInteraction(ctx context.Context, videoID, userID uint64, liked bool, ipAddress string) error
}

type VideoRepository struct {
	db *sql.DB
}

func NewVideoRepository(db *sql.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

// GetVideos retrieves paginated videos with optional category filters
func (r *VideoRepository) GetVideos(ctx context.Context, page, perPage int32, categoryID, subCategoryID *uint64) ([]*models.Video, int32, error) {
	query := `
		SELECT v.id, v.video_sub_category_id, v.title, v.slug, v.description, v.fileName, v.creator_code, v.image, v.created_at, v.updated_at
		FROM videos v
		WHERE 1=1
	`
	countQuery := "SELECT COUNT(*) FROM videos v WHERE 1=1"
	args := []interface{}{}

	if categoryID != nil {
		query += " AND EXISTS (SELECT 1 FROM video_sub_categories vsc WHERE vsc.id = v.video_sub_category_id AND vsc.video_category_id = ?)"
		countQuery += " AND EXISTS (SELECT 1 FROM video_sub_categories vsc WHERE vsc.id = v.video_sub_category_id AND vsc.video_category_id = ?)"
		args = append(args, *categoryID)
	}

	if subCategoryID != nil {
		query += " AND v.video_sub_category_id = ?"
		countQuery += " AND v.video_sub_category_id = ?"
		args = append(args, *subCategoryID)
	}

	// Get total count
	var total int32
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count videos: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY v.created_at DESC LIMIT ? OFFSET ?"
	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get videos: %w", err)
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		var video models.Video
		if err := rows.Scan(
			&video.ID,
			&video.VideoSubCategoryID,
			&video.Title,
			&video.Slug,
			&video.Description,
			&video.FileName,
			&video.CreatorCode,
			&video.Image,
			&video.CreatedAt,
			&video.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan video: %w", err)
		}
		videos = append(videos, &video)
	}

	return videos, total, nil
}

// GetVideoBySlug retrieves a video by slug
func (r *VideoRepository) GetVideoBySlug(ctx context.Context, slug string) (*models.Video, error) {
	query := `
		SELECT id, video_sub_category_id, title, slug, description, fileName, creator_code, image, created_at, updated_at
		FROM videos
		WHERE slug = ?
	`

	var video models.Video
	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&video.ID,
		&video.VideoSubCategoryID,
		&video.Title,
		&video.Slug,
		&video.Description,
		&video.FileName,
		&video.CreatorCode,
		&video.Image,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}

	return &video, nil
}

// GetVideoByFileName retrieves a video by partial file name match
func (r *VideoRepository) GetVideoByFileName(ctx context.Context, fileName string) (*models.Video, error) {
	query := `
		SELECT id, video_sub_category_id, title, slug, description, fileName, creator_code, image, created_at, updated_at
		FROM videos
		WHERE fileName LIKE ?
		LIMIT 1
	`

	var video models.Video
	err := r.db.QueryRowContext(ctx, query, "%"+fileName+"%").Scan(
		&video.ID,
		&video.VideoSubCategoryID,
		&video.Title,
		&video.Slug,
		&video.Description,
		&video.FileName,
		&video.CreatorCode,
		&video.Image,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get video by file name: %w", err)
	}

	return &video, nil
}

// SearchVideos searches videos by title
func (r *VideoRepository) SearchVideos(ctx context.Context, searchTerm string, page, perPage int32) ([]*models.Video, int32, error) {
	query := `
		SELECT id, video_sub_category_id, title, slug, description, fileName, creator_code, image, created_at, updated_at
		FROM videos
		WHERE title LIKE ?
		ORDER BY created_at DESC
	`
	countQuery := "SELECT COUNT(*) FROM videos WHERE title LIKE ?"

	searchPattern := "%" + searchTerm + "%"

	// Get total count
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count videos: %w", err)
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += " LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, searchPattern, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search videos: %w", err)
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		var video models.Video
		if err := rows.Scan(
			&video.ID,
			&video.VideoSubCategoryID,
			&video.Title,
			&video.Slug,
			&video.Description,
			&video.FileName,
			&video.CreatorCode,
			&video.Image,
			&video.CreatedAt,
			&video.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan video: %w", err)
		}
		videos = append(videos, &video)
	}

	return videos, total, nil
}

// GetVideoStats retrieves statistics for a video
func (r *VideoRepository) GetVideoStats(ctx context.Context, videoID uint64) (*models.VideoStats, error) {
	stats := &models.VideoStats{}

	// Get views count
	viewQuery := "SELECT COUNT(*) FROM views WHERE viewable_type = 'App\\\\Models\\\\Video' AND viewable_id = ?"
	r.db.QueryRowContext(ctx, viewQuery, videoID).Scan(&stats.ViewsCount)

	// Get likes count
	likeQuery := "SELECT COUNT(*) FROM interactions WHERE likeable_type = 'App\\\\Models\\\\Video' AND likeable_id = ? AND liked = 1"
	r.db.QueryRowContext(ctx, likeQuery, videoID).Scan(&stats.LikesCount)

	// Get dislikes count
	dislikeQuery := "SELECT COUNT(*) FROM interactions WHERE likeable_type = 'App\\\\Models\\\\Video' AND likeable_id = ? AND liked = 0"
	r.db.QueryRowContext(ctx, dislikeQuery, videoID).Scan(&stats.DislikesCount)

	// Get comments count
	commentQuery := "SELECT COUNT(*) FROM comments WHERE commentable_type = 'App\\\\Models\\\\Video' AND commentable_id = ? AND parent_id IS NULL"
	r.db.QueryRowContext(ctx, commentQuery, videoID).Scan(&stats.CommentsCount)

	return stats, nil
}

// GetUserInteraction returns the user's like state for a video (nil = no interaction).
func (r *VideoRepository) GetUserInteraction(ctx context.Context, videoID, userID uint64) (*bool, error) {
	query := `
		SELECT liked
		FROM interactions
		WHERE likeable_type = 'App\\Models\\Video' AND likeable_id = ? AND user_id = ?
		LIMIT 1
	`
	var liked bool
	err := r.db.QueryRowContext(ctx, query, videoID, userID).Scan(&liked)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user interaction: %w", err)
	}
	return &liked, nil
}

// IncrementView adds a view for a video
func (r *VideoRepository) IncrementView(ctx context.Context, videoID uint64, ipAddress string) error {
	query := "INSERT INTO views (viewable_type, viewable_id, ip_address, created_at, updated_at) VALUES ('App\\\\Models\\\\Video', ?, ?, NOW(), NOW())"
	_, err := r.db.ExecContext(ctx, query, videoID, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to increment view: %w", err)
	}
	return nil
}

// AddInteraction adds or updates a user's interaction on a video
func (r *VideoRepository) AddInteraction(ctx context.Context, videoID, userID uint64, liked bool, ipAddress string) error {
	query := `
		INSERT INTO interactions (likeable_type, likeable_id, user_id, liked, ip_address, created_at, updated_at) 
		VALUES ('App\\Models\\Video', ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE liked = ?, ip_address = ?, updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, videoID, userID, liked, ipAddress, liked, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to add interaction: %w", err)
	}

	return nil
}
