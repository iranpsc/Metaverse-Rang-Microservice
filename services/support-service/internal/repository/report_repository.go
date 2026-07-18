package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/support-service/internal/models"
)

type ReportRepository interface {
	Create(ctx context.Context, report *models.Report) (*models.Report, error)
	GetByID(ctx context.Context, reportID uint64) (*models.ReportWithImages, error)
	GetByUserID(ctx context.Context, userID uint64, page, perPage int32) ([]*models.Report, int, error)
	CreateImage(ctx context.Context, reportID uint64, url string) error
}

type reportRepository struct {
	db *sql.DB
}

func NewReportRepository(db *sql.DB) ReportRepository {
	return &reportRepository{db: db}
}

func (r *reportRepository) Create(ctx context.Context, report *models.Report) (*models.Report, error) {
	query := `
		INSERT INTO reports (subject, title, content, url, user_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		report.Subject,
		report.Title,
		report.Content,
		report.URL,
		report.UserID,
		report.Status,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	report.ID = uint64(id)
	return report, nil
}

func (r *reportRepository) GetByID(ctx context.Context, reportID uint64) (*models.ReportWithImages, error) {
	query := `
		SELECT id, subject, title, content, url, user_id, status, created_at, updated_at
		FROM reports
		WHERE id = ?
	`

	var report models.ReportWithImages
	err := r.db.QueryRowContext(ctx, query, reportID).Scan(
		&report.ID, &report.Subject, &report.Title, &report.Content,
		&report.URL, &report.UserID, &report.Status, &report.CreatedAt, &report.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	// Load images
	imagesQuery := `
		SELECT id, imageable_type, imageable_id, url, created_at, updated_at
		FROM images
		WHERE imageable_type = 'App\\Models\\Report' AND imageable_id = ?
	`

	rows, err := r.db.QueryContext(ctx, imagesQuery, reportID)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []models.Image
	for rows.Next() {
		var img models.Image
		err := rows.Scan(&img.ID, &img.ImageableType, &img.ImageableID, &img.URL, &img.CreatedAt, &img.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan image: %w", err)
		}
		images = append(images, img)
	}

	report.Images = images
	return &report, nil
}

func (r *reportRepository) GetByUserID(ctx context.Context, userID uint64, page, perPage int32) ([]*models.Report, int, error) {
	// Count total reports
	countQuery := `SELECT COUNT(*) FROM reports WHERE user_id = ?`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count reports: %w", err)
	}

	// Get reports with pagination
	offset := (page - 1) * perPage
	query := `
		SELECT id, subject, title, content, url, user_id, status, created_at, updated_at
		FROM reports
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get reports: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reports []*models.Report
	for rows.Next() {
		var report models.Report
		err := rows.Scan(
			&report.ID, &report.Subject, &report.Title, &report.Content,
			&report.URL, &report.UserID, &report.Status, &report.CreatedAt, &report.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan report: %w", err)
		}
		reports = append(reports, &report)
	}

	return reports, total, nil
}

func (r *reportRepository) CreateImage(ctx context.Context, reportID uint64, url string) error {
	query := `
		INSERT INTO images (imageable_type, imageable_id, url, created_at, updated_at)
		VALUES ('App\\Models\\Report', ?, ?, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, reportID, url)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	return nil
}
