package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileWriter writes sitemap XML files to an export directory.
type FileWriter struct {
	dir string
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

func validateFilename(filename string) error {
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid sitemap filename %q", filename)
	}
	return nil
}
