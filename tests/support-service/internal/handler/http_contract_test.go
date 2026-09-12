package handler_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbCommon "metarang/shared/pb/common"
	pbSupport "metarang/shared/pb/support"
	authpkg "metarang/shared/pkg/auth"
	"metarang/support-service/internal/handler"
)

type mockTicketAPI struct {
	GetTicketsFunc   func(context.Context, *pbSupport.GetTicketsRequest) (*pbSupport.TicketsResponse, error)
	CreateTicketFunc func(context.Context, *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error)
	GetTicketFunc    func(context.Context, *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error)
	UpdateTicketFunc func(context.Context, *pbSupport.UpdateTicketRequest) (*pbSupport.TicketResponse, error)
	AddResponseFunc  func(context.Context, *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error)
	CloseTicketFunc  func(context.Context, *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error)
}

func (m *mockTicketAPI) GetTickets(ctx context.Context, req *pbSupport.GetTicketsRequest) (*pbSupport.TicketsResponse, error) {
	if m.GetTicketsFunc != nil {
		return m.GetTicketsFunc(ctx, req)
	}
	return &pbSupport.TicketsResponse{}, nil
}
func (m *mockTicketAPI) CreateTicket(ctx context.Context, req *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error) {
	if m.CreateTicketFunc != nil {
		return m.CreateTicketFunc(ctx, req)
	}
	return &pbSupport.TicketResponse{}, nil
}
func (m *mockTicketAPI) GetTicket(ctx context.Context, req *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error) {
	if m.GetTicketFunc != nil {
		return m.GetTicketFunc(ctx, req)
	}
	return &pbSupport.TicketResponse{}, nil
}
func (m *mockTicketAPI) UpdateTicket(ctx context.Context, req *pbSupport.UpdateTicketRequest) (*pbSupport.TicketResponse, error) {
	if m.UpdateTicketFunc != nil {
		return m.UpdateTicketFunc(ctx, req)
	}
	return &pbSupport.TicketResponse{}, nil
}
func (m *mockTicketAPI) AddResponse(ctx context.Context, req *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error) {
	if m.AddResponseFunc != nil {
		return m.AddResponseFunc(ctx, req)
	}
	return &pbSupport.TicketResponse{}, nil
}
func (m *mockTicketAPI) CloseTicket(ctx context.Context, req *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error) {
	if m.CloseTicketFunc != nil {
		return m.CloseTicketFunc(ctx, req)
	}
	return &pbSupport.TicketResponse{}, nil
}

type mockReportAPI struct {
	GetReportsFunc   func(context.Context, *pbSupport.GetReportsRequest) (*pbSupport.ReportsResponse, error)
	CreateReportFunc func(context.Context, *pbSupport.CreateReportRequest) (*pbSupport.ReportResponse, error)
	GetReportFunc    func(context.Context, *pbSupport.GetReportRequest) (*pbSupport.ReportResponse, error)
}

func (m *mockReportAPI) GetReports(ctx context.Context, req *pbSupport.GetReportsRequest) (*pbSupport.ReportsResponse, error) {
	if m.GetReportsFunc != nil {
		return m.GetReportsFunc(ctx, req)
	}
	return &pbSupport.ReportsResponse{}, nil
}
func (m *mockReportAPI) CreateReport(ctx context.Context, req *pbSupport.CreateReportRequest) (*pbSupport.ReportResponse, error) {
	if m.CreateReportFunc != nil {
		return m.CreateReportFunc(ctx, req)
	}
	return &pbSupport.ReportResponse{}, nil
}
func (m *mockReportAPI) GetReport(ctx context.Context, req *pbSupport.GetReportRequest) (*pbSupport.ReportResponse, error) {
	if m.GetReportFunc != nil {
		return m.GetReportFunc(ctx, req)
	}
	return &pbSupport.ReportResponse{}, nil
}

type mockNoteAPI struct {
	GetNotesFunc   func(context.Context, *pbSupport.GetNotesRequest) (*pbSupport.NotesResponse, error)
	CreateNoteFunc func(context.Context, *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error)
	GetNoteFunc    func(context.Context, *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error)
	UpdateNoteFunc func(context.Context, *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error)
	DeleteNoteFunc func(context.Context, *pbSupport.DeleteNoteRequest) (*pbCommon.Empty, error)
}

func (m *mockNoteAPI) GetNotes(ctx context.Context, req *pbSupport.GetNotesRequest) (*pbSupport.NotesResponse, error) {
	if m.GetNotesFunc != nil {
		return m.GetNotesFunc(ctx, req)
	}
	return &pbSupport.NotesResponse{}, nil
}
func (m *mockNoteAPI) CreateNote(ctx context.Context, req *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error) {
	if m.CreateNoteFunc != nil {
		return m.CreateNoteFunc(ctx, req)
	}
	return &pbSupport.NoteResponse{}, nil
}
func (m *mockNoteAPI) GetNote(ctx context.Context, req *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error) {
	if m.GetNoteFunc != nil {
		return m.GetNoteFunc(ctx, req)
	}
	return &pbSupport.NoteResponse{}, nil
}
func (m *mockNoteAPI) UpdateNote(ctx context.Context, req *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error) {
	if m.UpdateNoteFunc != nil {
		return m.UpdateNoteFunc(ctx, req)
	}
	return &pbSupport.NoteResponse{}, nil
}
func (m *mockNoteAPI) DeleteNote(ctx context.Context, req *pbSupport.DeleteNoteRequest) (*pbCommon.Empty, error) {
	if m.DeleteNoteFunc != nil {
		return m.DeleteNoteFunc(ctx, req)
	}
	return &pbCommon.Empty{}, nil
}

func identityMW(next http.Handler) http.Handler { return next }

func withUser(userID uint64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: userID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newSupportMux(h *handler.HTTPSupportHandler, authMW func(http.Handler) http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterHTTPRoutes(mux, authMW)
	return mux
}

func doJSON(mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func sampleTicket() *pbSupport.TicketResponse {
	return &pbSupport.TicketResponse{
		Id: 5, Title: "Help", Content: "Please", Code: 100001, Status: 0,
		Department: "technical_support", Attachment: "a.png",
		UpdatedAt: "1403/01/01 12:00:00", CreatedAt: "1403/01/01 11:00:00",
		Sender:   &pbCommon.UserBasic{Id: 7, Name: "Alice", Code: "A1", ProfilePhoto: "sp.jpg"},
		Receiver: &pbCommon.UserBasic{Id: 8, Name: "Bob", Code: "B1", ProfilePhoto: "rp.jpg"},
		Responses: []*pbSupport.TicketResponseItem{{
			Id: 1, TicketId: 5, Response: "hi", Attachment: "", ResponserName: "Bob", ResponserId: 8,
			CreatedAt: "1403/01/01 12:30:00",
		}},
	}
}

func TestHTTPContract_HealthMethodNotAllowedAndUnauthenticated(t *testing.T) {
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://localhost:8000")
	mux := newSupportMux(h, identityMW)

	rr := doJSON(mux, http.MethodGet, "/health", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("health code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodDelete, "/api/tickets", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tickets method code=%d", rr.Code)
	}
	rr = doJSON(mux, http.MethodDelete, "/api/reports", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("reports method code=%d", rr.Code)
	}
	rr = doJSON(mux, http.MethodDelete, "/api/notes", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("notes method code=%d", rr.Code)
	}

	muxAuth := newSupportMux(h, withUser(7))
	rr = doJSON(muxAuth, http.MethodPost, "/api/tickets/5", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ticket item post code=%d", rr.Code)
	}
	rr = doJSON(muxAuth, http.MethodPut, "/api/reports/5", "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("report put code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodGet, "/api/tickets", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth list tickets code=%d", rr.Code)
	}
}

func TestHTTPContract_TicketCRUDPaginationAndAliases(t *testing.T) {
	var listed *pbSupport.GetTicketsRequest
	tickets := &mockTicketAPI{
		GetTicketsFunc: func(_ context.Context, req *pbSupport.GetTicketsRequest) (*pbSupport.TicketsResponse, error) {
			cp := *req
			listed = &cp
			return &pbSupport.TicketsResponse{
				Tickets: []*pbSupport.TicketResponse{sampleTicket(), sampleTicket()},
			}, nil
		},
		CreateTicketFunc: func(_ context.Context, req *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error) {
			if req.UserId != 7 || req.Title != "Title" || req.ReceiverId != 9 {
				t.Fatalf("create=%+v", req)
			}
			return sampleTicket(), nil
		},
		GetTicketFunc: func(_ context.Context, req *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error) {
			if req.TicketId != 5 {
				t.Fatalf("get=%+v", req)
			}
			return sampleTicket(), nil
		},
		UpdateTicketFunc: func(_ context.Context, req *pbSupport.UpdateTicketRequest) (*pbSupport.TicketResponse, error) {
			if req.Title != "nt" || req.Attachment != "x.png" {
				t.Fatalf("upd=%+v", req)
			}
			out := sampleTicket()
			out.Title = req.Title
			return out, nil
		},
		AddResponseFunc: func(_ context.Context, req *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error) {
			if req.Response != "reply" {
				t.Fatalf("resp=%+v", req)
			}
			return sampleTicket(), nil
		},
		CloseTicketFunc: func(_ context.Context, req *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error) {
			out := sampleTicket()
			out.Status = 5
			return out, nil
		},
	}
	h := handler.NewHTTPSupportHandler(tickets, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://app.test")
	mux := newSupportMux(h, withUser(7))

	rr := doJSON(mux, http.MethodGet, "/api/tickets?page=2&per_page=2&recieved=true", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rr.Code, rr.Body.String())
	}
	if listed == nil || listed.Pagination.Page != 2 || listed.Pagination.PerPage != 2 || !listed.Received {
		t.Fatalf("listed=%+v", listed)
	}
	if !strings.Contains(rr.Body.String(), `"next_page_url"`) {
		t.Fatalf("expected next_page_url %s", rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/tickets", `{"title":"Title","content":"Body","reciever":9}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/support/tickets/5", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"responses"`) {
		t.Fatalf("get code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPut, "/api/tickets/5", `{"title":"nt","content":"nc","attachment":"x.png"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"nt"`) {
		t.Fatalf("update code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPatch, "/api/tickets/5", `{"title":"nt","content":"nc","attachment":"x.png"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/tickets/response/5", `{"response":"reply"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("add response code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/support/tickets/close/5", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":5`) {
		t.Fatalf("close code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPContract_TicketValidationAndGRPCMapping(t *testing.T) {
	tickets := &mockTicketAPI{
		GetTicketFunc: func(context.Context, *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "nope")
		},
		CreateTicketFunc: func(context.Context, *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.Internal, "boom")
		},
		AddResponseFunc: func(context.Context, *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "cannot respond to closed ticket")
		},
		UpdateTicketFunc: func(context.Context, *pbSupport.UpdateTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "bad")
		},
		CloseTicketFunc: func(context.Context, *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "exists")
		},
	}
	h := handler.NewHTTPSupportHandler(tickets, &mockReportAPI{}, &mockNoteAPI{}, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(7))

	if doJSON(mux, http.MethodPost, "/api/tickets", `{`).Code != http.StatusBadRequest {
		t.Fatal("bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets", "").Code != http.StatusBadRequest {
		t.Fatal("empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets", `{"title":"","content":""}`).Code != http.StatusBadRequest {
		t.Fatal("required fields")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/abc", "").Code != http.StatusBadRequest {
		t.Fatal("invalid id")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/5", "").Code != http.StatusForbidden {
		t.Fatal("permission")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets", `{"title":"t","content":"c"}`).Code != http.StatusInternalServerError {
		t.Fatal("internal")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/5", `{"response":"x"}`).Code != http.StatusPreconditionFailed {
		t.Fatal("failed precondition")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/response/5", `{"response":""}`).Code != http.StatusBadRequest {
		t.Fatal("empty response")
	}
	if doJSON(mux, http.MethodPut, "/api/tickets/5", `{"title":"t","content":"c"}`).Code != http.StatusUnprocessableEntity {
		t.Fatal("invalid argument")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/close/5", "").Code != http.StatusConflict {
		t.Fatal("already exists mapping")
	}
	if doJSON(mux, http.MethodPost, "/api/tickets/close/5", "").Code != http.StatusMethodNotAllowed {
		t.Fatal("close method")
	}
	if doJSON(mux, http.MethodGet, "/api/tickets/response/abc", "").Code != http.StatusMethodNotAllowed {
		t.Fatal("response get")
	}
}

func TestHTTPContract_ReportsCRUDAndErrors(t *testing.T) {
	var listed *pbSupport.GetReportsRequest
	reports := &mockReportAPI{
		GetReportsFunc: func(_ context.Context, req *pbSupport.GetReportsRequest) (*pbSupport.ReportsResponse, error) {
			cp := *req
			listed = &cp
			return &pbSupport.ReportsResponse{
				Reports: []*pbSupport.ReportResponse{
					{Id: 1, Reason: "t", ReportableType: "displayError", Description: "d", Url: "https://x", CreatedAt: "1403/01/01 00:00:00", ImagePaths: []string{"a.png"}},
					{Id: 2, Reason: "t2", ReportableType: "FPSError", Description: "d2", CreatedAt: "1403/01/02 00:00:00"},
				},
			}, nil
		},
		CreateReportFunc: func(_ context.Context, req *pbSupport.CreateReportRequest) (*pbSupport.ReportResponse, error) {
			if req.ReportableType != "displayError" || req.Reason != "t" {
				t.Fatalf("create=%+v", req)
			}
			return &pbSupport.ReportResponse{Id: 9, Reason: req.Reason, ReportableType: req.ReportableType, Description: req.Description, Url: req.Url, ImagePaths: req.ImagePaths}, nil
		},
		GetReportFunc: func(_ context.Context, req *pbSupport.GetReportRequest) (*pbSupport.ReportResponse, error) {
			if req.ReportId != 9 {
				t.Fatalf("get=%+v", req)
			}
			return &pbSupport.ReportResponse{Id: 9, Reason: "t", ReportableType: "displayError", Description: "d", Url: "https://x"}, nil
		},
	}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, reports, &mockNoteAPI{}, nil, "http://app.test/")
	mux := newSupportMux(h, withUser(3))

	rr := doJSON(mux, http.MethodGet, "/api/support/reports?page=2&per_page=2", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rr.Code, rr.Body.String())
	}
	if listed == nil || listed.Pagination.Page != 2 || listed.Pagination.PerPage != 2 {
		t.Fatalf("listed=%+v", listed)
	}
	if !strings.Contains(rr.Body.String(), `"next_page_url"`) || !strings.Contains(rr.Body.String(), `/uploads/a.png`) {
		t.Fatalf("body=%s", rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/reports", `{"subject":"displayError","title":"t","content":"c","url":"https://x","attachments":["a.png"]}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/reports/9", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get code=%d body=%s", rr.Code, rr.Body.String())
	}

	if doJSON(mux, http.MethodPost, "/api/reports", `{`).Code != http.StatusBadRequest {
		t.Fatal("bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/reports", "").Code != http.StatusBadRequest {
		t.Fatal("empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/reports", `{"subject":"","title":"","content":"","url":""}`).Code != http.StatusBadRequest {
		t.Fatal("required")
	}
	if doJSON(mux, http.MethodGet, "/api/reports/abc", "").Code != http.StatusBadRequest {
		t.Fatal("invalid id")
	}
}

func TestHTTPContract_NotesCRUDAndMethodSpoof(t *testing.T) {
	notes := &mockNoteAPI{
		GetNotesFunc: func(_ context.Context, req *pbSupport.GetNotesRequest) (*pbSupport.NotesResponse, error) {
			return &pbSupport.NotesResponse{Notes: []*pbSupport.NoteResponse{{Id: 1, Title: "A", Content: "B", Date: "1403/01/01", Time: "10:00:00", Attachments: []string{"n.png"}}}}, nil
		},
		CreateNoteFunc: func(_ context.Context, req *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error) {
			if req.Title != "T" || len(req.Attachments) != 1 {
				t.Fatalf("create=%+v", req)
			}
			return &pbSupport.NoteResponse{Id: 2, Title: req.Title, Content: req.Content, Attachments: req.Attachments, Date: "1403/01/01", Time: "11:00:00"}, nil
		},
		GetNoteFunc: func(_ context.Context, req *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error) {
			return &pbSupport.NoteResponse{Id: req.NoteId, Title: "T", Content: "C", Date: "1403/01/01", Time: "11:00:00"}, nil
		},
		UpdateNoteFunc: func(_ context.Context, req *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error) {
			return &pbSupport.NoteResponse{Id: req.NoteId, Title: req.Title, Content: req.Content, Attachments: req.Attachments, Date: "1403/01/01", Time: "12:00:00"}, nil
		},
		DeleteNoteFunc: func(context.Context, *pbSupport.DeleteNoteRequest) (*pbCommon.Empty, error) {
			return &pbCommon.Empty{}, nil
		},
	}
	h := handler.NewHTTPSupportHandler(&mockTicketAPI{}, &mockReportAPI{}, notes, nil, "http://localhost:8000")
	mux := newSupportMux(h, withUser(4))

	rr := doJSON(mux, http.MethodGet, "/api/notes", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"n.png"`) {
		t.Fatalf("list code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/notes", `{"title":"T","content":"C","attachment":"n.png"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/notes/2", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPut, "/api/notes/2", `{"title":"N","content":"NC","attachment":""}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("update code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodDelete, "/api/notes/2", "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete code=%d", rr.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/notes/2?_method=DELETE", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("spoof delete code=%d body=%s", rr.Code, rr.Body.String())
	}

	if doJSON(mux, http.MethodPost, "/api/notes", `{`).Code != http.StatusBadRequest {
		t.Fatal("bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/notes", `{"title":"","content":""}`).Code != http.StatusBadRequest {
		t.Fatal("required")
	}
	if doJSON(mux, http.MethodGet, "/api/notes/abc", "").Code != http.StatusBadRequest {
		t.Fatal("invalid id")
	}
}
