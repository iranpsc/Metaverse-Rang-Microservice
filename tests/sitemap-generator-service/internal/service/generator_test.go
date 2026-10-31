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

type fixedTranslations []string

func (languages fixedTranslations) Active(context.Context) ([]string, error) {
	return languages, nil
}

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
		Translations:   fixedTranslations{"fa", "en"},
		CitizenMaxURLs: 5000,
		PageSize:       100,
	})

	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	names := writer.Filenames()
	wantPrefix := []string{
		"citizen.xml",
		"user-wallet.xml",
		"user-lands.xml",
		"user-buildings.xml",
		"education-single-video.xml",
		"education-category.xml",
		"education-sub-category.xml",
		"calendar-events.xml",
		"calendar-versions.xml",
	}
	for _, language := range []string{"fa", "en"} {
		for _, base := range wantPrefix {
			want := filepath.Join(language, base)
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
	}

	assertContains(t, string(writer.Existing[filepath.Join("fa", "citizen.xml")]), "https://metarang.com/fa/citizens/U1")
	assertContains(t, string(writer.Existing[filepath.Join("en", "citizen.xml")]), "https://metarang.com/en/citizens/U1")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "user-wallet.xml")]), "https://metarang.com/fa/citizens/U1/wallet")
	assertContains(t, string(writer.Existing[filepath.Join("en", "user-lands.xml")]), "https://metarang.com/en/citizens/U1/summary")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "user-buildings.xml")]), "https://metarang.com/fa/citizens/U1/buildings")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "education-single-video.xml")]), "https://metarang.com/fa/education/watch/vid-1")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "education-category.xml")]), "https://admin.metarang.com/uploads/c.jpg")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "education-sub-category.xml")]), "https://metarang.com/fa/education/category/cat-1/sub-1")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "calendar-events.xml")]), "https://metarang.com/fa/calendar/10")
	assertContains(t, string(writer.Existing[filepath.Join("fa", "calendar-events.xml")]), "<priority>0.6</priority>")
	assertContains(t, string(writer.Existing[filepath.Join("en", "calendar-versions.xml")]), "https://metarang.com/en/versions/v1")
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
		Translations:  fixedTranslations{"fa"},
	})
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name := range writer.Existing {
		if strings.HasPrefix(filepath.Base(name), "citizen-") {
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
			filepath.Join("fa", "citizen-3.xml"): []byte("stale"),
		},
	}
	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath:  writeTempTemplates(t),
		AdminPanelURL:  "https://admin.metarang.com",
		Translations:   fixedTranslations{"fa"},
		CitizenMaxURLs: 2500,
		PageSize:       500,
	})
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := writer.Existing[filepath.Join("fa", "citizen.xml")]; !ok {
		t.Fatal("missing citizen.xml")
	}
	if _, ok := writer.Existing[filepath.Join("fa", "citizen-2.xml")]; !ok {
		t.Fatal("missing citizen-2.xml")
	}
	if _, ok := writer.Existing[filepath.Join("fa", "citizen-3.xml")]; ok {
		t.Fatal("stale citizen-3.xml should be removed")
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
		Translations:  fixedTranslations{"fa"},
		PageSize:      10,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.GenerateAll(context.Background())
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		wg.Wait()
		t.Fatal("first generation did not reach repository")
	}
	err := svc.GenerateAll(context.Background())
	close(release)
	wg.Wait()
	if !errors.Is(err, service.ErrGenerationInProgress) {
		t.Fatalf("want ErrGenerationInProgress, got %v", err)
	}
}

func TestStartScheduler_RunsAndStops(t *testing.T) {
	calls := 0
	repo := &testutil.MockSitemapRepository{}
	writer := &testutil.MockWriter{}
	svc := service.NewSitemapService(repo, writer, service.Config{
		TemplatesPath: writeTempTemplates(t),
		AdminPanelURL: "https://admin.metarang.com",
		Translations:  fixedTranslations{"fa"},
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
