// Package ftp provides FTP client implementations for the storage service.
package ftp

import "io"

// FTPClientInterface defines the interface for FTP operations
type FTPClientInterface interface {
	UploadFile(remotePath string, data io.Reader) error
	DownloadFile(remotePath string) (io.ReadCloser, error)
	DeleteFile(remotePath string) error
	GenerateURL(remotePath string) string
	Close() error
}

// Ensure both clients implement the interface
var _ FTPClientInterface = (*FTPClient)(nil)
var _ FTPClientInterface = (*MockFTPClient)(nil)
