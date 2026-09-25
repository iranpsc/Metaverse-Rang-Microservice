// Package repository provides data access for sitemap source tables.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/sitemap-generator-service/internal/models"
)

// DefaultPageSize is the default keyset page size for sitemap queries.
const DefaultPageSize = 1000

// SitemapRepository reads sitemap source data from MySQL.
type SitemapRepository struct {
	db *sql.DB
}

// NewSitemapRepository creates a SitemapRepository.
func NewSitemapRepository(db *sql.DB) *SitemapRepository {
	return &SitemapRepository{db: db}
}

// ListUsers returns users with id > afterID ordered by id, limited to limit rows.
func (r *SitemapRepository) ListUsers(ctx context.Context, afterID uint64, limit int) ([]models.UserRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, code, updated_at
		FROM users
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.UserRow
	for rows.Next() {
		var u models.UserRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Code, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if updatedAt.Valid {
			u.UpdatedAt = updatedAt.Time
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListVideos returns videos with id > afterID.
func (r *SitemapRepository) ListVideos(ctx context.Context, afterID uint64, limit int) ([]models.VideoRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, updated_at
		FROM videos
		WHERE id > ? AND slug IS NOT NULL AND slug != ''
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.VideoRow
	for rows.Next() {
		var v models.VideoRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.Slug, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}
		if updatedAt.Valid {
			v.UpdatedAt = updatedAt.Time
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListCategories returns categories with id > afterID.
func (r *SitemapRepository) ListCategories(ctx context.Context, afterID uint64, limit int) ([]models.CategoryRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, COALESCE(image, ''), updated_at
		FROM video_categories
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.CategoryRow
	for rows.Next() {
		var c models.CategoryRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.Slug, &c.Image, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		if updatedAt.Valid {
			c.UpdatedAt = updatedAt.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListSubCategories returns sub-categories with id > afterID joined to parent slug.
func (r *SitemapRepository) ListSubCategories(ctx context.Context, afterID uint64, limit int) ([]models.SubCategoryRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT vsc.id, vsc.slug, COALESCE(vsc.image, ''), vc.slug, vsc.updated_at
		FROM video_sub_categories vsc
		INNER JOIN video_categories vc ON vc.id = vsc.video_category_id
		WHERE vsc.id > ?
		ORDER BY vsc.id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sub_categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.SubCategoryRow
	for rows.Next() {
		var sc models.SubCategoryRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&sc.ID, &sc.Slug, &sc.Image, &sc.CategorySlug, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan sub_category: %w", err)
		}
		if updatedAt.Valid {
			sc.UpdatedAt = updatedAt.Time
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ListEvents returns calendar events (is_version = 0) with id > afterID.
func (r *SitemapRepository) ListEvents(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, updated_at
		FROM calendars
		WHERE is_version = 0 AND id > ?
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.CalendarRow
	for rows.Next() {
		var e models.CalendarRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&e.ID, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if updatedAt.Valid {
			e.UpdatedAt = updatedAt.Time
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListVersions returns calendar versions (is_version = 1) with id > afterID.
func (r *SitemapRepository) ListVersions(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(version_title, ''), updated_at
		FROM calendars
		WHERE is_version = 1 AND id > ?
		ORDER BY id ASC
		LIMIT ?
	`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.CalendarRow
	for rows.Next() {
		var v models.CalendarRow
		var updatedAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.VersionTitle, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		if updatedAt.Valid {
			v.UpdatedAt = updatedAt.Time
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
