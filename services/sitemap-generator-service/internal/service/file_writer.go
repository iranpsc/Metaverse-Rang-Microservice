package service

import (
	"encoding/xml"
	"fmt"
	"metarang/sitemap-generator-service/internal/models"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileWriter writes sitemap XML files to an export directory.
type FileWriter struct {
	dir string
}

var generatedSitemap = regexp.MustCompile(`^(citizen|user-wallet|user-lands|user-buildings|education-single-video|education-category|education-sub-category|calendar-events|calendar-versions)([0-9]+)?\.xml$`)

func (w *FileWriter) List() ([]models.SitemapFile, error) {
	var out []models.SitemapFile
	languages, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, err
	}
	for _, lang := range languages {
		if !lang.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(w.dir, lang.Name()))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".xml") {
				continue
			}
			info, err := file.Info()
			if err != nil {
				return nil, err
			}
			data, err := os.ReadFile(filepath.Join(w.dir, lang.Name(), file.Name()))
			if err != nil {
				return nil, err
			}
			var root struct {
				XMLName  xml.Name   `xml:""`
				URLs     []struct{} `xml:"url"`
				Children []struct{} `xml:"sitemap"`
			}
			_ = xml.Unmarshal(data, &root)
			count := len(root.URLs)
			if root.XMLName.Local == "sitemapindex" {
				count = len(root.Children)
			}
			fileType := "static"
			if generatedSitemap.MatchString(file.Name()) {
				fileType = "dynamic"
			}
			out = append(out, models.SitemapFile{Language: lang.Name(), Filename: file.Name(), Type: fileType, URLCount: count, Status: "available", UpdatedAt: info.ModTime().UTC(), GeneratedAt: func() *time.Time { t := info.ModTime().UTC(); return &t }()})
		}
	}
	return out, nil
}

// NewFileWriter creates a FileWriter that writes into exportDir.
func NewFileWriter(exportDir string) (*FileWriter, error) {
	if exportDir == "" {
		return nil, fmt.Errorf("export directory is required")
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}
	return &FileWriter{dir: exportDir}, nil
}

// Write writes body to filename under the export directory.
func (w *FileWriter) Write(filename string, body []byte) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	path := filepath.Join(w.dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create sitemap dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("write temp sitemap %s: %w", filename, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename sitemap %s: %w", filename, err)
	}
	return nil
}

// Remove deletes filename from the export directory if it exists.
func (w *FileWriter) Remove(filename string) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	path := filepath.Join(w.dir, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove sitemap %s: %w", filename, err)
	}
	return nil
}

// CleanupInactive removes generated files for languages absent from the latest list.
func (w *FileWriter) CleanupInactive(active []string) error {
	keep := make(map[string]bool, len(active))
	for _, language := range active {
		keep[language] = true
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && languageCode.MatchString(name) && !keep[name] {
			files, readErr := os.ReadDir(filepath.Join(w.dir, name))
			if readErr != nil {
				return readErr
			}
			for _, file := range files {
				if !file.IsDir() && generatedSitemap.MatchString(file.Name()) {
					if err := os.Remove(filepath.Join(w.dir, name, file.Name())); err != nil && !os.IsNotExist(err) {
						return err
					}
				}
			}
			remaining, readErr := os.ReadDir(filepath.Join(w.dir, name))
			if readErr != nil {
				return readErr
			}
			if len(remaining) == 0 {
				_ = os.Remove(filepath.Join(w.dir, name))
			}
		}
	}
	return nil
}

func validateFilename(filename string) error {
	parts := strings.Split(filepath.ToSlash(filename), "/")
	if len(parts) != 2 || !languageCode.MatchString(parts[0]) || parts[1] == "" || parts[1] != filepath.Base(parts[1]) || strings.Contains(parts[1], "..") || !strings.HasSuffix(parts[1], ".xml") {
		return fmt.Errorf("invalid sitemap filename %q", filename)
	}
	return nil
}
