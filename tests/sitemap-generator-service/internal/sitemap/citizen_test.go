package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestExpandCitizenURLs_FromTemplates(t *testing.T) {
	templates := map[string][]string{
		"fa": {"https://metarang.com/fa/citizens/[code]"},
		"en": {"https://metarang.com/en/citizens/[code]"},
	}
	got := sitemap.ExpandCitizenURLs(templates, "ABC123")
	want := []string{
		"https://metarang.com/fa/citizens/ABC123",
		"https://metarang.com/en/citizens/ABC123",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d urls, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestExpandCitizenURLs_MissingTemplates(t *testing.T) {
	got := sitemap.ExpandCitizenURLs(nil, "ABC")
	if len(got) != 0 {
		t.Fatalf("expected no urls, got %v", got)
	}
}

func TestCitizenFileName(t *testing.T) {
	cases := []struct {
		index int
		want  string
	}{
		{0, "citizen-sitemap.xml"},
		{1, "citizen-sitemap-2.xml"},
		{2, "citizen-sitemap-3.xml"},
	}
	for _, tc := range cases {
		if got := sitemap.CitizenFileName(tc.index); got != tc.want {
			t.Fatalf("index %d: got %q want %q", tc.index, got, tc.want)
		}
	}
}

func TestSplitEntries(t *testing.T) {
	entries := make([]models.URLEntry, 5001)
	for i := range entries {
		entries[i].Loc = "https://example.com/" + string(rune('a'+i%26))
	}
	chunks := sitemap.SplitEntries(entries, 5000)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if len(chunks[0]) != 5000 || len(chunks[1]) != 1 {
		t.Fatalf("chunk sizes %d and %d", len(chunks[0]), len(chunks[1]))
	}
}

func TestSplitEntries_ExactLimit(t *testing.T) {
	entries := make([]models.URLEntry, 5000)
	chunks := sitemap.SplitEntries(entries, 5000)
	if len(chunks) != 1 || len(chunks[0]) != 5000 {
		t.Fatalf("got %d chunks", len(chunks))
	}
}

func TestSplitEntries_Empty(t *testing.T) {
	chunks := sitemap.SplitEntries(nil, 5000)
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %d", len(chunks))
	}
}
