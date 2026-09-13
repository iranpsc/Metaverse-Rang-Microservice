package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarang/storage-service/internal/ftp"
	"metarang/storage-service/internal/handler"
	"metarang/storage-service/internal/service"
)

func newHTTPHandlerForGaps(t *testing.T) (*handler.HTTPHandler, string, string) {
	t.Helper()
	tempDir := t.TempDir()
	chunkDir := filepath.Join(tempDir, "chunks")
	chunkManager, err := service.NewChunkManager(chunkDir)
	if err != nil {
		t.Fatalf("NewChunkManager: %v", err)
	}
	uploadRoot := filepath.Join(tempDir, "uploads")
	ftpClient := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://example.com")
	storageService := service.NewStorageService(ftpClient, chunkManager, uploadRoot)
	return handler.NewHTTPHandler(storageService, uploadRoot), uploadRoot, chunkDir
}

func TestHTTPHandler_HandleChunkUpload_UploadPath(t *testing.T) {
	h, uploadRoot, _ := newHTTPHandlerForGaps(t)

	contentType, body, err := createMultipartFormData("photo.jpg", []byte("jpeg-bytes"), map[string]string{
		"upload_id":    "custom-layout",
		"filename":     "photo.jpg",
		"content_type": "image/jpeg",
		"upload_path":  "/uploads/profile",
	})
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	h.HandleChunkUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}
	path, _ := response["path"].(string)
	name, _ := response["name"].(string)
	if path != "/uploads/profile/" {
		t.Fatalf("path = %q, want /uploads/profile/", path)
	}
	if name == "" {
		t.Fatal("expected generated name")
	}
	if _, err := os.Stat(filepath.Join(uploadRoot, "profile", name)); err != nil {
		t.Fatalf("expected file under profile/: %v", err)
	}
}

func TestHTTPHandler_HandleChunkUpload_ServiceFailure(t *testing.T) {
	tempDir := t.TempDir()
	chunkDir := filepath.Join(tempDir, "chunks")
	chunkManager, err := service.NewChunkManager(chunkDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "blocked-upload"), []byte("not-a-dir"), 0644); err != nil {
		t.Fatal(err)
	}
	uploadRoot := filepath.Join(tempDir, "uploads")
	ftpClient := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://example.com")
	storageService := service.NewStorageService(ftpClient, chunkManager, uploadRoot)
	h := handler.NewHTTPHandler(storageService, uploadRoot)

	contentType, body, err := createMultipartFormData("a.txt", []byte("payload"), map[string]string{
		"upload_id": "blocked-upload",
		"filename":  "a.txt",
	})
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	h.HandleChunkUpload(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if response["success"] != false {
		t.Fatalf("expected success=false, got %#v", response)
	}
	if response["error"] == nil || response["error"] == "" {
		t.Fatalf("expected error message, got %#v", response)
	}
}

func TestHTTPHandler_HandleChunkUpload_CORSHeaders(t *testing.T) {
	h, _, _ := newHTTPHandlerForGaps(t)

	assertCORS := func(t *testing.T, header http.Header) {
		t.Helper()
		if got := header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Allow-Origin = %q", got)
		}
		if got := header.Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
			t.Fatalf("Allow-Methods = %q", got)
		}
		if got := header.Get("Access-Control-Allow-Headers"); got != "Content-Type, Cache-Control, X-Requested-With" {
			t.Fatalf("Allow-Headers = %q", got)
		}
	}

	t.Run("OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/upload", nil)
		w := httptest.NewRecorder()
		h.HandleChunkUpload(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		assertCORS(t, w.Header())
	})

	t.Run("POST", func(t *testing.T) {
		contentType, body, err := createMultipartFormData("a.txt", []byte("x"), map[string]string{
			"upload_path": "cors",
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		h.HandleChunkUpload(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
		}
		assertCORS(t, w.Header())
	})
}

func TestHTTPHandler_HandleChunkUpload_MalformedMultipart(t *testing.T) {
	h, _, _ := newHTTPHandlerForGaps(t)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("not-multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----bad")
	w := httptest.NewRecorder()
	h.HandleChunkUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if response["success"] != false {
		t.Fatalf("expected success=false, got %#v", response)
	}
}

func TestHTTPHandler_HandleChunkUpload_MissingMultipart(t *testing.T) {
	h, _, _ := newHTTPHandlerForGaps(t)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`{"file":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleChunkUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestHTTPHandler_HandleChunkUpload_FilenameAndContentTypeDefaults(t *testing.T) {
	h, _, _ := newHTTPHandlerForGaps(t)

	t.Run("form fields override file header", func(t *testing.T) {
		contentType, body, err := createMultipartFormData("ignored.bin", []byte("png-bytes"), map[string]string{
			"filename":     "avatar.png",
			"content_type": "image/png",
			"upload_path":  "profile",
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		h.HandleChunkUpload(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
		}
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		name, _ := response["name"].(string)
		mime, _ := response["mime_type"].(string)
		if !strings.HasSuffix(name, ".png") {
			t.Fatalf("name = %q, want .png extension from filename field", name)
		}
		if mime != "image/png" {
			t.Fatalf("mime_type = %q, want image/png", mime)
		}
	})

	t.Run("empty content type defaults to octet-stream", func(t *testing.T) {
		body := &bytes.Buffer{}
		mw := multipart.NewWriter(body)
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="data.bin"`)
		part, err := mw.CreatePart(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("raw")); err != nil {
			t.Fatal(err)
		}
		if err := mw.WriteField("upload_path", "docs"); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		h.HandleChunkUpload(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
		}
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response["mime_type"] != "application/octet-stream" {
			t.Fatalf("mime_type = %#v, want application/octet-stream", response["mime_type"])
		}
		name, _ := response["name"].(string)
		if !strings.HasSuffix(name, ".bin") {
			t.Fatalf("name = %q, want fileHeader filename extension", name)
		}
	})
}

func TestHTTPHandler_ServeUploads_MissingFile(t *testing.T) {
	h, _, _ := newHTTPHandlerForGaps(t)

	req := httptest.NewRequest(http.MethodGet, "/uploads/profile/missing.jpg", nil)
	w := httptest.NewRecorder()
	h.ServeUploads(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHTTPHandler_ServeUploads_DefaultRoot(t *testing.T) {
	tempDir := t.TempDir()
	chunkManager, err := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
	if err != nil {
		t.Fatal(err)
	}
	ftpClient := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://example.com")
	svc := service.NewStorageService(ftpClient, chunkManager, tempDir)
	h := handler.NewHTTPHandler(svc, "")

	dir := filepath.Join("uploads", "gap-default-root")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("uploads", "gap-default-root")) })
	filePath := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(filePath, []byte("from-default-root"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/uploads/gap-default-root/ok.txt", nil)
	w := httptest.NewRecorder()
	h.ServeUploads(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), []byte("from-default-root")) {
		t.Fatalf("body = %q", w.Body.Bytes())
	}
}
