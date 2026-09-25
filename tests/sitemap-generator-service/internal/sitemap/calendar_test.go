package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestEventURLs(t *testing.T) {
	got := sitemap.EventURLs(42)
	want := []string{
		"https://metarang.com/fa/calendar/42",
		"https://metarang.com/en/calendar/42",
	}
	assertStrings(t, got, want)
}

func TestVersionURLs(t *testing.T) {
	got := sitemap.VersionURLs("v2024")
	want := []string{
		"https://metarang.com/fa/versions/v2024",
		"https://metarang.com/en/versions/v2024",
	}
	assertStrings(t, got, want)
}
