package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/shared/pkg/sentry"
	"metarang/support-service/internal/middleware"
	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"

	pbCommon "metarang/shared/pb/common"
	pbSupport "metarang/shared/pb/support"
)

type ticketAPI interface {
	GetTickets(context.Context, *pbSupport.GetTicketsRequest) (*pbSupport.TicketsResponse, error)
	CreateTicket(context.Context, *pbSupport.CreateTicketRequest) (*pbSupport.TicketResponse, error)
	GetTicket(context.Context, *pbSupport.GetTicketRequest) (*pbSupport.TicketResponse, error)
	UpdateTicket(context.Context, *pbSupport.UpdateTicketRequest) (*pbSupport.TicketResponse, error)
	AddResponse(context.Context, *pbSupport.AddResponseRequest) (*pbSupport.TicketResponse, error)
	CloseTicket(context.Context, *pbSupport.CloseTicketRequest) (*pbSupport.TicketResponse, error)
}

type reportAPI interface {
	GetReports(context.Context, *pbSupport.GetReportsRequest) (*pbSupport.ReportsResponse, error)
	CreateReport(context.Context, *pbSupport.CreateReportRequest) (*pbSupport.ReportResponse, error)
	GetReport(context.Context, *pbSupport.GetReportRequest) (*pbSupport.ReportResponse, error)
}

type noteAPI interface {
	GetNotes(context.Context, *pbSupport.GetNotesRequest) (*pbSupport.NotesResponse, error)
	CreateNote(context.Context, *pbSupport.CreateNoteRequest) (*pbSupport.NoteResponse, error)
	GetNote(context.Context, *pbSupport.GetNoteRequest) (*pbSupport.NoteResponse, error)
	UpdateNote(context.Context, *pbSupport.UpdateNoteRequest) (*pbSupport.NoteResponse, error)
	DeleteNote(context.Context, *pbSupport.DeleteNoteRequest) (*pbCommon.Empty, error)
	AddNoteAttachments(context.Context, uint64, uint64, []string) (*pbSupport.NoteResponse, error)
	DeleteNoteAttachment(context.Context, uint64, uint64, string) (*pbSupport.NoteResponse, error)
}

// HTTPSupportHandler serves Kong-facing REST routes for support-service.
type HTTPSupportHandler struct {
	tickets ticketAPI
	reports reportAPI
	notes   noteAPI
	storage fileStorageUploader
	appURL  string
}

// NewHTTPSupportHandler wraps gRPC support handlers for local HTTP use.
// storage must be a storage-service client; attachment bytes are never stored locally.
func NewHTTPSupportHandler(tickets ticketAPI, reports reportAPI, notes noteAPI, storage fileStorageUploader, appURL string) *HTTPSupportHandler {
	return &HTTPSupportHandler{
		tickets: tickets,
		reports: reports,
		notes:   notes,
		storage: storage,
		appURL:  appURL,
	}
}

func (h *HTTPSupportHandler) getAuthUserID(r *http.Request) (uint64, error) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		return 0, status.Error(codes.Unauthenticated, "Unauthenticated")
	}
	return userCtx.UserID, nil
}

// RegisterHTTPRoutes registers support REST routes and /health.
func (h *HTTPSupportHandler) RegisterHTTPRoutes(mux *http.ServeMux, authMiddleware func(http.Handler) http.Handler) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	ticketsCollection := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.ListTickets(w, r)
		case http.MethodPost:
			h.CreateTicket(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))
	ticketsItem := authMiddleware(http.HandlerFunc(h.handleTicketPath))
	reportsCollection := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.ListReports(w, r)
		case http.MethodPost:
			h.CreateReport(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))
	reportsItem := authMiddleware(http.HandlerFunc(h.GetReport))
	notesCollection := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.ListNotes(w, r)
		case http.MethodPost:
			h.CreateNote(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))
	notesItem := authMiddleware(http.HandlerFunc(h.handleNotePath))

	mux.Handle("/api/tickets", ticketsCollection)
	mux.Handle("/api/tickets/", ticketsItem)

	mux.Handle("/api/reports", reportsCollection)
	mux.Handle("/api/reports/", reportsItem)
	mux.Handle("/api/support/reports", reportsCollection)
	mux.Handle("/api/support/reports/", reportsItem)

	mux.Handle("/api/notes", notesCollection)
	mux.Handle("/api/notes/", notesItem)
}

func (h *HTTPSupportHandler) handleTicketPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.Contains(path, "/response/") {
		h.AddTicketResponse(w, r)
		return
	}
	if strings.Contains(path, "/close/") {
		h.CloseTicket(w, r)
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPatch {
		h.UpdateTicket(w, r)
		return
	}
	if r.Method == http.MethodGet {
		h.GetTicket(w, r)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *HTTPSupportHandler) handleNotePath(w http.ResponseWriter, r *http.Request) {
	if isNoteAttachmentsPath(r.URL.Path) {
		h.handleNoteAttachments(w, r)
		return
	}
	switch EffectiveHTTPMethod(r) {
	case http.MethodDelete:
		h.DeleteNote(w, r)
	case http.MethodPut, http.MethodPatch:
		h.UpdateNote(w, r)
	case http.MethodGet:
		h.GetNote(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func isNoteAttachmentsPath(path string) bool {
	return strings.HasSuffix(strings.TrimSuffix(path, "/"), "/attachments")
}

func (h *HTTPSupportHandler) handleNoteAttachments(w http.ResponseWriter, r *http.Request) {
	switch EffectiveHTTPMethod(r) {
	case http.MethodPost:
		h.AddNoteAttachments(w, r)
	case http.MethodDelete:
		h.DeleteNoteAttachment(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(httpHandler *HTTPSupportHandler, port string, authMiddleware func(http.Handler) http.Handler) error {
	mux := http.NewServeMux()
	httpHandler.RegisterHTTPRoutes(mux, authMiddleware)
	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}

func ticketIDFromPath(path string) string {
	if strings.Contains(path, "/response/") {
		return extractIDFromPath(path, "/api/tickets/response/")
	}
	if strings.Contains(path, "/close/") {
		return extractIDFromPath(path, "/api/tickets/close/")
	}
	return extractIDFromPath(path, "/api/tickets/")
}

func reportIDFromPath(path string) string {
	return extractIDFromPath(path, "/api/reports/", "/api/support/reports/")
}

func noteIDFromPath(path string) string {
	return extractIDFromPath(path, "/api/notes/", "/api/support/notes/")
}

// ListTickets handles GET /api/tickets
func (h *HTTPSupportHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	page := int32(1)
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.ParseInt(p, 10, 32); err == nil {
			page = int32(parsed)
		}
	}
	perPage := int32(10)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.ParseInt(pp, 10, 32); err == nil {
			perPage = int32(parsed)
		}
	}
	received := r.URL.Query().Get("recieved") == "true" || r.URL.Query().Get("recieved") == "1"

	resp, err := h.tickets.GetTickets(r.Context(), &pbSupport.GetTicketsRequest{
		UserId: userID,
		Pagination: &pbCommon.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
		Received: received,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	tickets := make([]map[string]interface{}, 0, len(resp.Tickets))
	for _, ticket := range resp.Tickets {
		tickets = append(tickets, formatTicketResource(ticket, userID, false))
	}

	response := map[string]interface{}{"data": tickets}
	if len(tickets) == int(perPage) {
		response["next_page_url"] = r.URL.Path + "?page=" + strconv.Itoa(int(page+1))
	}
	writeJSON(w, http.StatusOK, response)
}

// CreateTicket handles POST /api/tickets
func (h *HTTPSupportHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	title, content, department, receiverID, err := parseTicketFormFields(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachment := ""
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		urls, uploadErr := uploadTicketAttachments(r, h.storage, h.appURL)
		if uploadErr != nil {
			writeError(w, http.StatusBadRequest, uploadErr.Error())
			return
		}
		attachment, err = encodeTicketAttachments(urls)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		var req struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Attachment  string   `json:"attachment"`
			Attachments []string `json:"attachments"`
			Reciever    *uint64  `json:"reciever"`
			Department  string   `json:"department"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		department = req.Department
		receiverID = req.Reciever
		attachment, err = mergeTicketAttachmentFields(req.Attachment, req.Attachments)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if title == "" || content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	grpcReq := &pbSupport.CreateTicketRequest{
		UserId:     userID,
		Title:      title,
		Content:    content,
		Attachment: attachment,
	}
	if receiverID != nil {
		grpcReq.ReceiverId = *receiverID
	}
	if department != "" {
		grpcReq.Department = department
	}

	resp, err := h.tickets.CreateTicket(r.Context(), grpcReq)
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, formatTicketResource(resp, userID, true))
}

// GetTicket handles GET /api/tickets/{id}
func (h *HTTPSupportHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	ticketIDStr := ticketIDFromPath(r.URL.Path)
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	resp, err := h.tickets.GetTicket(r.Context(), &pbSupport.GetTicketRequest{
		TicketId: ticketID,
		UserId:   userID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatTicketResource(resp, userID, true))
}

// UpdateTicket handles PUT/PATCH /api/tickets/{id}
func (h *HTTPSupportHandler) UpdateTicket(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	ticketIDStr := ticketIDFromPath(r.URL.Path)
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	title, content, _, _, err := parseTicketFormFields(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachment := ""
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		urls, uploadErr := uploadTicketAttachments(r, h.storage, h.appURL)
		if uploadErr != nil {
			writeError(w, http.StatusBadRequest, uploadErr.Error())
			return
		}
		if len(urls) > 0 {
			attachment, err = encodeTicketAttachments(urls)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	} else {
		var req struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Attachment  string   `json:"attachment"`
			Attachments []string `json:"attachments"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		attachment, err = mergeTicketAttachmentFields(req.Attachment, req.Attachments)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	resp, err := h.tickets.UpdateTicket(r.Context(), &pbSupport.UpdateTicketRequest{
		TicketId:   ticketID,
		UserId:     userID,
		Title:      title,
		Content:    content,
		Attachment: attachment,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatTicketResponse(resp, userID))
}

// AddTicketResponse handles POST /api/tickets/response/{id}
func (h *HTTPSupportHandler) AddTicketResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	ticketIDStr := ticketIDFromPath(r.URL.Path)
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	responseText := ""
	attachment := ""
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse multipart form")
			return
		}
		responseText = r.FormValue("response")
		urls, uploadErr := uploadTicketAttachments(r, h.storage, h.appURL)
		if uploadErr != nil {
			writeError(w, http.StatusBadRequest, uploadErr.Error())
			return
		}
		attachment, err = encodeTicketAttachments(urls)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		var req struct {
			Response    string   `json:"response"`
			Attachment  string   `json:"attachment"`
			Attachments []string `json:"attachments"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		responseText = req.Response
		attachment, err = mergeTicketAttachmentFields(req.Attachment, req.Attachments)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if responseText == "" {
		writeError(w, http.StatusBadRequest, "response is required")
		return
	}

	resp, err := h.tickets.AddResponse(r.Context(), &pbSupport.AddResponseRequest{
		TicketId:   ticketID,
		UserId:     userID,
		Response:   responseText,
		Attachment: attachment,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatTicketResource(resp, userID, true))
}

// CloseTicket handles GET /api/tickets/close/{id}
func (h *HTTPSupportHandler) CloseTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	ticketIDStr := ticketIDFromPath(r.URL.Path)
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	resp, err := h.tickets.CloseTicket(r.Context(), &pbSupport.CloseTicketRequest{
		TicketId: ticketID,
		UserId:   userID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatTicketResource(resp, userID, false))
}

func formatTicketResource(resp *pbSupport.TicketResponse, viewerID uint64, includeThread bool) map[string]interface{} {
	dateStr, timeStr := splitJalaliDateTime(resp.UpdatedAt)
	ticketMap := map[string]interface{}{
		"id":          resp.Id,
		"title":       resp.Title,
		"content":     resp.Content,
		"code":        resp.Code,
		"status":      resp.Status,
		"attachment":  resp.Attachment,
		"attachments": decodeTicketAttachments(resp.Attachment),
		"date":        dateStr,
		"time":        timeStr,
	}
	if resp.Sender != nil {
		ticketMap["sender"] = formatTicketUser(resp.Sender)
	}
	if resp.Receiver != nil {
		ticketMap["reciever"] = formatTicketUser(resp.Receiver)
	}
	if resp.Department != "" {
		ticketMap["department"] = resp.Department
	}
	if includeThread {
		ticketMap["current_user_id"] = viewerID
		ticketMap["responses"] = formatTicketResponseItems(resp.Responses, viewerID)
		ticketMap["messages"] = formatTicketMessages(ticketThreadMessages(resp, viewerID), viewerID)
	}
	return ticketMap
}

func formatTicketResponse(resp *pbSupport.TicketResponse, viewerID uint64) map[string]interface{} {
	return formatTicketResource(resp, viewerID, false)
}

func formatTicketUser(user *pbCommon.UserBasic) map[string]interface{} {
	return map[string]interface{}{
		"id":            user.Id,
		"name":          user.Name,
		"code":          user.Code,
		"profile-photo": user.ProfilePhoto,
	}
}

func ticketThreadMessages(resp *pbSupport.TicketResponse, viewerID uint64) []*pbSupport.TicketResponseItem {
	if len(resp.Messages) > 0 {
		return resp.Messages
	}
	opening := &pbSupport.TicketResponseItem{
		Id:         0,
		TicketId:   resp.Id,
		Response:   resp.Content,
		Attachment: resp.Attachment,
		CreatedAt:  resp.CreatedAt,
		Role:       models.TicketMessageRoleSender,
		Kind:       models.TicketMessageKindOpening,
	}
	if resp.Sender != nil {
		opening.Author = resp.Sender
		opening.ResponserId = resp.Sender.Id
		opening.ResponserName = resp.Sender.Name
		opening.IsMine = resp.Sender.Id == viewerID
	}
	messages := make([]*pbSupport.TicketResponseItem, 0, 1+len(resp.Responses))
	messages = append(messages, opening)
	for _, item := range resp.Responses {
		clone := cloneTicketResponseItem(item)
		if clone.Kind == "" {
			clone.Kind = models.TicketMessageKindReply
		}
		if clone.Role == "" {
			clone.Role = ticketMessageRole(resp, item.ResponserId)
		}
		if clone.Author == nil {
			clone.Author = ticketMessageAuthor(resp, item)
		}
		clone.IsMine = item.ResponserId == viewerID
		messages = append(messages, clone)
	}
	return messages
}

func cloneTicketResponseItem(item *pbSupport.TicketResponseItem) *pbSupport.TicketResponseItem {
	if item == nil {
		return &pbSupport.TicketResponseItem{}
	}
	return &pbSupport.TicketResponseItem{
		Id:            item.Id,
		TicketId:      item.TicketId,
		Response:      item.Response,
		Attachment:    item.Attachment,
		ResponserName: item.ResponserName,
		ResponserId:   item.ResponserId,
		CreatedAt:     item.CreatedAt,
		Author:        item.Author,
		Role:          item.Role,
		IsMine:        item.IsMine,
		Kind:          item.Kind,
	}
}

func ticketMessageRole(resp *pbSupport.TicketResponse, userID uint64) string {
	if resp.Sender != nil && resp.Sender.Id == userID {
		return models.TicketMessageRoleSender
	}
	if resp.Receiver != nil && resp.Receiver.Id == userID {
		return models.TicketMessageRoleReceiver
	}
	return models.TicketMessageRoleStaff
}

func ticketMessageAuthor(resp *pbSupport.TicketResponse, item *pbSupport.TicketResponseItem) *pbCommon.UserBasic {
	if resp.Sender != nil && item.ResponserId == resp.Sender.Id {
		return resp.Sender
	}
	if resp.Receiver != nil && item.ResponserId == resp.Receiver.Id {
		return resp.Receiver
	}
	return &pbCommon.UserBasic{Id: item.ResponserId, Name: item.ResponserName}
}

func formatTicketMessages(items []*pbSupport.TicketResponseItem, viewerID uint64) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		dateStr, timeStr := splitJalaliDateTime(item.CreatedAt)
		kind := item.Kind
		if kind == "" {
			kind = models.TicketMessageKindReply
		}
		authorID := item.ResponserId
		if authorID == 0 && item.Author != nil {
			authorID = item.Author.Id
		}
		msg := map[string]interface{}{
			"id":          item.Id,
			"ticket_id":   strconv.FormatUint(item.TicketId, 10),
			"kind":        kind,
			"text":        item.Response,
			"attachment":  item.Attachment,
			"attachments": decodeTicketAttachments(item.Attachment),
			"is_mine":     authorID == viewerID,
			"role":        item.Role,
			"date":        dateStr,
			"time":        timeStr,
		}
		if item.Author != nil {
			msg["author"] = formatTicketUser(item.Author)
		}
		messages = append(messages, msg)
	}
	return messages
}

func formatTicketResponseItems(items []*pbSupport.TicketResponseItem, viewerID uint64) []map[string]interface{} {
	responses := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		dateStr, timeStr := splitJalaliDateTime(item.CreatedAt)
		row := map[string]interface{}{
			"id":             item.Id,
			"ticket_id":      strconv.FormatUint(item.TicketId, 10),
			"response":       item.Response,
			"attachment":     item.Attachment,
			"attachments":    decodeTicketAttachments(item.Attachment),
			"responser_id":   item.ResponserId,
			"responser_name": item.ResponserName,
			"is_mine":        item.ResponserId == viewerID,
			"role":           item.Role,
			"kind":           item.Kind,
			"date":           dateStr,
			"time":           timeStr,
		}
		if item.Author != nil {
			row["author"] = formatTicketUser(item.Author)
		}
		responses = append(responses, row)
	}
	return responses
}

// ListReports handles GET /api/reports
func (h *HTTPSupportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	page := int32(1)
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.ParseInt(p, 10, 32); err == nil {
			page = int32(parsed)
		}
	}
	perPage := int32(10)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.ParseInt(pp, 10, 32); err == nil {
			perPage = int32(parsed)
		}
	}

	resp, err := h.reports.GetReports(r.Context(), &pbSupport.GetReportsRequest{
		UserId: userID,
		Pagination: &pbCommon.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	reports := make([]map[string]interface{}, 0, len(resp.Reports))
	for _, report := range resp.Reports {
		reports = append(reports, formatReportResponse(report, h.appURL))
	}

	response := map[string]interface{}{"data": reports}
	if len(reports) == int(perPage) {
		nextURL := r.URL.Path + "?page=" + strconv.Itoa(int(page+1))
		response["next_page_url"] = nextURL
		response["links"] = map[string]interface{}{"next": nextURL}
	}
	writeJSON(w, http.StatusOK, response)
}

// CreateReport handles POST /api/reports
func (h *HTTPSupportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var subject, title, content, url string
	var imagePaths []string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		title, content, subject, url, err = parseReportFormFields(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		imagePaths, err = uploadReportAttachments(r, h.storage, h.appURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		var req struct {
			Subject     string   `json:"subject"`
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			URL         string   `json:"url"`
			Attachments []string `json:"attachments"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		subject = req.Subject
		title = req.Title
		content = req.Content
		url = req.URL
		imagePaths = req.Attachments
	}

	if subject == "" || title == "" || content == "" || url == "" {
		writeError(w, http.StatusBadRequest, "subject, title, content, and url are required")
		return
	}

	resp, err := h.reports.CreateReport(r.Context(), &pbSupport.CreateReportRequest{
		UserId:         userID,
		ReportableType: subject,
		Reason:         title,
		Description:    content,
		Url:            url,
		ImagePaths:     imagePaths,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, formatReportResponse(resp, h.appURL))
}

// GetReport handles GET /api/reports/{id}
func (h *HTTPSupportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	reportIDStr := reportIDFromPath(r.URL.Path)
	if reportIDStr == "" {
		writeError(w, http.StatusBadRequest, "report_id is required")
		return
	}
	reportID, err := strconv.ParseUint(reportIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report_id")
		return
	}

	resp, err := h.reports.GetReport(r.Context(), &pbSupport.GetReportRequest{
		ReportId: reportID,
		UserId:   userID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatReportResponse(resp, h.appURL))
}

func formatReportResponse(resp *pbSupport.ReportResponse, appURL string) map[string]interface{} {
	reportMap := map[string]interface{}{
		"id":       strconv.FormatUint(resp.Id, 10),
		"title":    resp.Reason,
		"subject":  resp.ReportableType,
		"content":  resp.Description,
		"datetime": resp.CreatedAt,
	}
	if resp.Url != "" {
		reportMap["url"] = resp.Url
	}
	attachments := make([]string, 0, len(resp.ImagePaths))
	base := strings.TrimRight(appURL, "/")
	for _, path := range resp.ImagePaths {
		path = strings.TrimPrefix(path, "/")
		attachments = append(attachments, base+"/uploads/"+path)
	}
	if len(attachments) > 0 {
		reportMap["attachments"] = attachments
	}
	return reportMap
}

// ListNotes handles GET /api/notes
func (h *HTTPSupportHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	resp, err := h.notes.GetNotes(r.Context(), &pbSupport.GetNotesRequest{UserId: userID})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	notes := make([]map[string]interface{}, 0, len(resp.Notes))
	for _, note := range resp.Notes {
		notes = append(notes, formatNoteResponse(note))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": notes})
}

// CreateNote handles POST /api/notes
func (h *HTTPSupportHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	title, content, err := parseNoteFormFields(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var attachments []string
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		urls, attachErr := uploadNoteAttachmentFiles(r, h.storage, h.appURL)
		if attachErr != nil {
			writeError(w, http.StatusBadRequest, attachErr.Error())
			return
		}
		attachments = urls
	} else {
		var req struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Attachment  string   `json:"attachment"`
			Attachments []string `json:"attachments"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		attachments, err = mergeNoteAttachmentURLs(req.Attachment, req.Attachments)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if title == "" || content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	grpcReq := &pbSupport.CreateNoteRequest{
		UserId:      userID,
		Title:       title,
		Content:     content,
		Attachments: attachments,
	}

	resp, err := h.notes.CreateNote(r.Context(), grpcReq)
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, formatNoteResponse(resp))
}

// GetNote handles GET /api/notes/{id}
func (h *HTTPSupportHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	noteIDStr := noteIDFromPath(r.URL.Path)
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return
	}
	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return
	}

	resp, err := h.notes.GetNote(r.Context(), &pbSupport.GetNoteRequest{
		NoteId: noteID,
		UserId: userID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

// UpdateNote handles PUT/PATCH /api/notes/{id}
func (h *HTTPSupportHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	noteIDStr := noteIDFromPath(r.URL.Path)
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return
	}
	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return
	}

	title, content, err := parseNoteFormFields(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	added, clearAttachments, bodyTitle, bodyContent, isJSON, err := h.prepareNoteUpdateAttachments(r, noteID, userID)
	if err != nil {
		if _, ok := status.FromError(err); ok {
			writeHandlerError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if isJSON {
		title = bodyTitle
		content = bodyContent
	}

	grpcReq := &pbSupport.UpdateNoteRequest{
		NoteId:  noteID,
		UserId:  userID,
		Title:   title,
		Content: content,
	}
	if clearAttachments {
		grpcReq.Attachments = []string{}
	} else if len(added) == 0 {
		existing, getErr := h.notes.GetNote(r.Context(), &pbSupport.GetNoteRequest{
			NoteId: noteID,
			UserId: userID,
		})
		if getErr != nil {
			writeHandlerError(w, getErr)
			return
		}
		grpcReq.Attachments = existing.Attachments
	} else {
		// UpdateNote appends these URLs onto the attachments already stored.
		grpcReq.Attachments = added
	}

	resp, err := h.notes.UpdateNote(r.Context(), grpcReq)
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

// DeleteNote handles DELETE /api/notes/{id}
func (h *HTTPSupportHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	noteIDStr := noteIDFromPath(r.URL.Path)
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return
	}
	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return
	}

	_, err = h.notes.DeleteNote(r.Context(), &pbSupport.DeleteNoteRequest{
		NoteId: noteID,
		UserId: userID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddNoteAttachments handles POST /api/notes/{id}/attachments
// Path param: id (Note ID)
func (h *HTTPSupportHandler) AddNoteAttachments(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	noteID, ok := noteIDFromRequest(w, r)
	if !ok {
		return
	}

	urls, err := h.readNoteAttachmentURLs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.notes.AddNoteAttachments(r.Context(), noteID, userID, urls)
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

// DeleteNoteAttachment handles DELETE /api/notes/{id}/attachments
// Path param: id (Note ID)
// Query params: attachment (URL of the attachment to remove)
func (h *HTTPSupportHandler) DeleteNoteAttachment(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	noteID, ok := noteIDFromRequest(w, r)
	if !ok {
		return
	}

	attachment, err := readDeletedNoteAttachment(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.notes.DeleteNoteAttachment(r.Context(), noteID, userID, attachment)
	if err != nil {
		writeHandlerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

func noteIDFromRequest(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	noteIDStr := noteIDFromPath(r.URL.Path)
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return 0, false
	}
	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return 0, false
	}
	return noteID, true
}

// prepareNoteUpdateAttachments reads files or URLs added during a note edit.
// New files are rejected before upload when they would push the note past 5 attachments.
// An empty attachments[] multipart value with no files clears every attachment.
func (h *HTTPSupportHandler) prepareNoteUpdateAttachments(r *http.Request, noteID, userID uint64) (added []string, clear bool, title, content string, isJSON bool, err error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		headers := collectAttachmentFileHeaders(r)
		if len(headers) == 0 && noteAttachmentsCleared(r) {
			return nil, true, "", "", false, nil
		}
		if len(headers) == 0 {
			return nil, false, "", "", false, nil
		}
		existing, getErr := h.notes.GetNote(r.Context(), &pbSupport.GetNoteRequest{
			NoteId: noteID,
			UserId: userID,
		})
		if getErr != nil {
			return nil, false, "", "", false, getErr
		}
		if len(existing.Attachments)+len(headers) > maxNoteAttachmentCount {
			return nil, false, "", "", false, fmt.Errorf("attachments must not have more than 5 items")
		}
		urls, uploadErr := uploadNoteAttachmentFiles(r, h.storage, h.appURL)
		if uploadErr != nil {
			return nil, false, "", "", false, uploadErr
		}
		return urls, false, "", "", false, nil
	}

	var req struct {
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Attachment  string   `json:"attachment"`
		Attachments []string `json:"attachments"`
	}
	if decodeErr := decodeJSONBody(r, &req); decodeErr != nil {
		if decodeErr == io.EOF {
			return nil, false, "", "", true, fmt.Errorf("request body is required")
		}
		return nil, false, "", "", true, fmt.Errorf("invalid request body")
	}
	urls, mergeErr := mergeNoteAttachmentURLs(req.Attachment, req.Attachments)
	if mergeErr != nil {
		return nil, false, "", "", true, mergeErr
	}
	if len(urls) > 0 {
		existing, getErr := h.notes.GetNote(r.Context(), &pbSupport.GetNoteRequest{
			NoteId: noteID,
			UserId: userID,
		})
		if getErr != nil {
			return nil, false, "", "", true, getErr
		}
		if _, countErr := service.MergeNoteAttachments(existing.Attachments, urls); countErr != nil {
			return nil, false, "", "", true, countErr
		}
	}
	return urls, false, req.Title, req.Content, true, nil
}

func noteAttachmentsCleared(r *http.Request) bool {
	if r == nil || r.MultipartForm == nil {
		return false
	}
	values, ok := r.MultipartForm.Value["attachments[]"]
	if !ok || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func (h *HTTPSupportHandler) readNoteAttachmentURLs(r *http.Request) ([]string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return uploadNoteAttachmentFiles(r, h.storage, h.appURL)
	}

	var req struct {
		Attachment  string   `json:"attachment"`
		Attachments []string `json:"attachments"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("request body is required")
		}
		return nil, fmt.Errorf("invalid request body")
	}
	if len(req.Attachments) > 0 {
		return req.Attachments, nil
	}
	if strings.TrimSpace(req.Attachment) != "" {
		return []string{strings.TrimSpace(req.Attachment)}, nil
	}
	return nil, nil
}

func readDeletedNoteAttachment(r *http.Request) (string, error) {
	if attachment := strings.TrimSpace(r.URL.Query().Get("attachment")); attachment != "" {
		return attachment, nil
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") || contentType == "" {
		var req struct {
			Attachment string `json:"attachment"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			if err == io.EOF {
				return "", nil
			}
			return "", fmt.Errorf("invalid request body")
		}
		return strings.TrimSpace(req.Attachment), nil
	}
	return "", nil
}

func formatNoteResponse(resp *pbSupport.NoteResponse) map[string]interface{} {
	noteMap := map[string]interface{}{
		"id":          resp.Id,
		"title":       resp.Title,
		"content":     resp.Content,
		"date":        resp.Date,
		"time":        resp.Time,
		"attachments": []string{},
	}
	if len(resp.Attachments) > 0 {
		noteMap["attachment"] = resp.Attachments[0]
		noteMap["attachments"] = resp.Attachments
	}
	return noteMap
}
