package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

var allowedUploadSubdirs = map[string]struct{}{
	"profile": {},
	"kyc":     {},
	"tickets": {},
	"notes":   {},
	"reports": {},
}

// normalizeUploadSubdir converts API-style upload paths (e.g. "/uploads/profile")
// into a subdirectory relative to the local upload base (e.g. "profile").
// An empty return value means the default mime/date layout under uploads/.
func normalizeUploadSubdir(uploadPath string) (string, error) {
	p := strings.TrimSpace(uploadPath)
	if p == "" {
		return "", nil
	}

	if strings.Contains(p, "..") || strings.Contains(p, "\x00") {
		return "", fmt.Errorf("invalid upload path")
	}

	p = strings.Trim(p, "/")
	if strings.HasPrefix(p, "uploads/") {
		p = strings.TrimPrefix(p, "uploads/")
	} else if p == "uploads" {
		p = ""
	}
	p = strings.Trim(p, "/")

	if p != "" {
		if strings.Contains(p, "/") || strings.Contains(p, "\\") {
			return "", fmt.Errorf("upload path not allowed")
		}
		if _, ok := allowedUploadSubdirs[p]; !ok {
			return "", fmt.Errorf("upload path not allowed")
		}
	}

	return p, nil
}

// sanitizeUploadID rejects path traversal in chunk upload session identifiers.
func sanitizeUploadID(uploadID string) error {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return fmt.Errorf("upload_id is required")
	}
	if strings.Contains(uploadID, "..") || strings.Contains(uploadID, "/") || strings.Contains(uploadID, "\\") {
		return fmt.Errorf("invalid upload_id")
	}
	return nil
}

// resolveChunkLocalPath maps an assembled relative path to a writable filesystem path.
func resolveChunkLocalPath(uploadBaseDir, relativePath string, customUpload bool) (string, error) {
	if !customUpload {
		return relativePath, nil
	}

	cleanBase := filepath.Clean(uploadBaseDir)
	localPath := filepath.Clean(filepath.Join(cleanBase, relativePath))
	if localPath != cleanBase && !strings.HasPrefix(localPath, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf("upload path escapes storage root")
	}
	return localPath, nil
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
	if !strings.HasSuffix(pathDir, "/") {
		pathDir += "/"
	}
	return pathDir
}
