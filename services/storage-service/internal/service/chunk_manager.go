// Package service implements business logic for the storage service.
package service

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ChunkSession represents an active chunk upload session
type ChunkSession struct {
	UploadID       string
	Filename       string
	ContentType    string
	TotalChunks    int32
	TotalSize      int64
	UploadPath     string
	ReceivedChunks map[int32]bool
	TempDir        string
	CreatedAt      time.Time
	mu             sync.RWMutex
}

// ChunkManager manages chunk upload sessions
type ChunkManager struct {
	sessions    map[string]*ChunkSession
	mu          sync.RWMutex
	baseTempDir string
}

// NewChunkManager creates a new chunk manager
func NewChunkManager(baseTempDir string) (*ChunkManager, error) {
	// Create base temp directory if it doesn't exist
	if err := os.MkdirAll(baseTempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cm := &ChunkManager{
		sessions:    make(map[string]*ChunkSession),
		baseTempDir: baseTempDir,
	}

	// Start cleanup goroutine for expired sessions
	go cm.cleanupExpiredSessions()

	return cm, nil
}

// GetOrCreateSession gets an existing session or creates a new one
func (cm *ChunkManager) GetOrCreateSession(uploadID, filename, contentType string, totalChunks int32, totalSize int64, uploadPath string) (*ChunkSession, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if session already exists
	if session, exists := cm.sessions[uploadID]; exists {
		return session, nil
	}

	// Create temp directory for this session
	tempDir := filepath.Join(cm.baseTempDir, uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session temp directory: %w", err)
	}

	// Create new session
	session := &ChunkSession{
		UploadID:       uploadID,
		Filename:       filename,
		ContentType:    contentType,
		TotalChunks:    totalChunks,
		TotalSize:      totalSize,
		UploadPath:     uploadPath,
		ReceivedChunks: make(map[int32]bool),
		TempDir:        tempDir,
		CreatedAt:      time.Now(),
	}

	cm.sessions[uploadID] = session

	return session, nil
}

// SaveChunk saves a chunk to disk
func (cm *ChunkManager) SaveChunk(session *ChunkSession, chunkIndex int32, chunkData []byte) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	// Check if chunk already exists
	if session.ReceivedChunks[chunkIndex] {
		return nil // Already received, skip
	}

	// Write chunk to temp file
	chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%d", chunkIndex))
	if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Mark chunk as received
	session.ReceivedChunks[chunkIndex] = true

	return nil
}

// GetProgress returns the upload progress percentage
func (cm *ChunkManager) GetProgress(session *ChunkSession) float64 {
	session.mu.RLock()
	defer session.mu.RUnlock()

	received := len(session.ReceivedChunks)
	total := int(session.TotalChunks)

	if total == 0 {
		return 0
	}

	return (float64(received) / float64(total)) * 100.0
}

// IsComplete checks if all chunks have been received
func (cm *ChunkManager) IsComplete(session *ChunkSession) bool {
	session.mu.RLock()
	defer session.mu.RUnlock()

	return len(session.ReceivedChunks) == int(session.TotalChunks)
}

// AssembleFile assembles all chunks into a single file
// Returns: assembledData, relativePath (like "uploads/{mime}/{YYYY-MM-DD}/{filename}"), finalFilename, error
func (cm *ChunkManager) AssembleFile(session *ChunkSession) ([]byte, string, string, error) {
	session.mu.RLock()
	defer session.mu.RUnlock()

	// Open a buffer to assemble the file
	var assembledData []byte

	// Assemble chunks in order
	for i := int32(0); i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%d", i))

		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		assembledData = append(assembledData, chunkData...)
	}

	// Create unique filename with content-based hash
	uniqueFilename := cm.createUniqueFilename(session.Filename, assembledData)

	// Determine final path with mime type and date organization
	// Format: uploads/{mime}/{YYYY-MM-DD}/{filename}
	// MIME type should be normalized (e.g., "image/jpeg" -> "image-jpeg" or just use the main type)
	mime := session.ContentType
	// Remove charset and other parameters from content type
	mime = strings.Split(mime, ";")[0]
	mime = strings.TrimSpace(mime)
	// Replace "/" with "-" for directory name (e.g., "image/jpeg" -> "image-jpeg")
	mimeDir := strings.ReplaceAll(mime, "/", "-")
	dateFolder := time.Now().Format("2006-01-02")

	var relativePath string
	if session.UploadPath != "" {
		relativePath = filepath.Join(session.UploadPath, uniqueFilename)
	} else {
		// Format: uploads/{mime}/{YYYY-MM-DD}/{filename}
		relativePath = filepath.Join("uploads", mimeDir, dateFolder, uniqueFilename)
	}

	return assembledData, relativePath, uniqueFilename, nil
}

// CleanupSession removes a session and its temporary files
func (cm *ChunkManager) CleanupSession(uploadID string) error {
	cm.mu.Lock()
	session, exists := cm.sessions[uploadID]
	if !exists {
		cm.mu.Unlock()
		return nil
	}
	delete(cm.sessions, uploadID)
	cm.mu.Unlock()

	// Remove temp directory
	if err := os.RemoveAll(session.TempDir); err != nil {
		return fmt.Errorf("failed to remove temp directory: %w", err)
	}

	return nil
}

// createUniqueFilename creates a unique filename with MD5 hash based on file content
// Format: {hash}.{ext} (e.g., "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6.jpg")
func (cm *ChunkManager) createUniqueFilename(originalFilename string, fileData []byte) string {
	ext := filepath.Ext(originalFilename)

	// Create MD5 hash of file content for unique identification
	hash := md5.New()
	hash.Write(fileData)
	hashSum := hash.Sum(nil)

	// Use first 16 characters of hash (32 hex characters = 16 bytes)
	hashStr := fmt.Sprintf("%x", hashSum)

	// If no extension, use empty string
	if ext == "" {
		return hashStr
	}

	return fmt.Sprintf("%s%s", hashStr, ext)
}

// cleanupExpiredSessions removes sessions older than 24 hours
func (cm *ChunkManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.Lock()
		now := time.Now()
		for uploadID, session := range cm.sessions {
			if now.Sub(session.CreatedAt) > 24*time.Hour {
				_ = os.RemoveAll(session.TempDir)
				delete(cm.sessions, uploadID)
			}
		}
		cm.mu.Unlock()
	}
}
