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
	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
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

func TestHTTP_NoteAttachmentAddAndDelete(t *testing.T) {
	notes := &mockNoteAPI{
		AddNoteAttachmentsFunc: func(_ context.Context, noteID, userID uint64, attachments []string) (*pbSupport.NoteResponse, error) {
			if noteID != 4 || userID != 7 || len(attachments) != 1 || attachments[0] != "http://a.pdf" {
				t.Fatalf("add note=%d user=%d attachments=%v", noteID, userID, attachments)
			}
			return &pbSupport.NoteResponse{Id: noteID, Title: "T", Content: "C", Attachments: attachments, Date: "1403/01/01", Time: "10:00:00"}, nil
		},
		DeleteNoteAttachmentFunc: func(_ context.Context, noteID, userID uint64, attachment string) (*pbSupport.NoteResponse, error) {
			if noteID != 4 || userID != 7 || attachment != "http://a.pdf" {
				t.Fatalf("delete note=%d user=%d attachment=%s", noteID, userID, attachment)
			}
			return &pbSupport.NoteResponse{Id: noteID, Title: "T", Content: "C", Attachments: []string{}, Date: "1403/01/01", Time: "10:00:00"}, nil
		},
	}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, nil, "http://app")
	mux := newSupportMux(h, withUser(7))

	rr := doJSON(mux, http.MethodPost, "/api/notes/4/attachments", `{"attachment":"http://a.pdf"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "http://a.pdf") {
		t.Fatalf("add code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodDelete, "/api/notes/4/attachments", `{"attachment":"http://a.pdf"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/notes/4/attachments", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method code=%d", rr.Code)
	}
}

func TestNoteHandler_AddAndDeleteAttachments(t *testing.T) {
	note := &models.Note{ID: 4, UserID: 7, Title: "T", Content: "C", Attachments: []string{"http://a.pdf"}}
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return true, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			note.Attachments = append([]string{}, n.Attachments...)
			return nil
		},
	}
	h := handler.NewNoteHandler(service.NewNoteService(repo))
	resp, err := h.AddNoteAttachments(context.Background(), 4, 7, []string{"http://b.docx"})
	if err != nil || len(resp.Attachments) != 2 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	resp, err = h.DeleteNoteAttachment(context.Background(), 4, 7, "http://a.pdf")
	if err != nil || len(resp.Attachments) != 1 || resp.Attachments[0] != "http://b.docx" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if _, err := h.AddNoteAttachments(context.Background(), 4, 7, []string{"1", "2", "3", "4", "5", "6"}); err == nil {
		t.Fatal("expected validation error")
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
