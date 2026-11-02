package repository_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/sitemap-generator-service/internal/repository"
)

func TestSitemapRepositoryLists(t *testing.T) {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		query   string
		columns []string
		row     []any
		call    func(*repository.SitemapRepository, context.Context, uint64, int) (int, time.Time, error)
	}{
		{"users", "FROM users", []string{"id", "code", "updated_at"}, []any{uint64(4), "U4", now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListUsers(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
		{"videos", "FROM videos", []string{"id", "slug", "updated_at"}, []any{uint64(4), "video", now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListVideos(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
		{"categories", "FROM video_categories", []string{"id", "slug", "image", "updated_at"}, []any{uint64(4), "cat", "cat.png", now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListCategories(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
		{"subcategories", "FROM video_sub_categories", []string{"id", "slug", "image", "slug", "updated_at"}, []any{uint64(4), "sub", "sub.png", "cat", now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListSubCategories(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
		{"events", "is_version = 0", []string{"id", "updated_at"}, []any{uint64(4), now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListEvents(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
		{"versions", "is_version = 1", []string{"id", "version_title", "updated_at"}, []any{uint64(4), "version", now}, func(r *repository.SitemapRepository, c context.Context, a uint64, n int) (int, time.Time, error) {
			v, e := r.ListVersions(c, a, n)
			if len(v) == 0 {
				return 0, time.Time{}, e
			}
			return len(v), v[0].UpdatedAt, e
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			r := repository.NewSitemapRepository(db)
			mock.ExpectQuery(tc.query).WithArgs(uint64(3), repository.DefaultPageSize).WillReturnRows(sqlmock.NewRows(tc.columns).AddRow(values(tc.row)...))
			count, updated, err := tc.call(r, context.Background(), 3, 0)
			if err != nil || count != 1 || !updated.Equal(now) {
				t.Fatalf("count=%d updated=%v err=%v", count, updated, err)
			}
			nullRow := append(append([]any{}, tc.row[:len(tc.row)-1]...), nil)
			mock.ExpectQuery(tc.query).WithArgs(uint64(4), 2).WillReturnRows(sqlmock.NewRows(tc.columns).AddRow(values(nullRow)...))
			count, updated, err = tc.call(r, context.Background(), 4, 2)
			if err != nil || count != 1 || !updated.IsZero() {
				t.Fatalf("nullable time: count=%d updated=%v err=%v", count, updated, err)
			}
			mock.ExpectQuery(tc.query).WithArgs(uint64(3), 1).WillReturnError(errors.New("database unavailable"))
			_, _, err = tc.call(r, context.Background(), 3, 1)
			if err == nil || !strings.Contains(err.Error(), "database unavailable") {
				t.Fatalf("query error: %v", err)
			}
			bad := make([]any, len(tc.row))
			copy(bad, tc.row)
			bad[0] = "not an integer"
			mock.ExpectQuery(tc.query).WithArgs(uint64(3), 1).WillReturnRows(sqlmock.NewRows(tc.columns).AddRow(values(bad)...))
			_, _, err = tc.call(r, context.Background(), 3, 1)
			if err == nil || !strings.Contains(err.Error(), "scan") {
				t.Fatalf("scan error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func values(in []any) []driver.Value {
	out := make([]driver.Value, len(in))
	for i, value := range in {
		out[i] = value
	}
	return out
}

func TestUserSitemapEligibilityQueries(t *testing.T) {
	base := "SELECT id, code, updated_at FROM users WHERE id > ? AND code IS NOT NULL AND code != '' AND code != 'hm-2000000' AND "
	tests := []struct {
		name      string
		predicate string
		call      func(*repository.SitemapRepository) error
	}{
		{"citizens require verified email", "email_verified_at IS NOT NULL", func(r *repository.SitemapRepository) error {
			_, err := r.ListUsers(context.Background(), 0, 10)
			return err
		}},
		{"wallets exclude default balances", "EXISTS ( SELECT 1 FROM wallets w WHERE w.user_id = users.id AND (w.psc != 0 OR w.irr != 0 OR w.red != 0 OR w.blue != 0 OR w.yellow != 0 OR w.satisfaction != 0.1) )", func(r *repository.SitemapRepository) error {
			_, err := r.ListWalletUsers(context.Background(), 0, 10)
			return err
		}},
		{"land owners exclude reserved user", "EXISTS (SELECT 1 FROM features f WHERE f.owner_id = users.id)", func(r *repository.SitemapRepository) error {
			_, err := r.ListLandOwners(context.Background(), 0, 10)
			return err
		}},
		{"building owners exclude reserved user", "EXISTS (SELECT 1 FROM buildings b INNER JOIN features f ON f.id = b.feature_id WHERE f.owner_id = users.id)", func(r *repository.SitemapRepository) error {
			_, err := r.ListBuildingOwners(context.Background(), 0, 10)
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery(regexp.QuoteMeta(base+tc.predicate+" ORDER BY id ASC LIMIT ?")).WithArgs(uint64(0), 10).WillReturnRows(sqlmock.NewRows([]string{"id", "code", "updated_at"}))
			if err := tc.call(repository.NewSitemapRepository(db)); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
