package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestVideoURLs(t *testing.T) {
	assertStrings(t, sitemap.VideoURLs("fa", "my-video"), []string{"https://metarang.com/fa/education/watch/my-video"})
	assertStrings(t, sitemap.VideoURLs("en", "my-video"), []string{"https://metarang.com/en/education/watch/my-video"})
}

func TestCategoryURLs(t *testing.T) {
	assertStrings(t, sitemap.CategoryURLs("fa", "programming"), []string{"https://metarang.com/fa/education/category/programming"})
	assertStrings(t, sitemap.CategoryURLs("en", "programming"), []string{"https://metarang.com/en/education/category/programming"})
}

func TestSubCategoryURLs(t *testing.T) {
	assertStrings(t, sitemap.SubCategoryURLs("fa", "programming", "go"), []string{"https://metarang.com/fa/education/category/programming/go"})
	assertStrings(t, sitemap.SubCategoryURLs("en", "programming", "go"), []string{"https://metarang.com/en/education/category/programming/go"})
}

func TestImageLoc(t *testing.T) {
	got := sitemap.ImageLoc("https://admin.metarang.com", "cat.jpg")
	want := "https://admin.metarang.com/uploads/cat.jpg"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestImageLoc_TrailingSlash(t *testing.T) {
	got := sitemap.ImageLoc("https://admin.metarang.com/", "path/img.png")
	want := "https://admin.metarang.com/uploads/path/img.png"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}
