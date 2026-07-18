package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

const maxTicketAttachmentSize = 5 << 20 // 5MB — matches Laravel SecureFileUpload limit

var allowedTicketAttachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".pdf": true, ".doc": true, ".docx": true,
}

// uploadTicketAttachment stores a ticket attachment via the storage service HTTP API.
// Returns a Laravel-compatible full URL (e.g. http://localhost:8000/uploads/tickets/<file>).
func uploadTicketAttachment(r *http.Request, storageAddr, appURL string) (string, error) {
	if storageAddr == "" {
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

	return uploadBytesToStorage(storageAddr, appURL, "tickets", header.Filename, contentType, data)
}

func uploadBytesToStorage(storageAddr, appURL, uploadSubdir, filename, contentType string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}

	_ = writer.WriteField("upload_path", "/uploads/"+uploadSubdir)
	_ = writer.WriteField("chunk_index", "0")
	_ = writer.WriteField("total_chunks", "1")
	_ = writer.WriteField("total_size", strconv.FormatInt(int64(len(data)), 10))
	_ = writer.WriteField("filename", filename)
	_ = writer.WriteField("content_type", contentType)
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+storageAddr+"/api/upload", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage upload failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("storage upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Path    string  `json:"path"`
		Name    string  `json:"name"`
		Done    float64 `json:"done"`
		Success bool    `json:"success"`
		Error   string  `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("invalid storage response: %w", err)
	}
	if result.Name == "" {
		return "", fmt.Errorf("storage upload did not return file path")
	}

	base := strings.TrimRight(appURL, "/")
	dir := strings.TrimLeft(result.Path, "/")
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return base + "/" + dir + result.Name, nil
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

const maxNoteAttachmentSize = 5 << 20 // 5MB — matches Laravel NoteRequest limit

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
// Frontend sends attachments[] (and current_attachments[] on update).
// DB stores a single attachment URL; the first uploaded or retained file is used.
// Returns attachmentURL, clearAttachment, and error.
func resolveNoteAttachmentURL(r *http.Request, storageAddr, appURL string) (string, bool, error) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return "", false, nil
	}

	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return "", false, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	// Upload first new file from attachments[]
	for _, headers := range r.MultipartForm.File["attachments[]"] {
		if headers == nil {
			continue
		}
		url, err := uploadMultipartFileHeader(storageAddr, appURL, "notes", headers)
		if err != nil {
			return "", false, err
		}
		return url, false, nil
	}

	// Fallback: singular attachment field (Laravel-style)
	if file, header, err := r.FormFile("attachment"); err == nil && header != nil {
		defer func() { _ = file.Close() }()
		url, uploadErr := uploadOpenedFile(storageAddr, appURL, "notes", header.Filename, header.Header.Get("Content-Type"), file, header.Size, allowedNoteAttachmentExts, maxNoteAttachmentSize)
		if uploadErr != nil {
			return "", false, uploadErr
		}
		return url, false, nil
	}

	// Explicit clear: frontend sends attachments[] with empty value
	if values, ok := r.MultipartForm.Value["attachments[]"]; ok {
		for _, v := range values {
			if v == "" {
				return "", true, nil
			}
		}
	}

	// Keep existing attachment from current_attachments[] on update
	if values, ok := r.MultipartForm.Value["current_attachments[]"]; ok {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				return v, false, nil
			}
		}
	}

	return "", false, nil
}

func uploadMultipartFileHeader(storageAddr, appURL, uploadSubdir string, header *multipart.FileHeader) (string, error) {
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
	return uploadOpenedFile(storageAddr, appURL, uploadSubdir, header.Filename, contentType, file, header.Size, allowedNoteAttachmentExts, maxNoteAttachmentSize)
}

func uploadOpenedFile(storageAddr, appURL, uploadSubdir, filename, contentType string, file io.Reader, size int64, allowedExts map[string]bool, maxSize int64) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return "", fmt.Errorf("invalid attachment type")
	}

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
	return uploadBytesToStorage(storageAddr, appURL, uploadSubdir, filename, contentType, data)
}

const maxReportAttachmentSize = 1 << 20 // 1MB — matches Laravel ReportRequest SecureFileUpload limit

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

// uploadReportAttachments uploads report attachment files and returns relative DB paths (e.g. reports/file.png).
func uploadReportAttachments(r *http.Request, storageAddr, appURL string) ([]string, error) {
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

	if len(headers) > 5 {
		return nil, fmt.Errorf("attachments must not have more than 5 items")
	}

	var paths []string
	for _, header := range headers {
		if header == nil {
			continue
		}
		relPath, err := uploadReportFileHeader(storageAddr, appURL, header)
		if err != nil {
			return nil, err
		}
		paths = append(paths, relPath)
	}
	return paths, nil
}

func uploadReportFileHeader(storageAddr, appURL string, header *multipart.FileHeader) (string, error) {
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

	_, relPath, err := uploadBytesToStorageWithRelativePath(storageAddr, appURL, "reports", header.Filename, contentType, data)
	return relPath, err
}

func uploadBytesToStorageWithRelativePath(storageAddr, appURL, uploadSubdir, filename, contentType string, data []byte) (fullURL, relativePath string, err error) {
	fullURL, err = uploadBytesToStorage(storageAddr, appURL, uploadSubdir, filename, contentType, data)
	if err != nil {
		return "", "", err
	}
	relativePath = uploadSubdir + "/" + filepath.Base(filename)
	return fullURL, relativePath, nil
}
