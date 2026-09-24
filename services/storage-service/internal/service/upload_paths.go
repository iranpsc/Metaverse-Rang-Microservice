package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

// normalizeUploadSubdir converts API-style upload paths (e.g. "/uploads/profile")
// into a subdirectory relative to the local upload base (e.g. "profile").
// An empty return value means the default mime/date layout under uploads/.
func normalizeUploadSubdir(uploadPath string) (string, error) {
	p := strings.TrimSpace(uploadPath)
	p = strings.Trim(p, "/")
	if strings.HasPrefix(p, "uploads/") {
		p = strings.TrimPrefix(p, "uploads/")
	} else if p == "uploads" {
		p = ""
	}
	p = strings.Trim(p, "/")
	if err := validateUploadSubdir(p); err != nil {
		return "", err
	}
	return p, nil
}

func validateUploadSubdir(uploadSubdir string) error {
	if uploadSubdir == "" {
		return nil
	}
	if strings.Contains(uploadSubdir, "..") {
		return fmt.Errorf("invalid upload path")
	}
	for _, part := range strings.Split(uploadSubdir, "/") {
		if part == ".." || part == "." {
			return fmt.Errorf("invalid upload path")
		}
	}
	return nil
}

func validateUploadID(uploadID string) error {
	if uploadID == "" {
		return fmt.Errorf("upload_id is required")
	}
	if !filepath.IsLocal(uploadID) {
		return fmt.Errorf("invalid upload_id")
	}
	return nil
}

// resolveChunkLocalPath maps an assembled relative path to a writable filesystem path
// under uploadBaseDir, rejecting path traversal.
func resolveChunkLocalPath(uploadBaseDir, relativePath string) (string, error) {
	return safePathUnderBase(uploadBaseDir, relativePath)
}

// resolveChunkPublicDir returns the directory path exposed to API clients.
func resolveChunkPublicDir(relativePath, uploadSubdir string, customUpload bool) string {
	if customUpload {
		dir := "/uploads/" + strings.ReplaceAll(uploadSubdir, "\\", "/")
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		return dir
	}

	pathDir := filepath.Dir(relativePath)
	pathDir = strings.ReplaceAll(pathDir, "\\", "/")
	if pathDir == "." {
		pathDir = ""
	}
	pathDir = strings.Trim(pathDir, "/")
	if pathDir == "" {
		return "uploads/"
	}
	if !strings.HasPrefix(pathDir, "uploads/") && pathDir != "uploads" {
		pathDir = "uploads/" + pathDir
	}
	return pathDir + "/"
}

func safePathUnderBase(baseDir, relativePath string) (string, error) {
	cleaned := filepath.Clean(relativePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid relative path")
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve upload base directory: %w", err)
	}

	absPath, err := filepath.Abs(filepath.Join(absBase, cleaned))
	if err != nil {
		return "", fmt.Errorf("resolve upload path: %w", err)
	}

	basePrefix := absBase + string(filepath.Separator)
	if absPath != absBase && !strings.HasPrefix(absPath, basePrefix) {
		return "", fmt.Errorf("upload path escapes base directory")
	}

	return absPath, nil
}
