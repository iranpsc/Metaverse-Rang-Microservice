package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fileStorageUploader delegates binary uploads to storage-service.
// Support-service must never persist attachment bytes locally.
type fileStorageUploader interface {
	UploadChunk(ctx context.Context, uploadID, uploadPath, filename, contentType string, data []byte) (relativePath string, err error)
}

const (
	maxTicketAttachmentSize  = 5 << 20
	maxTicketAttachmentCount = 5
)

// allowedTicketAttachmentExts accepts pdf, docx, jpg, and jpeg, and keeps png and doc.
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

// uploadTicketAttachments stores up to 5 ticket files from attachment, attachments, and attachments[].
func uploadTicketAttachments(r *http.Request, storage fileStorageUploader, appURL string) ([]string, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, nil
	}
	if storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	headers := collectAttachmentFileHeaders(r)
	if len(headers) == 0 {
		return nil, nil
	}
	if len(headers) > maxTicketAttachmentCount {
		return nil, fmt.Errorf("attachments must not have more than 5 items")
	}

	urls := make([]string, 0, len(headers))
	for _, header := range headers {
		fileURL, err := uploadTicketFileHeader(r.Context(), storage, appURL, header)
		if err != nil {
			return nil, err
		}
		urls = append(urls, fileURL)
	}
	return urls, nil
}

func uploadTicketFileHeader(ctx context.Context, storage fileStorageUploader, appURL string, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedTicketAttachmentExts[ext] {
		return "", fmt.Errorf("invalid attachment type: only png, jpg, jpeg, pdf, doc, docx are allowed")
	}
	if header.Size > maxTicketAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 5MB limit")
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
	if int64(len(data)) > maxTicketAttachmentSize {
		return "", fmt.Errorf("attachment exceeds 5MB limit")
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return uploadBytesToStorage(ctx, storage, appURL, "tickets", header.Filename, contentType, data)
}

// encodeTicketAttachments stores one URL as-is and two to five URLs as a JSON array.
func encodeTicketAttachments(urls []string) (string, error) {
	cleaned := make([]string, 0, len(urls))
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			cleaned = append(cleaned, raw)
		}
	}
	if len(cleaned) > maxTicketAttachmentCount {
		return "", fmt.Errorf("attachments must not have more than 5 items")
	}
	switch len(cleaned) {
	case 0:
		return "", nil
	case 1:
		return cleaned[0], nil
	default:
		encoded, err := json.Marshal(cleaned)
		if err != nil {
			return "", fmt.Errorf("encode ticket attachments: %w", err)
		}
		return string(encoded), nil
	}
}

// decodeTicketAttachments expands a stored ticket attachment into a list of URLs.
func decodeTicketAttachments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	if strings.HasPrefix(raw, "[") {
		var urls []string
		if err := json.Unmarshal([]byte(raw), &urls); err == nil {
			cleaned := make([]string, 0, len(urls))
			for _, item := range urls {
				item = strings.TrimSpace(item)
				if item != "" {
					cleaned = append(cleaned, item)
				}
			}
			return cleaned
		}
	}
	return []string{raw}
}

// mergeTicketAttachmentFields prefers an attachments list and still accepts a single attachment string.
func mergeTicketAttachmentFields(single string, many []string) (string, error) {
	if len(many) > 0 {
		return encodeTicketAttachments(many)
	}
	if len(decodeTicketAttachments(single)) > maxTicketAttachmentCount {
		return "", fmt.Errorf("attachments must not have more than 5 items")
	}
	return single, nil
}

func collectAttachmentFileHeaders(r *http.Request) []*multipart.FileHeader {
	if r == nil || r.MultipartForm == nil {
		return nil
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
	for _, key := range []string{"attachment", "attachments", "attachments[]"} {
		if files, ok := r.MultipartForm.File[key]; ok && len(files) > 0 {
			addHeaders(files)
		}
	}
	for key, files := range r.MultipartForm.File {
		if strings.HasPrefix(key, "attachments[") && key != "attachments[]" && len(files) > 0 {
			addHeaders(files)
		}
	}
	return headers
}

// noteUpdateAttachmentInput separates kept links from new files on a note edit.
// present is true when the form included an attachment field or a file, including
// an empty attachments[] value that clears the list.
func noteUpdateAttachmentInput(r *http.Request) (links []string, files []*multipart.FileHeader, present bool) {
	if r == nil || r.MultipartForm == nil {
		return nil, nil, false
	}
	files = collectAttachmentFileHeaders(r)
	if len(files) > 0 {
		present = true
	}
	values := make([]string, 0)
	for _, key := range []string{"attachments[]", "attachments", "attachment", "current_attachments[]"} {
		if items, ok := r.MultipartForm.Value[key]; ok {
			present = true
			values = append(values, items...)
		}
	}
	indexed := make([]string, 0)
	for key := range r.MultipartForm.Value {
		if !isIndexedAttachmentKey(key) {
			continue
		}
		present = true
		indexed = append(indexed, key)
	}
	sort.Strings(indexed)
	for _, key := range indexed {
		values = append(values, r.MultipartForm.Value[key]...)
	}
	return dedupeNonEmpty(values), files, present
}

func isIndexedAttachmentKey(key string) bool {
	if key == "attachments[]" || key == "current_attachments[]" {
		return false
	}
	return strings.HasPrefix(key, "attachments[") || strings.HasPrefix(key, "current_attachments[")
}

func dedupeNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// uploadNoteFileHeaders uploads an already counted set of note files.
func uploadNoteFileHeaders(r *http.Request, storage fileStorageUploader, appURL string, headers []*multipart.FileHeader) ([]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	if storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}
	urls := make([]string, 0, len(headers))
	for _, header := range headers {
		fileURL, err := uploadMultipartFileHeader(r.Context(), storage, appURL, "notes", header)
		if err != nil {
			return nil, err
		}
		urls = append(urls, fileURL)
	}
	return urls, nil
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

const (
	maxNoteAttachmentCount = 5
	maxNoteAttachmentSize  = 5 << 20
)

var allowedNoteAttachmentExts = map[string]bool{
	".pdf": true, ".docx": true, ".jpg": true, ".jpeg": true, ".png": true,
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

// uploadNoteAttachmentFiles stores note files from attachment, attachments, and attachments[].
// Accepted extensions are pdf, docx, jpg, jpeg, and png. At most 5 files are accepted per request.
func uploadNoteAttachmentFiles(r *http.Request, storage fileStorageUploader, appURL string) ([]string, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, nil
	}
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	headers := collectAttachmentFileHeaders(r)
	if len(headers) == 0 {
		return nil, nil
	}
	if len(headers) > maxNoteAttachmentCount {
		return nil, fmt.Errorf("attachments must not have more than 5 items")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}

	urls := make([]string, 0, len(headers))
	for _, header := range headers {
		fileURL, err := uploadMultipartFileHeader(r.Context(), storage, appURL, "notes", header)
		if err != nil {
			return nil, err
		}
		urls = append(urls, fileURL)
	}
	return urls, nil
}

// mergeNoteAttachmentURLs prefers an attachments list and still accepts a single attachment string.
func mergeNoteAttachmentURLs(single string, many []string) ([]string, error) {
	urls := make([]string, 0, len(many)+1)
	if len(many) > 0 {
		for _, item := range many {
			item = strings.TrimSpace(item)
			if item != "" {
				urls = append(urls, item)
			}
		}
	} else if item := strings.TrimSpace(single); item != "" {
		urls = append(urls, item)
	}
	if len(urls) > maxNoteAttachmentCount {
		return nil, fmt.Errorf("attachments must not have more than 5 items")
	}
	if len(urls) == 0 {
		return nil, nil
	}
	return urls, nil
}

func uploadMultipartFileHeader(ctx context.Context, storage fileStorageUploader, appURL, uploadSubdir string, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedNoteAttachmentExts[ext] {
		return "", fmt.Errorf("invalid attachment type: only pdf, docx, jpg, jpeg, png are allowed")
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
