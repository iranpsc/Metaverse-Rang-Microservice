package handler

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fileStorageUploader delegates binary uploads to storage-service.
// Support-service must never persist attachment bytes locally.
type fileStorageUploader interface {
	UploadChunk(ctx context.Context, uploadID, uploadPath, filename, contentType string, data []byte) (relativePath string, err error)
}

const maxTicketAttachmentSize = 5 << 20

var allowedTicketAttachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".pdf": true, ".doc": true, ".docx": true,
}

// uploadTicketAttachment stores a ticket attachment via storage-service.
func uploadTicketAttachment(r *http.Request, storage fileStorageUploader, appURL string) (string, error) {
	if storage == nil {
		return "", fmt.Errorf("storage service not configured")
	}

	file, header, err := r.FormFile("attachment")
	if err != nil {
		if err == http.ErrMissingFile {
			return "", nil
		}
		return "", fmt.Errorf("failed to read attachment: %w", err)
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedTicketAttachmentExts[ext] {
		return "", fmt.Errorf("invalid attachment type: only png, jpg, jpeg, pdf, doc, docx are allowed")
	}

	if header.Size > maxTicketAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 5MB limit")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read attachment data: %w", err)
	}
	if int64(len(data)) > maxTicketAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 5MB limit")
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return uploadBytesToStorage(r.Context(), storage, appURL, "tickets", header.Filename, contentType, data)
}

func prependPublicURL(appURL, path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if appURL == "" {
		return path
	}
	return strings.TrimRight(appURL, "/") + "/" + strings.TrimLeft(path, "/")
}

// relativeDBPath strips a leading "uploads/" so report image rows store paths
// like "reports/<hash>.png" (formatReportResponse prefixes APP_URL/uploads/).
func relativeDBPath(storagePath string) string {
	p := strings.ReplaceAll(storagePath, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "uploads/")
	return strings.TrimPrefix(p, "/")
}

func uploadBytesToStorage(ctx context.Context, storage fileStorageUploader, appURL, uploadSubdir, filename, contentType string, data []byte) (string, error) {
	if storage == nil {
		return "", fmt.Errorf("storage service not configured")
	}
	uploadID := fmt.Sprintf("support_%s_%d", uploadSubdir, time.Now().UnixNano())
	path, err := storage.UploadChunk(ctx, uploadID, "/uploads/"+uploadSubdir, filename, contentType, data)
	if err != nil {
		return "", err
	}
	return prependPublicURL(appURL, path), nil
}

func uploadBytesToStorageWithRelativePath(ctx context.Context, storage fileStorageUploader, appURL, uploadSubdir, filename, contentType string, data []byte) (fullURL, relativePath string, err error) {
	if storage == nil {
		return "", "", fmt.Errorf("storage service not configured")
	}
	uploadID := fmt.Sprintf("support_%s_%d", uploadSubdir, time.Now().UnixNano())
	path, err := storage.UploadChunk(ctx, uploadID, "/uploads/"+uploadSubdir, filename, contentType, data)
	if err != nil {
		return "", "", err
	}
	return prependPublicURL(appURL, path), relativeDBPath(path), nil
}

// parseTicketFormFields extracts ticket form fields from multipart or urlencoded bodies.
func parseTicketFormFields(r *http.Request) (title, content, department string, receiverID *uint64, err error) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return "", "", "", nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
		title = r.FormValue("title")
		content = r.FormValue("content")
		department = r.FormValue("department")
		if rec := strings.TrimSpace(r.FormValue("reciever")); rec != "" {
			id, parseErr := strconv.ParseUint(rec, 10, 64)
			if parseErr != nil {
				return "", "", "", nil, fmt.Errorf("invalid reciever")
			}
			receiverID = &id
		}
		return title, content, department, receiverID, nil
	}

	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return "", "", "", nil, fmt.Errorf("failed to parse form: %w", err)
		}
		title = r.FormValue("title")
		content = r.FormValue("content")
		department = r.FormValue("department")
		if rec := strings.TrimSpace(r.FormValue("reciever")); rec != "" {
			id, parseErr := strconv.ParseUint(rec, 10, 64)
			if parseErr != nil {
				return "", "", "", nil, fmt.Errorf("invalid reciever")
			}
			receiverID = &id
		}
		return title, content, department, receiverID, nil
	}

	return "", "", "", nil, nil
}

const maxNoteAttachmentSize = 5 << 20

var allowedNoteAttachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".pdf": true,
}

// parseNoteFormFields extracts note title/content from multipart or urlencoded bodies.
func parseNoteFormFields(r *http.Request) (title, content string, err error) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return "", "", fmt.Errorf("failed to parse multipart form: %w", err)
		}
		return r.FormValue("title"), r.FormValue("content"), nil
	}

	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return "", "", fmt.Errorf("failed to parse form: %w", err)
		}
		return r.FormValue("title"), r.FormValue("content"), nil
	}

	return "", "", nil
}

// resolveNoteAttachmentURL handles note file uploads from multipart form.
// Returns attachmentURL, clearAttachment, and error.
func resolveNoteAttachmentURL(r *http.Request, storage fileStorageUploader, appURL string) (string, bool, error) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return "", false, nil
	}

	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return "", false, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	for _, headers := range r.MultipartForm.File["attachments[]"] {
		if headers == nil {
			continue
		}
		if storage == nil {
			return "", false, fmt.Errorf("storage service not configured")
		}
		url, err := uploadMultipartFileHeader(r.Context(), storage, appURL, "notes", headers)
		if err != nil {
			return "", false, err
		}
		return url, false, nil
	}

	if file, header, err := r.FormFile("attachment"); err == nil && header != nil {
		defer func() { _ = file.Close() }()
		if storage == nil {
			return "", false, fmt.Errorf("storage service not configured")
		}
		url, uploadErr := uploadOpenedFile(r.Context(), storage, appURL, "notes", header.Filename, header.Header.Get("Content-Type"), file, header.Size, allowedNoteAttachmentExts, maxNoteAttachmentSize)
		if uploadErr != nil {
			return "", false, uploadErr
		}
		return url, false, nil
	}

	if values, ok := r.MultipartForm.Value["attachments[]"]; ok {
		for _, v := range values {
			if v == "" {
				return "", true, nil
			}
		}
	}

	if values, ok := r.MultipartForm.Value["current_attachments[]"]; ok {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return v, false, nil
			}
		}
	}

	return "", false, nil
}

func uploadMultipartFileHeader(ctx context.Context, storage fileStorageUploader, appURL, uploadSubdir string, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedNoteAttachmentExts[ext] {
		return "", fmt.Errorf("invalid attachment type: only png, jpg, jpeg, pdf are allowed")
	}
	if header.Size > maxNoteAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 5MB limit")
	}

	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to read attachment: %w", err)
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	return uploadOpenedFile(ctx, storage, appURL, uploadSubdir, header.Filename, contentType, file, header.Size, allowedNoteAttachmentExts, maxNoteAttachmentSize)
}

func uploadOpenedFile(ctx context.Context, storage fileStorageUploader, appURL, uploadSubdir, filename, contentType string, file io.Reader, size int64, allowedExts map[string]bool, maxSize int64) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return "", fmt.Errorf("invalid attachment type")
	}
	_ = size

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read attachment data: %w", err)
	}
	if int64(len(data)) > maxSize {
		return "", fmt.Errorf("attachment exceeds size limit")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return uploadBytesToStorage(ctx, storage, appURL, uploadSubdir, filename, contentType, data)
}

const maxReportAttachmentSize = 1 << 20

var allowedReportAttachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".pdf": true,
}

// parseReportFormFields extracts report form fields from multipart or urlencoded bodies.
func parseReportFormFields(r *http.Request) (title, content, subject, url string, err error) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return "", "", "", "", fmt.Errorf("failed to parse multipart form: %w", err)
		}
		return r.FormValue("title"), r.FormValue("content"), r.FormValue("subject"), r.FormValue("url"), nil
	}

	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return "", "", "", "", fmt.Errorf("failed to parse form: %w", err)
		}
		return r.FormValue("title"), r.FormValue("content"), r.FormValue("subject"), r.FormValue("url"), nil
	}

	return "", "", "", "", nil
}

// uploadReportAttachments uploads report attachment files via storage-service
// and returns relative DB paths (e.g. "reports/<stored-name>"). ReportService
// only wires these paths into image records — same pattern as notes.
func uploadReportAttachments(r *http.Request, storage fileStorageUploader, appURL string) ([]string, error) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, nil
	}

	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	var headers []*multipart.FileHeader
	seen := make(map[string]bool)
	addHeaders := func(files []*multipart.FileHeader) {
		for _, header := range files {
			if header == nil {
				continue
			}
			key := header.Filename + ":" + strconv.FormatInt(header.Size, 10)
			if seen[key] {
				continue
			}
			seen[key] = true
			headers = append(headers, header)
		}
	}
	for _, key := range []string{"attachments[]", "attachments"} {
		if files, ok := r.MultipartForm.File[key]; ok && len(files) > 0 {
			addHeaders(files)
		}
	}
	for key, files := range r.MultipartForm.File {
		if strings.HasPrefix(key, "attachments[") && key != "attachments[]" && len(files) > 0 {
			addHeaders(files)
		}
	}

	if len(headers) == 0 {
		return nil, nil
	}
	if storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}
	if len(headers) > 5 {
		return nil, fmt.Errorf("attachments must not have more than 5 items")
	}

	var paths []string
	for _, header := range headers {
		if header == nil {
			continue
		}
		relPath, err := uploadReportFileHeader(r.Context(), storage, appURL, header)
		if err != nil {
			return nil, err
		}
		paths = append(paths, relPath)
	}
	return paths, nil
}

func uploadReportFileHeader(ctx context.Context, storage fileStorageUploader, appURL string, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedReportAttachmentExts[ext] {
		return "", fmt.Errorf("invalid attachment type: only png, jpg, jpeg, pdf are allowed")
	}
	if header.Size > maxReportAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 1MB limit")
	}

	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to read attachment: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read attachment data: %w", err)
	}
	if int64(len(data)) > maxReportAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 1MB limit")
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, relPath, err := uploadBytesToStorageWithRelativePath(ctx, storage, appURL, "reports", header.Filename, contentType, data)
	return relPath, err
}
