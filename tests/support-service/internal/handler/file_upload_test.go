package handler_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"metarang/support-service/internal/handler"
)

type stubFileStorage struct {
	path           string
	err            error
	calls          int
	lastUploadPath string
	lastFilename   string
}

func (s *stubFileStorage) UploadChunk(_ context.Context, _, uploadPath, filename, _ string, _ []byte) (string, error) {
	s.calls++
	s.lastUploadPath = uploadPath
	s.lastFilename = filename
	if s.err != nil {
		return "", s.err
	}
	if s.path != "" {
		return s.path, nil
	}
	return strings.TrimSuffix(uploadPath, "/") + "/stored-" + filename, nil
}

func TestUploadTicketAttachment_StorageNotConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := handler.ExportUploadTicketAttachment(req, nil, "http://app")
	if err == nil || !strings.Contains(err.Error(), "storage service not configured") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadTicketAttachment_MissingFileReturnsEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "t")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	url, err := handler.ExportUploadTicketAttachment(req, &stubFileStorage{}, "http://app")
	if err != nil || url != "" {
		t.Fatalf("url=%q err=%v", url, err)
	}
}

func TestUploadTicketAttachment_InvalidType(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachment", "malware.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("MZ"))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = handler.ExportUploadTicketAttachment(req, &stubFileStorage{}, "http://app")
	if err == nil || !strings.Contains(err.Error(), "invalid attachment type") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadTicketAttachment_ExceedsSize(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachment", "big.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(bytes.Repeat([]byte("x"), handler.ExportMaxTicketAttachmentSize+1))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = handler.ExportUploadTicketAttachment(req, &stubFileStorage{}, "http://app")
	if err == nil || !strings.Contains(err.Error(), "5MB") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadBytesToStorage_DelegatesToStorageService(t *testing.T) {
	storage := &stubFileStorage{path: "/uploads/tickets/hash.png"}
	got, err := handler.ExportUploadBytesToStorage(context.Background(), storage, "http://app.test", "tickets", "a.png", "image/png", []byte("hi"))
	if err != nil || got != "http://app.test/uploads/tickets/hash.png" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if storage.calls != 1 || storage.lastUploadPath != "/uploads/tickets" {
		t.Fatalf("calls=%d uploadPath=%q", storage.calls, storage.lastUploadPath)
	}

	_, err = handler.ExportUploadBytesToStorage(context.Background(), nil, "http://app", "tickets", "a.png", "image/png", []byte("hi"))
	if err == nil || !strings.Contains(err.Error(), "storage service not configured") {
		t.Fatalf("err=%v", err)
	}

	_, err = handler.ExportUploadBytesToStorage(context.Background(), &stubFileStorage{err: fmt.Errorf("boom")}, "http://app", "tickets", "a.png", "image/png", []byte("hi"))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseTicketFormFields(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "T")
	_ = w.WriteField("content", "C")
	_ = w.WriteField("department", "D")
	_ = w.WriteField("reciever", "42")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	title, content, dept, receiverID, err := handler.ExportParseTicketFormFields(req)
	if err != nil || title != "T" || content != "C" || dept != "D" || receiverID == nil || *receiverID != 42 {
		t.Fatalf("got title=%q content=%q dept=%q rec=%v err=%v", title, content, dept, receiverID, err)
	}

	form := url.Values{}
	form.Set("title", "t2")
	form.Set("content", "c2")
	form.Set("department", "d2")
	form.Set("reciever", "7")
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	title, content, dept, receiverID, err = handler.ExportParseTicketFormFields(req)
	if err != nil || title != "t2" || content != "c2" || dept != "d2" || receiverID == nil || *receiverID != 7 {
		t.Fatalf("urlencoded title=%q content=%q dept=%q rec=%v err=%v", title, content, dept, receiverID, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	title, content, dept, receiverID, err = handler.ExportParseTicketFormFields(req)
	if err != nil || title != "" || content != "" || dept != "" || receiverID != nil {
		t.Fatalf("json title=%q content=%q dept=%q rec=%v err=%v", title, content, dept, receiverID, err)
	}
}

func TestParseNoteAndReportFormFields(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "nt")
	_ = w.WriteField("content", "nc")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	title, content, err := handler.ExportParseNoteFormFields(req)
	if err != nil || title != "nt" || content != "nc" {
		t.Fatalf("note title=%q content=%q err=%v", title, content, err)
	}

	buf.Reset()
	w = multipart.NewWriter(&buf)
	_ = w.WriteField("title", "rt")
	_ = w.WriteField("content", "rc")
	_ = w.WriteField("subject", "displayError")
	_ = w.WriteField("url", "https://x")
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	title, content, subject, u, err := handler.ExportParseReportFormFields(req)
	if err != nil || title != "rt" || content != "rc" || subject != "displayError" || u != "https://x" {
		t.Fatalf("report title=%q content=%q subject=%q url=%q err=%v", title, content, subject, u, err)
	}
}

func TestResolveNoteAttachmentURL(t *testing.T) {
	storage := &stubFileStorage{path: "/uploads/notes/hash.png"}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachments[]", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("png"))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	url, clear, err := handler.ExportResolveNoteAttachmentURL(req, storage, "http://app")
	if err != nil || clear || url != "http://app/uploads/notes/hash.png" {
		t.Fatalf("url=%q clear=%v err=%v", url, clear, err)
	}

	buf.Reset()
	w = multipart.NewWriter(&buf)
	part, err = w.CreateFormFile("attachments[]", "bad.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("MZ"))
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, _, err = handler.ExportResolveNoteAttachmentURL(req, storage, "http://app")
	if err == nil || !strings.Contains(err.Error(), "invalid attachment type") {
		t.Fatalf("err=%v", err)
	}

	buf.Reset()
	w = multipart.NewWriter(&buf)
	_ = w.WriteField("attachments[]", "")
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	url, clear, err = handler.ExportResolveNoteAttachmentURL(req, storage, "http://app")
	if err != nil || url != "" || !clear {
		t.Fatalf("clear url=%q clear=%v err=%v", url, clear, err)
	}

	buf.Reset()
	w = multipart.NewWriter(&buf)
	_ = w.WriteField("current_attachments[]", "keep.png")
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	url, clear, err = handler.ExportResolveNoteAttachmentURL(req, storage, "http://app")
	if err != nil || url != "keep.png" || clear {
		t.Fatalf("current url=%q clear=%v err=%v", url, clear, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	url, clear, err = handler.ExportResolveNoteAttachmentURL(req, storage, "http://app")
	if err != nil || url != "" || clear {
		t.Fatalf("non-multipart url=%q clear=%v err=%v", url, clear, err)
	}
}

func TestUploadReportFileHeader_ExceedsSize(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachments[]", "big.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(bytes.Repeat([]byte("x"), handler.ExportMaxReportAttachmentSize+1))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = handler.ExportUploadReportAttachments(req, &stubFileStorage{}, "http://app")
	if err == nil || !strings.Contains(err.Error(), "1MB") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadBytesToStorageWithRelativePath(t *testing.T) {
	storage := &stubFileStorage{path: "/uploads/reports/abc123def.png"}
	full, rel, err := handler.ExportUploadBytesToStorageWithRelativePath(
		context.Background(),
		storage,
		"http://app",
		"reports",
		"original.png",
		"image/png",
		[]byte("hi"),
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if full != "http://app/uploads/reports/abc123def.png" {
		t.Fatalf("full=%q", full)
	}
	if rel != "reports/abc123def.png" {
		t.Fatalf("rel=%q want reports/abc123def.png (must use storage name, not original filename)", rel)
	}
}

func TestUploadReportAttachments_DelegatesToStorage(t *testing.T) {
	storage := &stubFileStorage{path: "/uploads/reports/stored-hash.png"}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachments[]", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("png-bytes"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	paths, err := handler.ExportUploadReportAttachments(req, storage, "http://app")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(paths) != 1 || paths[0] != "reports/stored-hash.png" {
		t.Fatalf("paths=%v", paths)
	}
	if storage.calls != 1 || storage.lastUploadPath != "/uploads/reports" {
		t.Fatalf("calls=%d uploadPath=%q", storage.calls, storage.lastUploadPath)
	}
}

func TestUploadReportAttachments_StorageNotConfigured(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachments[]", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("png"))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = handler.ExportUploadReportAttachments(req, nil, "http://app")
	if err == nil || !strings.Contains(err.Error(), "storage service not configured") {
		t.Fatalf("err=%v", err)
	}
}

func TestRelativeDBPath(t *testing.T) {
	if got := handler.ExportRelativeDBPath("/uploads/reports/a.png"); got != "reports/a.png" {
		t.Fatalf("got=%q", got)
	}
	if got := handler.ExportPrependPublicURL("http://app/", "/uploads/x.png"); got != "http://app/uploads/x.png" {
		t.Fatalf("got=%q", got)
	}
}
