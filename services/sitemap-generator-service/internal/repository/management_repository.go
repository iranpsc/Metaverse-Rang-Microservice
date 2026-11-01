package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/sitemap-generator-service/internal/models"
)

type ManagementRepository struct{ db *sql.DB }

func NewManagementRepository(db *sql.DB) *ManagementRepository { return &ManagementRepository{db: db} }

func (r *ManagementRepository) CreateRun(ctx context.Context, run *models.GenerationRun) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO sitemap_generation_runs
		(trigger_type, target, language, status, started_at, finished_at, duration_ms, url_count, added_count, removed_count, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`, run.Trigger, run.Target, run.Language, run.Status,
		run.StartedAt, run.FinishedAt, run.DurationMS, run.URLCount, run.AddedCount, run.RemovedCount, run.Error)
	if err != nil {
		return 0, fmt.Errorf("create sitemap run: %w", err)
	}
	return result.LastInsertId()
}

func (r *ManagementRepository) CompleteRun(ctx context.Context, id int64, run *models.GenerationRun) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sitemap_generation_runs SET status=?, finished_at=?, duration_ms=?, url_count=?, added_count=?, removed_count=?, error_message=?, updated_at=NOW() WHERE id=?`,
		run.Status, run.FinishedAt, run.DurationMS, run.URLCount, run.AddedCount, run.RemovedCount, run.Error, id)
	if err != nil {
		return fmt.Errorf("complete sitemap run: %w", err)
	}
	return nil
}

func (r *ManagementRepository) Logs(ctx context.Context, limit int) ([]models.GenerationRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, _, err := r.LogsPage(ctx, 1, limit)
	return items, err
}

func (r *ManagementRepository) LogsPage(ctx context.Context, page, perPage int) ([]models.GenerationRun, int, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sitemap_generation_runs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sitemap logs: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := r.db.QueryContext(ctx, `SELECT id, trigger_type, target, COALESCE(language,''), status, started_at, finished_at, duration_ms, url_count, added_count, removed_count, COALESCE(error_message,'') FROM sitemap_generation_runs ORDER BY id DESC LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list sitemap logs: %w", err)
	}
	defer rows.Close()
	// Keep the JSON contract stable for empty pages (`[]`, never `null`).
	out := make([]models.GenerationRun, 0)
	for rows.Next() {
		var item models.GenerationRun
		if err := rows.Scan(&item.ID, &item.Trigger, &item.Target, &item.Language, &item.Status, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.URLCount, &item.AddedCount, &item.RemovedCount, &item.Error); err != nil {
			return nil, 0, fmt.Errorf("scan sitemap log: %w", err)
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *ManagementRepository) UpsertFile(ctx context.Context, item models.SitemapFile) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sitemap_files (language, filename, file_type, url_count, status, updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW()) ON DUPLICATE KEY UPDATE file_type=VALUES(file_type), url_count=VALUES(url_count), status=VALUES(status), updated_at=VALUES(updated_at)`,
		item.Language, item.Filename, item.Type, item.URLCount, item.Status, item.UpdatedAt)
	return err
}

func NewRun(trigger, target, language string) *models.GenerationRun {
	now := time.Now().UTC()
	return &models.GenerationRun{Trigger: trigger, Target: target, Language: language, Status: "running", StartedAt: &now}
}
