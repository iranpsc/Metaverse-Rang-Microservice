package sitemap_test

import (
	"strings"
	"testing"
	"time"

	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestRenderURLSet_FullFields(t *testing.T) {
	entries := []models.URLEntry{{
		Loc:        "https://metarang.com/fa/education/watch/abc",
		LastMod:    time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		ChangeFreq: "monthly",
		Priority:   "0.8",
	}}

	xml, err := sitemap.RenderURLSet(entries)
	if err != nil {
		t.Fatal(err)
	}
	body := string(xml)
	for _, want := range []string{
		`xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`,
		"<loc>https://metarang.com/fa/education/watch/abc</loc>",
		"<lastmod>2024-06-15</lastmod>",
		"<changefreq>monthly</changefreq>",
		"<priority>0.8</priority>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "image:") {
		t.Fatal("unexpected image extension")
	}
}

func TestRenderURLSet_WithImage(t *testing.T) {
	entries := []models.URLEntry{{
		Loc:        "https://metarang.com/fa/education/category/cat",
		LastMod:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		ChangeFreq: "monthly",
		Priority:   "0.8",
		ImageLoc:   "https://admin.metarang.com/uploads/cat.jpg",
	}}

	xml, err := sitemap.RenderURLSet(entries)
	if err != nil {
		t.Fatal(err)
	}
	body := string(xml)
	for _, want := range []string{
		`xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"`,
		"<image:image>",
		"<image:loc>https://admin.metarang.com/uploads/cat.jpg</image:loc>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestRenderURLSet_Empty(t *testing.T) {
	xml, err := sitemap.RenderURLSet(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := string(xml)
	if !strings.Contains(body, "<urlset") || !strings.Contains(body, "</urlset>") {
		t.Fatalf("expected empty urlset, got:\n%s", body)
	}
}
