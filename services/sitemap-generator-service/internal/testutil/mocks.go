package testutil

import (
	"context"

	"metarang/sitemap-generator-service/internal/models"
)

// MockSitemapRepository is a func-field mock for sitemap data access.
type MockSitemapRepository struct {
	Users         []models.UserRow
	Videos        []models.VideoRow
	Categories    []models.CategoryRow
	SubCategories []models.SubCategoryRow
	Events        []models.CalendarRow
	Versions      []models.CalendarRow

	ListUsersFunc         func(ctx context.Context, afterID uint64, limit int) ([]models.UserRow, error)
	ListVideosFunc        func(ctx context.Context, afterID uint64, limit int) ([]models.VideoRow, error)
	ListCategoriesFunc    func(ctx context.Context, afterID uint64, limit int) ([]models.CategoryRow, error)
	ListSubCategoriesFunc func(ctx context.Context, afterID uint64, limit int) ([]models.SubCategoryRow, error)
	ListEventsFunc        func(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error)
	ListVersionsFunc      func(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error)
}

func pageSlice[T any](all []T, afterID uint64, limit int, idFn func(T) uint64) []T {
	if limit <= 0 {
		limit = len(all)
	}
	var out []T
	for _, item := range all {
		if idFn(item) <= afterID {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (m *MockSitemapRepository) ListUsers(ctx context.Context, afterID uint64, limit int) ([]models.UserRow, error) {
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx, afterID, limit)
	}
	return pageSlice(m.Users, afterID, limit, func(u models.UserRow) uint64 { return u.ID }), nil
}

func (m *MockSitemapRepository) ListVideos(ctx context.Context, afterID uint64, limit int) ([]models.VideoRow, error) {
	if m.ListVideosFunc != nil {
		return m.ListVideosFunc(ctx, afterID, limit)
	}
	return pageSlice(m.Videos, afterID, limit, func(v models.VideoRow) uint64 { return v.ID }), nil
}

func (m *MockSitemapRepository) ListCategories(ctx context.Context, afterID uint64, limit int) ([]models.CategoryRow, error) {
	if m.ListCategoriesFunc != nil {
		return m.ListCategoriesFunc(ctx, afterID, limit)
	}
	return pageSlice(m.Categories, afterID, limit, func(c models.CategoryRow) uint64 { return c.ID }), nil
}

func (m *MockSitemapRepository) ListSubCategories(ctx context.Context, afterID uint64, limit int) ([]models.SubCategoryRow, error) {
	if m.ListSubCategoriesFunc != nil {
		return m.ListSubCategoriesFunc(ctx, afterID, limit)
	}
	return pageSlice(m.SubCategories, afterID, limit, func(s models.SubCategoryRow) uint64 { return s.ID }), nil
}

func (m *MockSitemapRepository) ListEvents(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error) {
	if m.ListEventsFunc != nil {
		return m.ListEventsFunc(ctx, afterID, limit)
	}
	return pageSlice(m.Events, afterID, limit, func(e models.CalendarRow) uint64 { return e.ID }), nil
}

func (m *MockSitemapRepository) ListVersions(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error) {
	if m.ListVersionsFunc != nil {
		return m.ListVersionsFunc(ctx, afterID, limit)
	}
	return pageSlice(m.Versions, afterID, limit, func(v models.CalendarRow) uint64 { return v.ID }), nil
}

// WrittenFile records a Write call.
type WrittenFile struct {
	Filename string
	Body     []byte
}

// MockWriter captures sitemap file writes and removals.
type MockWriter struct {
	Writes     []WrittenFile
	Removed    []string
	WriteFunc  func(filename string, body []byte) error
	RemoveFunc func(filename string) error
	Existing   map[string][]byte
}

func (m *MockWriter) Write(filename string, body []byte) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(filename, body)
	}
	m.Writes = append(m.Writes, WrittenFile{Filename: filename, Body: body})
	if m.Existing == nil {
		m.Existing = map[string][]byte{}
	}
	m.Existing[filename] = body
	return nil
}

func (m *MockWriter) Remove(filename string) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(filename)
	}
	m.Removed = append(m.Removed, filename)
	if m.Existing != nil {
		delete(m.Existing, filename)
	}
	return nil
}

func (m *MockWriter) Filenames() []string {
	names := make([]string, len(m.Writes))
	for i, w := range m.Writes {
		names[i] = w.Filename
	}
	return names
}
