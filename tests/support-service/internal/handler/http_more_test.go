package handler_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbCommon "metarang/shared/pb/common"
	pbSupport "metarang/shared/pb/support"
	"metarang/support-service/internal/handler"
)

func TestHTTPContract_GRPCStatusMappingAndEmptyPaths(t *testing.T) {
	tickets := &mockTicketAPI{
		GetTicketsFunc: func(context.Context, *pbSupport.GetTicketsRequest) (*pbSupport.TicketsResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "token")
		},
		GetTicketFunc: func(context.Context, *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.NotFound, "ticket not found")
		},
		CloseTicketFunc: func(context.Context, *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.Unavailable, "down")
		},
		AddResponseFunc: func(context.Context, *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.Unknown, "mystery")
		},
	}
	reports := &mockReportAPI{
		GetReportFunc: func(context.Context, *pbSupport.GetReportRequest) (*pbSupport.ReportResponse, error) {
			return nil, status.Error(codes.NotFound, "report not found")
		},
		GetReportsFunc: func(context.Context, *pbSupport.GetReportsRequest) (*pbSupport.ReportsResponse, error) {
			return nil, status.Error(codes.Internal, "list")
		},
	}
	notes := &mockNoteAPI{
		GetNoteFunc: func(context.Context, *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "nope")
		},
		DeleteNoteFunc: func(context.Context, *pbSupport.DeleteNoteRequest) (*pbCommon.Empty, error) {
			return nil, status.Error(codes.NotFound, "note not found")
		},
		CreateNoteFunc: func(context.Context, *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "bad")
		},
	}
	h := handler.NewHTTPSupportHandler(tickets, reports, notes, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(1))

	if doJSON(mux, http.MethodGet, "/api/tickets", "").Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated mapping")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/9", "").Code != http.StatusNotFound {
		t.Fatal("ticket not found")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/close/9", "").Code != http.StatusServiceUnavailable {
		t.Fatal("unavailable")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/9", `{"response":"x"}`).Code != http.StatusInternalServerError {
		t.Fatal("unknown mapping")
	}
	if doJSON(mux, http.MethodGet, "/api/reports/9", "").Code != http.StatusNotFound {
		t.Fatal("report not found")
	}
	if doJSON(mux, http.MethodGet, "/api/reports", "").Code != http.StatusInternalServerError {
		t.Fatal("reports internal")
	}
	if doJSON(mux, http.MethodGet, "/api/notes/9", "").Code != http.StatusForbidden {
		t.Fatal("note forbidden")
	}
	if doJSON(mux, http.MethodDelete, "/api/notes/9", "").Code != http.StatusNotFound {
		t.Fatal("note delete not found")
	}
	if doJSON(mux, http.MethodPost, "/api/notes", `{"title":"t","content":"c"}`).Code != http.StatusUnprocessableEntity {
		t.Fatal("note invalid argument")
	}
}

func TestHTTPContract_PathParsingAndJSONDecode(t *testing.T) {
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(1))

	if doJSON(mux, http.MethodPut, "/api/tickets/abc", `{"title":"t","content":"c"}`).Code != http.StatusBadRequest {
		t.Fatal("update invalid id")
	}
	if doJSON(mux, http.MethodPut, "/api/tickets/5", `{`).Code != http.StatusBadRequest {
		t.Fatal("update bad json")
	}
	if doJSON(mux, http.MethodPut, "/api/tickets/5", "").Code != http.StatusBadRequest {
		t.Fatal("update empty")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/abc", `{"response":"x"}`).Code != http.StatusBadRequest {
		t.Fatal("response invalid id")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/5", `{`).Code != http.StatusBadRequest {
		t.Fatal("response bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/5", "").Code != http.StatusBadRequest {
		t.Fatal("response empty")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/close/abc", "").Code != http.StatusBadRequest {
		t.Fatal("close invalid id")
	}
	if doJSON(mux, http.MethodPut, "/api/notes/abc", `{"title":"t","content":"c"}`).Code != http.StatusBadRequest {
		t.Fatal("note update invalid id")
	}
	if doJSON(mux, http.MethodPut, "/api/notes/5", `{`).Code != http.StatusBadRequest {
		t.Fatal("note update bad json")
	}
	if doJSON(mux, http.MethodPut, "/api/notes/5", "").Code != http.StatusBadRequest {
		t.Fatal("note update empty")
	}
	if doJSON(mux, http.MethodDelete, "/api/notes/abc", "").Code != http.StatusBadRequest {
		t.Fatal("note delete invalid id")
	}

	rr := doJSON(mux, http.MethodPost, "/api/notes/5", `{"title":"t","content":"c"}`)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("note post item code=%d", rr.Code)
	}
}

func TestHTTPContract_URLEncodedCreateStillHitsJSONDecoder(t *testing.T) {
	// Production parseTicketFormFields/parseNoteFormFields consume urlencoded bodies,
	// then CreateTicket/CreateNote still JSON-decode the already-read body and return 400.
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(7))

	form := url.Values{}
	form.Set("title", "T")
	form.Set("content", "Body")
	form.Set("department", "technical_support")
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("urlencoded ticket currently returns %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPContract_MultipartCreateTicketStorageNotConfigured(t *testing.T) {
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(7))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "T")
	_ = w.WriteField("content", "C")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "storage service not configured") {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPContract_NoteUpdateKeepsAttachmentsWhenMultipartHasNoFile(t *testing.T) {
	var kept []string
	notes := &mockNoteAPI{
		GetNoteFunc: func(context.Context, *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error) {
			return &pbSupport.NoteResponse{Id: 2, Title: "old", Content: "old", Attachments: []string{"keep.png"}}, nil
		},
		UpdateNoteFunc: func(_ context.Context, req *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error) {
			kept = append([]string{}, req.Attachments...)
			return &pbSupport.NoteResponse{Id: req.NoteId, Title: req.Title, Content: req.Content, Attachments: req.Attachments}, nil
		},
	}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(4))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "N")
	_ = w.WriteField("content", "NC")
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPut, "/api/notes/2", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(kept) != 1 || kept[0] != "keep.png" {
		t.Fatalf("kept=%v", kept)
	}
}

func TestEffectiveHTTPMethod_QueryAndFormSpoof(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if handler.EffectiveHTTPMethod(req) != http.MethodGet {
		t.Fatal("get passthrough")
	}

	req = httptest.NewRequest(http.MethodPost, "/x?_method=patch", nil)
	if handler.EffectiveHTTPMethod(req) != http.MethodPatch {
		t.Fatalf("query spoof=%s", handler.EffectiveHTTPMethod(req))
	}

	form := url.Values{}
	form.Set("_method", "put")
	req = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if handler.EffectiveHTTPMethod(req) != http.MethodPut {
		t.Fatalf("form spoof=%s", handler.EffectiveHTTPMethod(req))
	}
}
