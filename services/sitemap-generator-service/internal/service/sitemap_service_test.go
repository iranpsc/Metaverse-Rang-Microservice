package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/service"
	"metarang/sitemap-generator-service/internal/testutil"
)

func TestGenerateAllByActiveTranslation(t *testing.T) {
	response := `{"data":[{"code":"fa"},{"code":"en"}]}`
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	dir := t.TempDir()
	templates := filepath.Join(dir, "templates.json")
	if err := os.WriteFile(templates, []byte(`{"citizens":{"fa":["https://metarang.com/fa/citizens/[code]"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writer, err := service.NewFileWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	repo := &testutil.MockSitemapRepository{
		Users:         []models.UserRow{{ID: 1, Code: "abc"}},
		Videos:        []models.VideoRow{{ID: 1, Slug: "lesson"}},
		Categories:    []models.CategoryRow{{ID: 1, Slug: "cat"}},
		SubCategories: []models.SubCategoryRow{{ID: 1, CategorySlug: "cat", Slug: "sub"}},
		Events:        []models.CalendarRow{{ID: 1}},
		Versions:      []models.CalendarRow{{ID: 1, VersionTitle: "v1"}},
	}
	svc := service.NewSitemapService(repo, writer, service.Config{TemplatesPath: templates, AdminPanelURL: server.URL, Translations: service.NewTranslationSource(server.URL, filepath.Join(dir, ".translations-cache.json"))})
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	names := []string{"citizen.xml", "user-wallet.xml", "user-lands.xml", "user-buildings.xml", "education-single-video.xml", "education-category.xml", "education-sub-category.xml", "calendar-events.xml", "calendar-versions.xml"}
	for _, lang := range []string{"en", "fa"} {
		for _, name := range names {
			data, err := os.ReadFile(filepath.Join(dir, lang, name))
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, name, err)
			}
			if !strings.Contains(string(data), "https://metarang.com/"+lang+"/") {
				t.Fatalf("%s/%s lacks language URL: %s", lang, name, data)
			}
			other := "en"
			if lang == "en" {
				other = "fa"
			}
			if strings.Contains(string(data), "https://metarang.com/"+other+"/") {
				t.Fatalf("%s/%s contains %s URL", lang, name, other)
			}
		}
	}
	mu.Lock()
	response = `{"data":[{"code":"fa"}]}`
	mu.Unlock()
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "en")); !os.IsNotExist(err) {
		t.Fatalf("inactive en directory remains: %v", err)
	}
	mu.Lock()
	response = `{"data":[{"code":"fr"}]}`
	mu.Unlock()
	if err := svc.GenerateAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fr", "citizen.xml")); err != nil {
		t.Fatalf("new language citizen sitemap: %v", err)
	}
}
