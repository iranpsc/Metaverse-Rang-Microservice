package service

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"metarang/storage-service/internal/ftp"
)

type StorageService struct {
	ftpClient     ftp.FTPClientInterface
	chunkManager  *ChunkManager
	uploadBaseDir string
}

func NewStorageService(ftpClient ftp.FTPClientInterface, chunkManager *ChunkManager, uploadBaseDir string) *StorageService {
	if uploadBaseDir == "" {
		uploadBaseDir = "uploads"
	}
	return &StorageService{
		ftpClient:     ftpClient,
		chunkManager:  chunkManager,
		uploadBaseDir: uploadBaseDir,
	}
}

// UploadFile uploads a file to FTP server
func (s *StorageService) UploadFile(filename, contentType string, data []byte, uploadPath string) (string, error) {
	// Generate unique filename
	timestamp := time.Now().Unix()
	ext := filepath.Ext(filename)
	uniqueFilename := fmt.Sprintf("%d_%s%s", timestamp, filename[:len(filename)-len(ext)], ext)

	// Build remote path
	remotePath := filepath.Join(uploadPath, uniqueFilename)

	// Upload to FTP
	reader := bytes.NewReader(data)
	if err := s.ftpClient.UploadFile(remotePath, reader); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Generate URL
	url := s.ftpClient.GenerateURL(remotePath)

	return url, nil
}

// GetFile retrieves a file from local chunk uploads or FTP.
func (s *StorageService) GetFile(filePath string) ([]byte, string, error) {
	if data, contentType, ok := s.readLocalUploadFile(filePath); ok {
		return data, contentType, nil
	}

	// Download from FTP
	reader, err := s.ftpClient.DownloadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	return data, contentTypeForPath(filePath), nil
}

// readLocalUploadFile reads a file written by HandleChunkUpload from the local uploads directory.
func (s *StorageService) readLocalUploadFile(filePath string) ([]byte, string, bool) {
	for _, candidate := range localUploadPathCandidates(s.uploadBaseDir, filePath) {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		return data, contentTypeForPath(candidate), true
	}
	return nil, "", false
}

func localUploadPathCandidates(uploadBaseDir, filePath string) []string {
	filePath = strings.TrimPrefix(strings.ReplaceAll(filePath, "\\", "/"), "/")
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(filepath.Join(uploadBaseDir, filePath))
	// Laravel chunk uploads use storage/app/upload/...; microservice uses uploads/...
	if strings.HasPrefix(filePath, "upload/") && !strings.HasPrefix(filePath, "uploads/") {
		add(filepath.Join(uploadBaseDir, "uploads", strings.TrimPrefix(filePath, "upload/")))
	}
	if !strings.HasPrefix(filePath, "uploads/") {
		add(filepath.Join(uploadBaseDir, "uploads", filePath))
	}
	return out
}

func contentTypeForPath(filePath string) string {
	contentType := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".pdf":
		contentType = "application/pdf"
	case ".mp4":
		contentType = "video/mp4"
	case ".webm":
		contentType = "video/webm"
	case ".mov":
		contentType = "video/quicktime"
	}
	return contentType
}

// DeleteFile deletes a file from FTP server
func (s *StorageService) DeleteFile(filePath string) error {
	return s.ftpClient.DeleteFile(filePath)
}

// HandleChunkUpload processes a chunk upload
// Returns: isFinished, progress, filePath (relative path like "uploads/mime/date/"), finalFilename, mimeType, error
func (s *StorageService) HandleChunkUpload(uploadID, filename, contentType string, chunkData []byte, chunkIndex, totalChunks int32, totalSize int64, uploadPath string) (bool, float64, string, string, string, error) {
	uploadSubdir := normalizeUploadSubdir(uploadPath)
	customUpload := uploadSubdir != ""

	// Get or create session
	session, err := s.chunkManager.GetOrCreateSession(uploadID, filename, contentType, totalChunks, totalSize, uploadSubdir)
	if err != nil {
		return false, 0, "", "", "", fmt.Errorf("failed to create session: %w", err)
	}

	// Save the chunk
	if err := s.chunkManager.SaveChunk(session, chunkIndex, chunkData); err != nil {
		return false, 0, "", "", "", fmt.Errorf("failed to save chunk: %w", err)
	}

	// Get progress
	progress := s.chunkManager.GetProgress(session)

	// Check if upload is complete
	if !s.chunkManager.IsComplete(session) {
		return false, progress, "", "", "", nil
	}

	// Assemble file
	assembledData, relativePath, finalFilename, err := s.chunkManager.AssembleFile(session)
	if err != nil {
		_ = s.chunkManager.CleanupSession(uploadID)
		return false, 0, "", "", "", fmt.Errorf("failed to assemble file: %w", err)
	}

	localPath := resolveChunkLocalPath(s.uploadBaseDir, relativePath, customUpload)
	localDir := filepath.Dir(localPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(localDir, 0755); err != nil {
		_ = s.chunkManager.CleanupSession(uploadID)
		return false, 0, "", "", "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Write file to local storage
	if err := os.WriteFile(localPath, assembledData, 0644); err != nil {
		_ = s.chunkManager.CleanupSession(uploadID)
		return false, 0, "", "", "", fmt.Errorf("failed to save file: %w", err)
	}

	// Extract mime type (clean it up - remove charset if present)
	mimeType := strings.Split(contentType, ";")[0]
	mimeType = strings.TrimSpace(mimeType)

	// Return directory path for API consumers (e.g. "/uploads/profile/" or "uploads/image-jpeg/2024-01-01/")
	pathDir := resolveChunkPublicDir(relativePath, uploadSubdir, customUpload)

	// Cleanup session
	_ = s.chunkManager.CleanupSession(uploadID)

	return true, 100.0, pathDir, finalFilename, mimeType, nil
}
