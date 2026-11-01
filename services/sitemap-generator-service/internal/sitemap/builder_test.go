package sitemap

import (
	"strings"
	"testing"
	"time"

	"metarang/sitemap-generator-service/internal/models"
)

func TestRenderURLSetIncludesOptionalFieldsAndEscapesValues(t *testing.T) {
	body, err := RenderURLSet([]models.URLEntry{{
		Loc: "https://example.test/a?x=1&y=2", LastMod: time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600)),
		ChangeFreq: "weekly&special", Priority: "0.8", ImageLoc: "https://example.test/a&image.jpg",
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"xmlns:image=", "a?x=1&amp;y=2", "2024-01-02", "weekly&amp;special", "a&amp;image.jpg"} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered XML missing %q:\n%s", want, text)
		}
	}
	if got, _ := RenderURLSet(nil); !strings.Contains(string(got), "</urlset>") {
		t.Fatalf("empty URL set was not rendered: %s", got)
	}
}

func TestBuilderHelpers(t *testing.T) {
	if got := ExpandCitizenURLs(map[string][]string{"en": {"/en/[code]"}, "fa": {"/fa/[code]"}, "de": {"/de/[code]"}}, "X"); strings.Join(got, ",") != "/fa/X,/en/X,/de/X" {
		t.Fatalf("unexpected citizen URLs: %v", got)
	}
	if got := ExpandCitizenURLs(nil, "X"); got != nil {
		t.Fatalf("nil templates: %v", got)
	}
	if got := ChunkedFileName("citizen.xml", 0); got != "citizen.xml" {
		t.Fatal(got)
	}
	if got := ChunkedFileName("citizen.xml", 1); got != "citizen-2.xml" {
		t.Fatal(got)
	}
	if got := ChunkedFileName("citizen", 1); got != "citizen-2" {
		t.Fatal(got)
	}
	if got := CitizenFileName(1); got != "citizen-2.xml" {
		t.Fatal(got)
	}
	if got := SplitEntries([]models.URLEntry{{}, {}, {}}, 2); len(got) != 2 || len(got[0]) != 2 {
		t.Fatalf("split: %#v", got)
	}
	if got := SplitEntries(nil, 2); got != nil {
		t.Fatalf("empty split: %#v", got)
	}
	if got := ImageLoc("https://admin.test/", "/x.jpg"); got != "https://admin.test/uploads/x.jpg" {
		t.Fatal(got)
	}
	if got := VersionURLs("fa", "v1"); len(got) != 1 || !strings.Contains(got[0], "/fa/versions/v1") {
		t.Fatal(got)
	}
}
