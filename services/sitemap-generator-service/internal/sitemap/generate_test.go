package sitemap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureSource struct {
	users    []User
	videos   []Video
	cats     []Category
	subs     []SubCategory
	events   []CalendarEvent
	versions []CalendarVersion
}

func (f fixtureSource) Users(context.Context) ([]User, error)          { return f.users, nil }
func (f fixtureSource) Videos(context.Context) ([]Video, error)        { return f.videos, nil }
func (f fixtureSource) Categories(context.Context) ([]Category, error) { return f.cats, nil }
func (f fixtureSource) SubCategories(context.Context) ([]SubCategory, error) {
	return f.subs, nil
}
func (f fixtureSource) CalendarEvents(context.Context) ([]CalendarEvent, error) {
	return f.events, nil
}
func (f fixtureSource) CalendarVersions(context.Context) ([]CalendarVersion, error) {
	return f.versions, nil
}

func TestGenerateFollowsSpec(t *testing.T) {
	dir := t.TempDir()
	updated := time.Date(2026, 3, 1, 12, 30, 0, 0, time.FixedZone("IRST", 3*3600+30*60))
	templates := Templates{
		HasCitizens: true,
		Citizens: []LanguageTemplates{
			{Language: "fa", URLs: []string{"https://metarang.com/fa/citizens/[code]"}},
			{Language: "en", URLs: []string{"https://metarang.com/en/citizens/[code]"}},
		},
	}
	src := fixtureSource{
		users: []User{
			{Code: "hm-1000", UpdatedAt: updated, HasUpdated: true},
			{Code: "  ", UpdatedAt: updated, HasUpdated: true},
		},
		videos: []Video{{Slug: "intro", UpdatedAt: updated, HasUpdated: true}},
		cats: []Category{{
			Slug: "basics", Image: "cat.png", UpdatedAt: updated, HasUpdated: true,
		}},
		subs: []SubCategory{{
			Slug: "start", CategorySlug: "basics", Image: "sub.png", UpdatedAt: updated, HasUpdated: true,
		}},
		events:   []CalendarEvent{{ID: 42, UpdatedAt: updated, HasUpdated: true}},
		versions: []CalendarVersion{{VersionTitle: "v1", UpdatedAt: updated, HasUpdated: true}},
	}

	result, err := Generate(context.Background(), src, Options{
		OutputDir:      dir,
		AdminPanelURL:  "https://admin.rgb.irpsc.com/",
		Templates:      &templates,
		MaxURLsPerFile: 5000,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	citizen := string(mustRead(t, filepath.Join(dir, "citizen-sitemap.xml")))
	assertContains(t, citizen, "https://metarang.com/fa/citizens/hm-1000")
	assertContains(t, citizen, "https://metarang.com/en/citizens/hm-1000")
	assertContains(t, citizen, "<changefreq>daily</changefreq>")
	assertContains(t, citizen, "<priority>0.8</priority>")
	assertContains(t, citizen, "<lastmod>2026-03-01T12:30:00+03:30</lastmod>")
	if strings.Contains(citizen, "hm-1000/hm-1000") || strings.Count(citizen, "<loc>") != 2 {
		t.Fatalf("unexpected citizen urls:\n%s", citizen)
	}

	videos := string(mustRead(t, filepath.Join(dir, fileVideos)))
	assertContains(t, videos, "https://rgb.irpsc.com/fa/education/watch/intro")
	assertContains(t, videos, "https://rgb.irpsc.com/en/education/watch/intro")
	assertContains(t, videos, "<changefreq>monthly</changefreq>")

	cats := string(mustRead(t, filepath.Join(dir, fileCategories)))
	assertContains(t, cats, "https://rgb.irpsc.com/fa/education/category/basics")
	assertContains(t, cats, "https://admin.rgb.irpsc.com/uploads/cat.png")

	subs := string(mustRead(t, filepath.Join(dir, fileSubCategories)))
	assertContains(t, subs, "https://rgb.irpsc.com/en/education/category/basics/start")
	assertContains(t, subs, "https://admin.rgb.irpsc.com/uploads/sub.png")

	events := string(mustRead(t, filepath.Join(dir, fileCalendarEvents)))
	assertContains(t, events, "https://metarang.com/fa/calendar/42")
	assertContains(t, events, "https://metarang.com/en/calendar/42")
	assertContains(t, events, "<priority>0.6</priority>")

	versions := string(mustRead(t, filepath.Join(dir, fileCalendarVersions)))
	assertContains(t, versions, "https://metarang.com/fa/versions/v1")
	assertContains(t, versions, "https://metarang.com/en/versions/v1")

	if len(result.Files) != 6 {
		t.Fatalf("files = %#v", result.Files)
	}
}

func TestCitizenFilesSplitAtLimitAndDropStale(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "citizen-sitemap-9.xml")
	if err := os.WriteFile(stale, []byte("<urlset></urlset>"), 0o644); err != nil {
		t.Fatal(err)
	}
	templates := Templates{
		HasCitizens: true,
		Citizens: []LanguageTemplates{
			{Language: "fa", URLs: []string{"https://metarang.com/fa/citizens/[code]"}},
		},
	}
	users := make([]User, 0, 4)
	for _, code := range []string{"a", "b", "c", "d"} {
		users = append(users, User{Code: code, HasUpdated: false})
	}
	_, err := Generate(context.Background(), fixtureSource{users: users}, Options{
		OutputDir:      dir,
		Templates:      &templates,
		MaxURLsPerFile: 3,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "citizen-sitemap.xml")); err != nil {
		t.Fatal(err)
	}
	second := string(mustRead(t, filepath.Join(dir, "citizen-sitemap-2.xml")))
	assertContains(t, second, "https://metarang.com/fa/citizens/d")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file still present: %v", err)
	}
}

func TestMissingCitizensKeyLeavesCitizenFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "citizen-sitemap.xml")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	templates := Templates{HasCitizens: false}
	if _, err := Generate(context.Background(), fixtureSource{}, Options{
		OutputDir: dir,
		Templates: &templates,
	}); err != nil {
		t.Fatal(err)
	}
	body := string(mustRead(t, path))
	if body != "keep" {
		t.Fatalf("citizen file changed: %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, fileVideos)); err != nil {
		t.Fatal(err)
	}
}

func TestParseTemplatesPreservesLanguageOrder(t *testing.T) {
	raw := []byte(`{"citizens":{"en":["https://metarang.com/en/citizens/[code]"],"fa":["https://metarang.com/fa/citizens/[code]"]}}`)
	parsed, err := ParseTemplates(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.HasCitizens || len(parsed.Citizens) != 2 {
		t.Fatalf("parsed = %#v", parsed)
	}
	if parsed.Citizens[0].Language != "en" || parsed.Citizens[1].Language != "fa" {
		t.Fatalf("order = %#v", parsed.Citizens)
	}
	urls := ExpandCitizenURLs("hm-1", parsed)
	if len(urls) != 2 || urls[0] != "https://metarang.com/en/citizens/hm-1" {
		t.Fatalf("urls = %#v", urls)
	}
}

func TestShippedTemplatesMatchServiceRoot(t *testing.T) {
	path := filepath.Join("..", "..", "templates.json")
	parsed, err := LoadTemplates(path)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.HasCitizens || len(parsed.Citizens) < 2 {
		t.Fatalf("shipped templates = %#v", parsed)
	}
	for _, lang := range parsed.Citizens {
		if len(lang.URLs) == 0 {
			t.Fatalf("language %s has no urls", lang.Language)
		}
		for _, url := range lang.URLs {
			if !strings.Contains(url, "[code]") {
				t.Fatalf("template %q missing [code]", url)
			}
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertContains(t *testing.T, body, needle string) {
	t.Helper()
	if !strings.Contains(body, needle) {
		t.Fatalf("missing %q in:\n%s", needle, body)
	}
}
