package ftp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MockFTPClient saves files to local filesystem for testing
type MockFTPClient struct {
	baseDir string
	baseURL string
}

func NewMockFTPClient(baseDir, baseURL string) *MockFTPClient {
	// Create base directory if it doesn't exist
	_ = os.MkdirAll(baseDir, 0755)

	return &MockFTPClient{
		baseDir: baseDir,
		baseURL: baseURL,
	}
}

// UploadFile saves a file to the local filesystem
func (c *MockFTPClient) UploadFile(remotePath string, data io.Reader) error {
	// Create full local path
	localPath := filepath.Join(c.baseDir, remotePath)

	// Create directory structure
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Copy data
	if _, err := io.Copy(file, data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// DownloadFile reads a file from the local filesystem
func (c *MockFTPClient) DownloadFile(remotePath string) (io.ReadCloser, error) {
	localPath := filepath.Join(c.baseDir, remotePath)

	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// DeleteFile deletes a file from the local filesystem
func (c *MockFTPClient) DeleteFile(remotePath string) error {
	localPath := filepath.Join(c.baseDir, remotePath)

	if err := os.Remove(localPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GenerateURL generates the full URL for a file
func (c *MockFTPClient) GenerateURL(remotePath string) string {
	return c.baseURL + "/" + remotePath
}

// Close is a no-op for mock client
func (c *MockFTPClient) Close() error {
	return nil
}
