package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarang/storage-service/internal/ftp"
	"metarang/storage-service/internal/service"
)

func TestHandleChunkUpload_CustomUploadPaths(t *testing.T) {
	tests := []struct {
		name          string
		uploadPath    string
		wantPublicDir string
		wantLocalSub  string
	}{
		{name: "profile API path", uploadPath: "/uploads/profile", wantPublicDir: "/uploads/profile/", wantLocalSub: "profile"},
		{name: "profile without leading slash", uploadPath: "uploads/profile", wantPublicDir: "/uploads/profile/", wantLocalSub: "profile"},
		{name: "kyc path", uploadPath: "/uploads/kyc", wantPublicDir: "/uploads/kyc/", wantLocalSub: "kyc"},
		{name: "subdir only", uploadPath: "profile", wantPublicDir: "/uploads/profile/", wantLocalSub: "profile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			uploadBase := filepath.Join(tempDir, "uploads")
			chunkManager, err := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
			if err != nil {
				t.Fatalf("NewChunkManager: %v", err)
			}

			mockFTP := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://localhost/uploads")
			svc := service.NewStorageService(mockFTP, chunkManager, uploadBase)

			finished, _, publicDir, filename, _, err := svc.HandleChunkUpload(
				"upload-"+tt.name,
				"photo.jpg",
				"image/jpeg",
				[]byte("fake-jpeg-data"),
				0,
				1,
				int64(len("fake-jpeg-data")),
				tt.uploadPath,
			)
			if err != nil {
				t.Fatalf("HandleChunkUpload: %v", err)
			}
			if !finished {
				t.Fatal("expected upload to finish in one chunk")
			}
			if publicDir != tt.wantPublicDir {
				t.Fatalf("publicDir = %q, want %q", publicDir, tt.wantPublicDir)
			}

			localFile := filepath.Join(uploadBase, tt.wantLocalSub, filename)
			if _, err := os.Stat(localFile); err != nil {
				t.Fatalf("expected file at %s: %v", localFile, err)
			}
		})
	}
}

func TestHandleChunkUpload_DefaultUploadLayout(t *testing.T) {
	tempDir := t.TempDir()
	uploadBase := filepath.Join(tempDir, "data", "uploads")
	chunkManager, err := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
	if err != nil {
		t.Fatalf("NewChunkManager: %v", err)
	}

	mockFTP := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://localhost/uploads")
	svc := service.NewStorageService(mockFTP, chunkManager, uploadBase)

	finished, _, publicDir, filename, _, err := svc.HandleChunkUpload(
		"default-layout",
		"photo.jpg",
		"image/jpeg",
		[]byte("fake-jpeg-data"),
		0,
		1,
		int64(len("fake-jpeg-data")),
		"",
	)
	if err != nil {
		t.Fatalf("HandleChunkUpload: %v", err)
	}
	if !finished {
		t.Fatal("expected upload to finish in one chunk")
	}
	if !strings.HasPrefix(publicDir, "uploads/image-jpeg/") || !strings.HasSuffix(publicDir, "/") {
		t.Fatalf("publicDir = %q, want uploads/image-jpeg/<date>/", publicDir)
	}
	if filename == "" {
		t.Fatal("expected generated filename")
	}

	dateFolder := strings.TrimSuffix(strings.TrimPrefix(publicDir, "uploads/image-jpeg/"), "/")
	localFile := filepath.Join(uploadBase, "image-jpeg", dateFolder, filename)
	if _, err := os.Stat(localFile); err != nil {
		t.Fatalf("expected file at %s: %v", localFile, err)
	}

	got, contentType, err := svc.GetFile(strings.TrimSuffix(publicDir, "/") + "/" + filename)
	if err != nil {
		t.Fatalf("GetFile public path: %v", err)
	}
	if contentType != "image/jpeg" || string(got) != "fake-jpeg-data" {
		t.Fatalf("GetFile: type=%q data=%q", contentType, got)
	}
}

func TestHandleChunkUpload_RejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	uploadBase := filepath.Join(tempDir, "uploads")
	chunkManager, err := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
	if err != nil {
		t.Fatalf("NewChunkManager: %v", err)
	}

	mockFTP := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://localhost/uploads")
	svc := service.NewStorageService(mockFTP, chunkManager, uploadBase)

	_, _, _, _, _, err = svc.HandleChunkUpload(
		"traversal-test",
		"photo.jpg",
		"image/jpeg",
		[]byte("fake-jpeg-data"),
		0,
		1,
		int64(len("fake-jpeg-data")),
		"/uploads/../etc",
	)
	if err == nil {
		t.Fatal("expected path traversal upload path to be rejected")
	}
}
