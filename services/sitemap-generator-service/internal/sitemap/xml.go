package sitemap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const urlsetOpen = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
	`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1" xmlns:video="http://www.google.com/schemas/sitemap-video/1.1" xmlns:news="http://www.google.com/schemas/sitemap-news/0.9">` + "\n"

// Render writes a sitemap urlset document.
func Render(entries []Entry) []byte {
	var buf bytes.Buffer
	buf.WriteString(urlsetOpen)
	for _, entry := range entries {
		buf.WriteString("  <url>\n")
		buf.WriteString("    <loc>")
		_ = xml.EscapeText(&buf, []byte(entry.Loc))
		buf.WriteString("</loc>\n")
		if entry.HasLastMod && !entry.LastMod.IsZero() {
			buf.WriteString("    <lastmod>")
			buf.WriteString(formatLastMod(entry.LastMod))
			buf.WriteString("</lastmod>\n")
		}
		if entry.ChangeFreq != "" {
			buf.WriteString("    <changefreq>")
			_ = xml.EscapeText(&buf, []byte(entry.ChangeFreq))
			buf.WriteString("</changefreq>\n")
		}
		buf.WriteString("    <priority>")
		buf.WriteString(strconv.FormatFloat(entry.Priority, 'f', 1, 64))
		buf.WriteString("</priority>\n")
		for _, image := range entry.Images {
			if image == "" {
				continue
			}
			buf.WriteString("    <image:image>\n      <image:loc>")
			_ = xml.EscapeText(&buf, []byte(image))
			buf.WriteString("</image:loc>\n    </image:image>\n")
		}
		buf.WriteString("  </url>\n")
	}
	buf.WriteString("</urlset>\n")
	return buf.Bytes()
}

func formatLastMod(t time.Time) string {
	return t.Format("2006-01-02T15:04:05-07:00")
}

// WriteFileAtomic replaces path with data, creating parent directories as needed.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".sitemap-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp sitemap: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp sitemap: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp sitemap: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp sitemap: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename sitemap: %w", err)
	}
	cleanup = false
	return nil
}
