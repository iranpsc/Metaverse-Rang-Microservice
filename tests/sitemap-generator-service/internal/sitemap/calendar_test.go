package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestEventURLs(t *testing.T) {
	assertStrings(t, sitemap.EventURLs("fa", 42), []string{"https://metarang.com/fa/calendar/42"})
	assertStrings(t, sitemap.EventURLs("en", 42), []string{"https://metarang.com/en/calendar/42"})
}

func TestVersionURLs(t *testing.T) {
	assertStrings(t, sitemap.VersionURLs("fa", "v2024"), []string{"https://metarang.com/fa/versions/v2024"})
	assertStrings(t, sitemap.VersionURLs("en", "v2024"), []string{"https://metarang.com/en/versions/v2024"})
}
