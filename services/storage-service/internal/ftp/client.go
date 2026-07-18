package ftp

import (
	"fmt"
	"io"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPClient struct {
	host     string
	port     string
	user     string
	password string
	baseURL  string
	conn     *ftp.ServerConn
}

func NewFTPClient(host, port, user, password, baseURL string) *FTPClient {
	return &FTPClient{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		baseURL:  baseURL,
	}
}

// Connect establishes connection to FTP server
func (c *FTPClient) Connect() error {
	addr := c.host + ":" + c.port
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("failed to connect to FTP: %w", err)
	}

	if err := conn.Login(c.user, c.password); err != nil {
		_ = conn.Quit()
		return fmt.Errorf("failed to login to FTP: %w", err)
	}

	c.conn = conn
	return nil
}

// UploadFile uploads a file to the FTP server
func (c *FTPClient) UploadFile(remotePath string, data io.Reader) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	if err := c.conn.Stor(remotePath, data); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// DownloadFile downloads a file from the FTP server
func (c *FTPClient) DownloadFile(remotePath string) (io.ReadCloser, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	resp, err := c.conn.Retr(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return resp, nil
}

// DeleteFile deletes a file from the FTP server
func (c *FTPClient) DeleteFile(remotePath string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	if err := c.conn.Delete(remotePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GenerateURL generates the full URL for a file
func (c *FTPClient) GenerateURL(remotePath string) string {
	return c.baseURL + "/" + remotePath
}

// Close closes the FTP connection
func (c *FTPClient) Close() error {
	if c.conn != nil {
		return c.conn.Quit()
	}
	return nil
}
