package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/service"
	"metarang/sitemap-generator-service/internal/testutil"
)

func TestGenerateAll_WritesExpectedFiles(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := &testutil.MockSitemapRepository{
		Users:         []models.UserRow{{ID: 1, Code: "U1", UpdatedAt: now}},
		Videos:        []models.VideoRow{{ID: 1, Slug: "vid-1", UpdatedAt: now}},
		Categories:    []models.CategoryRow{{ID: 1, Slug: "cat-1", Image: "c.jpg", UpdatedAt: now}},
		SubCategories: []models.SubCategoryRow{{ID: 1, Slug: "sub-1", Image: "s.jpg", CategorySlug: "cat-1", UpdatedAt: now}},
		Events:        []models.CalendarRow{{ID: 10, UpdatedAt: now}},
		Versions:      []models.CalendarRow{{ID: 20, VersionTitle: "v1", UpdatedAt: now}},
	}
	writer := &testutil.MockWriter{}
	templatesPath := writeTempTemplates(t)

	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath:  templatesPath,
		AdminPanelURL:  "https://admin.metarang.com",
		CitizenMaxURLs: 5000,
		PageSize:       100,
	})

	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	names := writer.Filenames()
	wantPrefix := []string{
		"citizen-sitemap.xml",
		"user-wallet-sitemaps.xml",
		"user-lands-sitemap.xml",
		"user-buildings-sitemap.xml",
		"education-single-video-sitemap.xml",
		"education-category-sitemap.xml",
		"education-sub-category-sitemap.xml",
		"calendar-events-sitemap.xml",
		"calendar-versions-sitemap.xml",
	}
	for _, want := range wantPrefix {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing file %q in %v", want, names)
		}
	}

	assertContains(t, string(writer.Existing["citizen-sitemap.xml"]), "https://metarang.com/fa/citizens/U1")
	assertContains(t, string(writer.Existing["citizen-sitemap.xml"]), "https://metarang.com/en/citizens/U1")
	assertContains(t, string(writer.Existing["user-wallet-sitemaps.xml"]), "https://metarang.com/fa/citizens/U1/wallet")
	assertContains(t, string(writer.Existing["user-lands-sitemap.xml"]), "https://metarang.com/en/citizens/U1/summary")
	assertContains(t, string(writer.Existing["user-buildings-sitemap.xml"]), "https://metarang.com/fa/citizens/U1/buildings")
	assertContains(t, string(writer.Existing["education-single-video-sitemap.xml"]), "https://metarang.com/fa/education/watch/vid-1")
	assertContains(t, string(writer.Existing["education-category-sitemap.xml"]), "https://admin.metarang.com/uploads/c.jpg")
	assertContains(t, string(writer.Existing["education-sub-category-sitemap.xml"]), "https://metarang.com/fa/education/category/cat-1/sub-1")
	assertContains(t, string(writer.Existing["calendar-events-sitemap.xml"]), "https://metarang.com/fa/calendar/10")
	assertContains(t, string(writer.Existing["calendar-events-sitemap.xml"]), "<priority>0.6</priority>")
	assertContains(t, string(writer.Existing["calendar-versions-sitemap.xml"]), "https://metarang.com/en/versions/v1")
}

func TestGenerateAll_SkipsCitizenWhenNoTemplates(t *testing.T) {
	repo := &testutil.MockSitemapRepository{
		Users: []models.UserRow{{ID: 1, Code: "U1", UpdatedAt: time.Now()}},
	}
	writer := &testutil.MockWriter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "templates.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath: path,
		AdminPanelURL: "https://admin.metarang.com",
	})
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name := range writer.Existing {
		if strings.HasPrefix(name, "citizen-") {
			t.Fatalf("unexpected citizen file %q", name)
		}
	}
}

func TestGenerateAll_CitizenSplitAndCleanup(t *testing.T) {
	now := time.Now()
	users := make([]models.UserRow, 2501)
	for i := range users {
		users[i] = models.UserRow{ID: uint64(i + 1), Code: "CODE" + itoa(i), UpdatedAt: now}
	}
	repo := &testutil.MockSitemapRepository{Users: users}
	writer := &testutil.MockWriter{
		Existing: map[string][]byte{
			"citizen-sitemap-3.xml": []byte("stale"),
		},
	}
	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath:  writeTempTemplates(t),
		AdminPanelURL:  "https://admin.metarang.com",
		CitizenMaxURLs: 5000,
		PageSize:       500,
	})
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := writer.Existing["citizen-sitemap.xml"]; !ok {
		t.Fatal("missing citizen-sitemap.xml")
	}
	if _, ok := writer.Existing["citizen-sitemap-2.xml"]; !ok {
		t.Fatal("missing citizen-sitemap-2.xml")
	}
	if _, ok := writer.Existing["citizen-sitemap-3.xml"]; ok {
		t.Fatal("stale citizen-sitemap-3.xml should be removed")
	}
}

func TestGenerateAll_SingleFlight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	repo := &testutil.MockSitemapRepository{
		ListUsersFunc: func(ctx context.Context, afterID uint64, limit int) ([]models.UserRow, error) {
			once.Do(func() {
				close(started)
				<-release
			})
			return nil, nil
		},
	}
	writer := &testutil.MockWriter{}
	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath: writeTempTemplates(t),
		AdminPanelURL: "https://admin.metarang.com",
		PageSize:      10,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.GenerateAll(context.Background())
	}()

	<-started
	err := svc.GenerateAll(context.Background())
	if !errors.Is(err, service.ErrGenerationInProgress) {
		t.Fatalf("want ErrGenerationInProgress, got %v", err)
	}
	close(release)
	wg.Wait()
}

func TestStartScheduler_RunsAndStops(t *testing.T) {
	calls := 0
	repo := &testutil.MockSitemapRepository{}
	writer := &testutil.MockWriter{}
	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath: writeTempTemplates(t),
		AdminPanelURL: "https://admin.metarang.com",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		svc.StartScheduler(ctx, 20*time.Millisecond, func() {
			calls++
			if calls >= 2 {
				cancel()
			}
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 runs, got %d", calls)
	}
}

func writeTempTemplates(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "templates.json")
	content := `{
  "citizens": {
    "fa": ["https://metarang.com/fa/citizens/[code]"],
    "en": ["https://metarang.com/en/citizens/[code]"]
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("missing %q in:\n%s", want, body)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
