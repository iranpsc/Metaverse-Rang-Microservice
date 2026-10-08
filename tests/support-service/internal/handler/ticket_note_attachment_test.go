package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pbSupport "metarang/shared/pb/support"
	"metarang/support-service/internal/handler"
)

func multipartAttachmentRequest(t *testing.T, field string, names ...string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, name := range names {
		part, err := w.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("file")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadTicketAttachments_AcceptsRequiredExtensionsAndFiveFiles(t *testing.T) {
	storage := &stubFileStorage{}
	req := multipartAttachmentRequest(t, "attachments[]", "a.pdf", "b.docx", "c.jpg", "d.jpeg", "e.PDF")
	urls, err := handler.ExportUploadTicketAttachments(req, storage, "http://app")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 5 || storage.calls != 5 {
		t.Fatalf("urls=%v calls=%d", urls, storage.calls)
	}
	for _, url := range urls {
		if !strings.HasPrefix(url, "http://app/uploads/tickets/stored-") {
			t.Fatalf("url=%s", url)
		}
	}
}

func TestUploadTicketAttachments_RejectsSixthFileAndBadExtension(t *testing.T) {
	req := multipartAttachmentRequest(t, "attachments[]", "1.pdf", "2.pdf", "3.pdf", "4.pdf", "5.pdf", "6.pdf")
	_, err := handler.ExportUploadTicketAttachments(req, &stubFileStorage{}, "http://app")
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}

	req = multipartAttachmentRequest(t, "attachment", "malware.exe")
	_, err = handler.ExportUploadTicketAttachments(req, &stubFileStorage{}, "http://app")
	if err == nil || !strings.Contains(err.Error(), "invalid attachment type") {
		t.Fatalf("err=%v", err)
	}
}

func TestEncodeTicketAttachments(t *testing.T) {
	single, err := handler.ExportEncodeTicketAttachments([]string{"http://a.pdf"})
	if err != nil || single != "http://a.pdf" {
		t.Fatalf("single=%q err=%v", single, err)
	}
	many, err := handler.ExportEncodeTicketAttachments([]string{"http://a.pdf", "http://b.docx"})
	if err != nil || many != `["http://a.pdf","http://b.docx"]` {
		t.Fatalf("many=%q err=%v", many, err)
	}
	if _, err := handler.ExportEncodeTicketAttachments([]string{"1", "2", "3", "4", "5", "6"}); err == nil {
		t.Fatal("expected max error")
	}
	decoded := handler.ExportDecodeTicketAttachments(many)
	if len(decoded) != 2 || decoded[1] != "http://b.docx" {
		t.Fatalf("decoded=%v", decoded)
	}
}

func TestUploadNoteAttachmentFiles_AcceptsRequiredExtensions(t *testing.T) {
	storage := &stubFileStorage{}
	req := multipartAttachmentRequest(t, "attachments[]", "a.pdf", "b.docx", "c.jpg", "d.jpeg")
	urls, err := handler.ExportUploadNoteAttachmentFiles(req, storage, "http://app")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 4 || storage.calls != 4 || storage.lastUploadPath != "/uploads/notes" {
		t.Fatalf("urls=%v calls=%d path=%s", urls, storage.calls, storage.lastUploadPath)
	}

	req = multipartAttachmentRequest(t, "attachment", "bad.exe")
	_, err = handler.ExportUploadNoteAttachmentFiles(req, storage, "http://app")
	if err == nil || !strings.Contains(err.Error(), "invalid attachment type") {
		t.Fatalf("err=%v", err)
	}
}

func TestHTTP_TicketAttachmentsLimit(t *testing.T) {
	var stored string
	tickets := &mockTicketAPI{
		CreateTicketFunc: func(_ context.Context, req *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error) {
			stored = req.Attachment
			out := sampleTicket()
			out.Attachment = req.Attachment
			return out, nil
		},
	}
	h := handler.NewHTTPSupportHandler(tickets, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://app")
	mux := newSupportMux(h, withUser(7))

	body := `{"title":"Title","content":"Body","reciever":9,"attachments":["http://a.pdf","http://b.docx"]}`
	rr := doJSON(mux, http.MethodPost, "/api/tickets", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if stored != `["http://a.pdf","http://b.docx"]` {
		t.Fatalf("stored=%s", stored)
	}
	if !strings.Contains(rr.Body.String(), `"attachments"`) {
		t.Fatalf("body=%s", rr.Body.String())
	}

	tooMany := `{"title":"Title","content":"Body","reciever":9,"attachments":["1","2","3","4","5","6"]}`
	rr = doJSON(mux, http.MethodPost, "/api/tickets", tooMany)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "more than 5") {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func multipartNoteCreateRequest(t *testing.T, field string, names ...string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("title", "T"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("content", "C"); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		part, err := w.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("file-" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/notes", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHTTP_CreateNoteAcceptsUpToFiveFiles(t *testing.T) {
	var got []string
	notes := &mockNoteAPI{
		CreateNoteFunc: func(_ context.Context, req *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error) {
			if req.Title != "T" || req.Content != "C" || req.UserId != 7 {
				t.Fatalf("create=%+v", req)
			}
			got = append([]string{}, req.Attachments...)
			return &pbSupport.NoteResponse{
				Id: 2, Title: req.Title, Content: req.Content,
				Attachments: req.Attachments, Date: "1403/01/01", Time: "11:00:00",
			}, nil
		},
	}
	storage := &stubFileStorage{}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, storage, "http://app")
	mux := newSupportMux(h, withUser(7))

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteCreateRequest(t, "attachments[]", "a.pdf", "b.docx", "c.jpg", "d.jpeg", "e.png"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(got) != 5 || storage.calls != 5 {
		t.Fatalf("attachments=%v calls=%d", got, storage.calls)
	}
	for _, url := range got {
		if !strings.Contains(rr.Body.String(), url) {
			t.Fatalf("missing %s in %s", url, rr.Body.String())
		}
	}

	storage = &stubFileStorage{}
	h = handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, storage, "http://app")
	mux = newSupportMux(h, withUser(7))
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteCreateRequest(t, "attachment", "one.pdf", "two.docx"))
	if rr.Code != http.StatusCreated || len(got) != 2 || storage.calls != 2 {
		t.Fatalf("code=%d attachments=%v calls=%d body=%s", rr.Code, got, storage.calls, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteCreateRequest(t, "attachments[]", "1.pdf", "2.pdf", "3.pdf", "4.pdf", "5.pdf", "6.pdf"))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "more than 5") {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTP_CreateNoteJSONAttachments(t *testing.T) {
	var got []string
	notes := &mockNoteAPI{
		CreateNoteFunc: func(_ context.Context, req *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error) {
			got = append([]string{}, req.Attachments...)
			return &pbSupport.NoteResponse{
				Id: 2, Title: req.Title, Content: req.Content,
				Attachments: req.Attachments, Date: "1403/01/01", Time: "11:00:00",
			}, nil
		},
	}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, nil, "http://app")
	mux := newSupportMux(h, withUser(7))

	body := `{"title":"T","content":"C","attachments":["http://a.pdf","http://b.docx","http://c.jpg","http://d.jpeg","http://e.png"]}`
	rr := doJSON(mux, http.MethodPost, "/api/notes", body)
	if rr.Code != http.StatusCreated || len(got) != 5 || got[4] != "http://e.png" {
		t.Fatalf("code=%d attachments=%v body=%s", rr.Code, got, rr.Body.String())
	}

	tooMany := `{"title":"T","content":"C","attachments":["1","2","3","4","5","6"]}`
	rr = doJSON(mux, http.MethodPost, "/api/notes", tooMany)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "more than 5") {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func multipartNoteUpdateRequest(t *testing.T, field string, names ...string) *http.Request {
	t.Helper()
	req := multipartNoteCreateRequest(t, field, names...)
	req.URL.Path = "/api/notes/4"
	req.Method = http.MethodPut
	return req
}

func multipartNoteUpdateWithLinks(t *testing.T, links []string, files ...string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("title", "T"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("content", "C"); err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if err := w.WriteField("attachments[]", link); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range files {
		part, err := w.CreateFormFile("attachments[]", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("file-" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/notes/4", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHTTP_UpdateNoteReplacesAttachmentsAndDropsRemovedLinks(t *testing.T) {
	var got []string
	updated := 0
	notes := &mockNoteAPI{
		UpdateNoteFunc: func(_ context.Context, req *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error) {
			updated++
			got = append([]string{}, req.Attachments...)
			atts := req.Attachments
			if atts == nil {
				atts = []string{}
			}
			return &pbSupport.NoteResponse{Id: 4, Title: req.Title, Content: req.Content, Attachments: atts}, nil
		},
	}
	storage := &stubFileStorage{}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, storage, "http://app")
	mux := newSupportMux(h, withUser(7))

	body := `{"title":"T2","content":"C2","attachments":["http://1.pdf"]}`
	rr := doJSON(mux, http.MethodPut, "/api/notes/4", body)
	if rr.Code != http.StatusOK || updated != 1 || len(got) != 1 || got[0] != "http://1.pdf" {
		t.Fatalf("json code=%d updated=%d attachments=%v body=%s", rr.Code, updated, got, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"attachment":`) {
		t.Fatalf("note responses must not include attachment: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteUpdateWithLinks(t, []string{"http://1.pdf", "http://1.pdf"}, "5.png"))
	if rr.Code != http.StatusOK || updated != 2 || storage.calls != 1 || len(got) != 2 || got[0] != "http://1.pdf" || got[1] != "http://app/uploads/notes/stored-5.png" {
		t.Fatalf("mix code=%d updated=%d attachments=%v calls=%d body=%s", rr.Code, updated, got, storage.calls, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteUpdateWithLinks(t, []string{"http://1.pdf", "http://2.pdf", "http://3.pdf", "http://4.pdf"}, "a.png", "b.png"))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "more than 5") || updated != 2 || storage.calls != 1 {
		t.Fatalf("limit code=%d updated=%d calls=%d body=%s", rr.Code, updated, storage.calls, rr.Body.String())
	}

	tooMany := `{"title":"T2","content":"C2","attachments":["1","2","3","4","5","6"]}`
	rr = doJSON(mux, http.MethodPut, "/api/notes/4", tooMany)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "more than 5") || updated != 2 {
		t.Fatalf("json limit code=%d updated=%d body=%s", rr.Code, updated, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, multipartNoteUpdateWithLinks(t, []string{""}))
	if rr.Code != http.StatusOK || updated != 3 || len(got) != 0 {
		t.Fatalf("clear code=%d updated=%d attachments=%v body=%s", rr.Code, updated, got, rr.Body.String())
	}
}

func TestMergeTicketAttachmentFieldsJSON(t *testing.T) {
	raw, err := json.Marshal([]string{"http://a.jpg", "http://b.jpeg"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := handler.ExportMergeTicketAttachmentFields("", []string{"http://a.jpg", "http://b.jpeg"})
	if err != nil || got != string(raw) {
		t.Fatalf("got=%s err=%v", got, err)
	}
}
