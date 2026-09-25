package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestVideoURLs(t *testing.T) {
	got := sitemap.VideoURLs("my-video")
	want := []string{
		"https://metarang.com/fa/education/watch/my-video",
		"https://metarang.com/en/education/watch/my-video",
	}
	assertStrings(t, got, want)
}

func TestCategoryURLs(t *testing.T) {
	got := sitemap.CategoryURLs("programming")
	want := []string{
		"https://metarang.com/fa/education/category/programming",
		"https://metarang.com/en/education/category/programming",
	}
	assertStrings(t, got, want)
}

func TestSubCategoryURLs(t *testing.T) {
	got := sitemap.SubCategoryURLs("programming", "go")
	want := []string{
		"https://metarang.com/fa/education/category/programming/go",
		"https://metarang.com/en/education/category/programming/go",
	}
	assertStrings(t, got, want)
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
