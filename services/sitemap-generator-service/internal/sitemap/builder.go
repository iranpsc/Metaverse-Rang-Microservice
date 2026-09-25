package sitemap

import (
	"fmt"
	"html"
	"strings"
	"time"

	"metarang/sitemap-generator-service/internal/models"
)

const (
	SiteBaseURL    = "https://metarang.com"
	CitizenMaxURLs = 5000
	xmlnsSitemap   = "http://www.sitemaps.org/schemas/sitemap/0.9"
	xmlnsImage     = "http://www.google.com/schemas/sitemap-image/1.1"
)

// RenderURLSet encodes entries as a sitemap XML document.
func RenderURLSet(entries []models.URLEntry) ([]byte, error) {
	hasImage := false
	for _, e := range entries {
		if e.ImageLoc != "" {
			hasImage = true
			break
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteByte('\n')
	b.WriteString(`<urlset xmlns="`)
	b.WriteString(xmlnsSitemap)
	b.WriteByte('"')
	if hasImage {
		b.WriteString(` xmlns:image="`)
		b.WriteString(xmlnsImage)
		b.WriteByte('"')
	}
	b.WriteString(">\n")

	for _, e := range entries {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>")
		b.WriteString(html.EscapeString(e.Loc))
		b.WriteString("</loc>\n")
		if lm := formatLastMod(e.LastMod); lm != "" {
			b.WriteString("    <lastmod>")
			b.WriteString(lm)
			b.WriteString("</lastmod>\n")
		}
		if e.ChangeFreq != "" {
			b.WriteString("    <changefreq>")
			b.WriteString(html.EscapeString(e.ChangeFreq))
			b.WriteString("</changefreq>\n")
		}
		if e.Priority != "" {
			b.WriteString("    <priority>")
			b.WriteString(html.EscapeString(e.Priority))
			b.WriteString("</priority>\n")
		}
		if e.ImageLoc != "" {
			b.WriteString("    <image:image>\n")
			b.WriteString("      <image:loc>")
			b.WriteString(html.EscapeString(e.ImageLoc))
			b.WriteString("</image:loc>\n")
			b.WriteString("    </image:image>\n")
		}
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	return []byte(b.String()), nil
}

func formatLastMod(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// ExpandCitizenURLs replaces [code] in each language template.
func ExpandCitizenURLs(templates map[string][]string, code string) []string {
	if len(templates) == 0 {
		return nil
	}
	langs := make([]string, 0, len(templates))
	for lang := range templates {
		langs = append(langs, lang)
	}
	ordered := make([]string, 0, len(langs))
	for _, preferred := range []string{"fa", "en"} {
		if _, ok := templates[preferred]; ok {
			ordered = append(ordered, preferred)
		}
	}
	for _, lang := range langs {
		if lang == "fa" || lang == "en" {
			continue
		}
		ordered = append(ordered, lang)
	}

	var urls []string
	for _, lang := range ordered {
		for _, tmpl := range templates[lang] {
			urls = append(urls, strings.ReplaceAll(tmpl, "[code]", code))
		}
	}
	return urls
}

// CitizenFileName returns the public filename for citizen chunk index (0-based).
func CitizenFileName(index int) string {
	return ChunkedFileName("citizen-sitemap.xml", index)
}

// ChunkedFileName returns base for index 0, or base-N.xml for index >= 1.
func ChunkedFileName(base string, index int) string {
	if index <= 0 {
		return base
	}
	if len(base) > 4 && base[len(base)-4:] == ".xml" {
		return fmt.Sprintf("%s-%d.xml", base[:len(base)-4], index+1)
	}
	return fmt.Sprintf("%s-%d", base, index+1)
}

// SplitEntries chunks entries into slices of at most maxPerFile.
func SplitEntries(entries []models.URLEntry, maxPerFile int) [][]models.URLEntry {
	if len(entries) == 0 {
		return nil
	}
	if maxPerFile <= 0 {
		maxPerFile = CitizenMaxURLs
	}
	var chunks [][]models.URLEntry
	for i := 0; i < len(entries); i += maxPerFile {
		end := i + maxPerFile
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}
	return chunks
}

// VideoURLs returns fa/en watch URLs for a video slug.
func VideoURLs(slug string) []string {
	return []string{
		SiteBaseURL + "/fa/education/watch/" + slug,
		SiteBaseURL + "/en/education/watch/" + slug,
	}
}

// CategoryURLs returns fa/en category URLs.
func CategoryURLs(slug string) []string {
	return []string{
		SiteBaseURL + "/fa/education/category/" + slug,
		SiteBaseURL + "/en/education/category/" + slug,
	}
}

// SubCategoryURLs returns fa/en sub-category URLs.
func SubCategoryURLs(categorySlug, subSlug string) []string {
	return []string{
		SiteBaseURL + "/fa/education/category/" + categorySlug + "/" + subSlug,
		SiteBaseURL + "/en/education/category/" + categorySlug + "/" + subSlug,
	}
}

// ImageLoc builds an admin panel uploads URL.
func ImageLoc(adminBase, image string) string {
	base := strings.TrimRight(adminBase, "/")
	image = strings.TrimLeft(image, "/")
	return base + "/uploads/" + image
}

// EventURLs returns fa/en calendar event URLs.
func EventURLs(id uint64) []string {
	return []string{
		fmt.Sprintf("%s/fa/calendar/%d", SiteBaseURL, id),
		fmt.Sprintf("%s/en/calendar/%d", SiteBaseURL, id),
	}
}

// VersionURLs returns fa/en calendar version URLs.
func VersionURLs(versionTitle string) []string {
	return []string{
		SiteBaseURL + "/fa/versions/" + versionTitle,
		SiteBaseURL + "/en/versions/" + versionTitle,
	}
}

// UserWalletURLs returns fa/en citizen wallet page URLs.
func UserWalletURLs(code string) []string {
	return citizenPathURLs(code, "wallet")
}

// UserLandsURLs returns fa/en citizen summary/lands page URLs.
func UserLandsURLs(code string) []string {
	return citizenPathURLs(code, "summary")
}

// UserBuildingsURLs returns fa/en citizen buildings page URLs.
func UserBuildingsURLs(code string) []string {
	return citizenPathURLs(code, "buildings")
}

func citizenPathURLs(code, path string) []string {
	return []string{
		SiteBaseURL + "/fa/citizens/" + code + "/" + path,
		SiteBaseURL + "/en/citizens/" + code + "/" + path,
	}
}
