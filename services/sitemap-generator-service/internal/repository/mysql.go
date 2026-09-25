package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"metarang/sitemap-generator-service/internal/sitemap"
)

// MySQL reads sitemap rows from the shared MetaRang schema.
type MySQL struct {
	db *sql.DB
}

// NewMySQL returns a repository backed by db.
func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

// Users returns citizens in primary-key order.
func (r *MySQL) Users(ctx context.Context) ([]sitemap.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT code, updated_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []sitemap.User
	for rows.Next() {
		var code sql.NullString
		var updated sql.NullTime
		if err := rows.Scan(&code, &updated); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, sitemap.User{
			Code:       code.String,
			UpdatedAt:  updated.Time,
			HasUpdated: updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("users rows: %w", err)
	}
	return users, nil
}

// Videos returns tutorial videos.
func (r *MySQL) Videos(ctx context.Context) ([]sitemap.Video, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, updated_at FROM videos ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query videos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var videos []sitemap.Video
	for rows.Next() {
		var slug sql.NullString
		var updated sql.NullTime
		if err := rows.Scan(&slug, &updated); err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}
		videos = append(videos, sitemap.Video{
			Slug:       slug.String,
			UpdatedAt:  updated.Time,
			HasUpdated: updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("videos rows: %w", err)
	}
	return videos, nil
}

// Categories returns video categories.
func (r *MySQL) Categories(ctx context.Context) ([]sitemap.Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, image, updated_at FROM video_categories ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var categories []sitemap.Category
	for rows.Next() {
		var slug, image sql.NullString
		var updated sql.NullTime
		if err := rows.Scan(&slug, &image, &updated); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, sitemap.Category{
			Slug:       slug.String,
			Image:      image.String,
			UpdatedAt:  updated.Time,
			HasUpdated: updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories rows: %w", err)
	}
	return categories, nil
}

// SubCategories returns subcategories with the parent category slug.
func (r *MySQL) SubCategories(ctx context.Context) ([]sitemap.SubCategory, error) {
	const query = `
		SELECT sc.slug, c.slug, sc.image, sc.updated_at
		FROM video_sub_categories sc
		INNER JOIN video_categories c ON c.id = sc.video_category_id
		ORDER BY sc.id`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query subcategories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var subs []sitemap.SubCategory
	for rows.Next() {
		var slug, categorySlug, image sql.NullString
		var updated sql.NullTime
		if err := rows.Scan(&slug, &categorySlug, &image, &updated); err != nil {
			return nil, fmt.Errorf("scan subcategory: %w", err)
		}
		subs = append(subs, sitemap.SubCategory{
			Slug:         slug.String,
			CategorySlug: categorySlug.String,
			Image:        image.String,
			UpdatedAt:    updated.Time,
			HasUpdated:   updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("subcategories rows: %w", err)
	}
	return subs, nil
}

// CalendarEvents returns calendars where is_version = 0, newest start first.
func (r *MySQL) CalendarEvents(ctx context.Context) ([]sitemap.CalendarEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, updated_at
		FROM calendars
		WHERE is_version = 0
		ORDER BY starts_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query calendar events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []sitemap.CalendarEvent
	for rows.Next() {
		var id int64
		var updated sql.NullTime
		if err := rows.Scan(&id, &updated); err != nil {
			return nil, fmt.Errorf("scan calendar event: %w", err)
		}
		events = append(events, sitemap.CalendarEvent{
			ID:         id,
			UpdatedAt:  updated.Time,
			HasUpdated: updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("calendar events rows: %w", err)
	}
	return events, nil
}

// CalendarVersions returns calendars where is_version = 1, newest first.
func (r *MySQL) CalendarVersions(ctx context.Context) ([]sitemap.CalendarVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT version_title, updated_at
		FROM calendars
		WHERE is_version = 1
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query calendar versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []sitemap.CalendarVersion
	for rows.Next() {
		var title sql.NullString
		var updated sql.NullTime
		if err := rows.Scan(&title, &updated); err != nil {
			return nil, fmt.Errorf("scan calendar version: %w", err)
		}
		versions = append(versions, sitemap.CalendarVersion{
			VersionTitle: title.String,
			UpdatedAt:    updated.Time,
			HasUpdated:   updated.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("calendar versions rows: %w", err)
	}
	return versions, nil
}

// Open connects to MySQL and waits until ping succeeds or ctx is cancelled.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	backoff := time.Second
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		log.Printf("database not ready (%v); retrying", err)
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, fmt.Errorf("database unavailable: %w", ctx.Err())
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
