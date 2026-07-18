package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"metarang/grpc-gateway/internal/middleware"
	pbAuth "metarang/shared/pb/auth"
	pbCommon "metarang/shared/pb/common"
	pbSupport "metarang/shared/pb/support"
)

type SupportHandler struct {
	ticketClient       pbSupport.TicketServiceClient
	reportClient       pbSupport.ReportServiceClient
	userEventClient    pbSupport.UserEventReportServiceClient
	noteClient         pbSupport.NoteServiceClient
	authClient         pbAuth.AuthServiceClient
	storageServiceAddr string
	appURL             string
}

func NewSupportHandler(supportConn, authConn *grpc.ClientConn, storageServiceAddr, appURL string) *SupportHandler {
	return &SupportHandler{
		ticketClient:       pbSupport.NewTicketServiceClient(supportConn),
		reportClient:       pbSupport.NewReportServiceClient(supportConn),
		userEventClient:    pbSupport.NewUserEventReportServiceClient(supportConn),
		noteClient:         pbSupport.NewNoteServiceClient(supportConn),
		authClient:         pbAuth.NewAuthServiceClient(authConn),
		storageServiceAddr: storageServiceAddr,
		appURL:             appURL,
	}
}

// Helper function to get authenticated user ID from context (set by auth middleware)
func (h *SupportHandler) getAuthUserID(r *http.Request) (uint64, error) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		return 0, err
	}
	return userCtx.UserID, nil
}

// ============================================================================
// Tickets API
// ============================================================================

// ListTickets handles GET /api/tickets
// Query params: page, cursor, recieved (bool)
func (h *SupportHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Parse pagination
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

	grpcReq := &pbSupport.GetTicketsRequest{
		UserId: userID,
		Pagination: &pbCommon.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
		Received: received,
	}

	resp, err := h.ticketClient.GetTickets(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Convert to Laravel-compatible format
	tickets := make([]map[string]interface{}, 0, len(resp.Tickets))
	for _, ticket := range resp.Tickets {
		tickets = append(tickets, formatTicketResource(ticket, false))
	}

	// Simple pagination response (matching Laravel simplePaginate)
	response := map[string]interface{}{
		"data": tickets,
	}
	if len(tickets) == int(perPage) {
		response["next_page_url"] = r.URL.Path + "?page=" + strconv.Itoa(int(page+1))
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateTicket handles POST /api/tickets
func (h *SupportHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
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
		attachment, err = uploadTicketAttachment(r, h.storageServiceAddr, h.appURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		var req struct {
			Title      string  `json:"title"`
			Content    string  `json:"content"`
			Attachment string  `json:"attachment"`
			Reciever   *uint64 `json:"reciever"`
			Department string  `json:"department"`
		}
		if err := decodeRequestBody(r, &req); err != nil {
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
		attachment = req.Attachment
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

	resp, err := h.ticketClient.CreateTicket(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, formatTicketResponse(resp))
}

// GetTicket handles GET /api/tickets/{ticket}
func (h *SupportHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	ticketIDStr := extractIDFromPath(r.URL.Path, "/api/tickets/")
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}

	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	grpcReq := &pbSupport.GetTicketRequest{
		TicketId: ticketID,
		UserId:   userID,
	}

	resp, err := h.ticketClient.GetTicket(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatTicketResource(resp, true))
}

// UpdateTicket handles PUT/PATCH /api/tickets/{ticket}
func (h *SupportHandler) UpdateTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	ticketIDStr := extractIDFromPath(r.URL.Path, "/api/tickets/")
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
		if _, hdr, fileErr := r.FormFile("attachment"); fileErr == nil && hdr != nil {
			attachment, err = uploadTicketAttachment(r, h.storageServiceAddr, h.appURL)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	} else {
		var req struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			Attachment string `json:"attachment"`
		}
		if err := decodeRequestBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		attachment = req.Attachment
	}

	grpcReq := &pbSupport.UpdateTicketRequest{
		TicketId:   ticketID,
		UserId:     userID,
		Title:      title,
		Content:    content,
		Attachment: attachment,
	}

	resp, err := h.ticketClient.UpdateTicket(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatTicketResponse(resp))
}

// AddTicketResponse handles POST /api/tickets/response/{ticket}
func (h *SupportHandler) AddTicketResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	ticketIDStr := extractIDFromPath(r.URL.Path, "/api/tickets/response/")
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
		attachment, err = uploadTicketAttachment(r, h.storageServiceAddr, h.appURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		var req struct {
			Response   string `json:"response"`
			Attachment string `json:"attachment"`
		}
		if err := decodeRequestBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		responseText = req.Response
		attachment = req.Attachment
	}

	if responseText == "" {
		writeError(w, http.StatusBadRequest, "response is required")
		return
	}

	grpcReq := &pbSupport.AddResponseRequest{
		TicketId:   ticketID,
		UserId:     userID,
		Response:   responseText,
		Attachment: attachment,
	}

	resp, err := h.ticketClient.AddResponse(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatTicketResponse(resp))
}

// formatTicketResource converts a proto ticket to Laravel TicketResource-compatible JSON.
// includeResponses mirrors Laravel's whenLoaded('responses') on the show endpoint.
func formatTicketResource(resp *pbSupport.TicketResponse, includeResponses bool) map[string]interface{} {
	dateStr, timeStr := splitJalaliDateTime(resp.UpdatedAt)
	ticketMap := map[string]interface{}{
		"id":         resp.Id,
		"title":      resp.Title,
		"content":    resp.Content,
		"code":       resp.Code,
		"status":     resp.Status,
		"attachment": resp.Attachment,
		"date":       dateStr,
		"time":       timeStr,
	}

	if resp.Sender != nil {
		ticketMap["sender"] = map[string]interface{}{
			"name":          resp.Sender.Name,
			"code":          resp.Sender.Code,
			"profile-photo": resp.Sender.ProfilePhoto,
		}
	}

	if resp.Receiver != nil {
		ticketMap["reciever"] = map[string]interface{}{
			"name":          resp.Receiver.Name,
			"code":          resp.Receiver.Code,
			"profile-photo": resp.Receiver.ProfilePhoto,
		}
	}

	if resp.Department != "" {
		ticketMap["department"] = resp.Department
	}

	if includeResponses {
		ticketMap["responses"] = formatTicketResponseItems(resp.Responses)
	}

	return ticketMap
}

func formatTicketResponse(resp *pbSupport.TicketResponse) map[string]interface{} {
	return formatTicketResource(resp, false)
}

func formatTicketResponseItems(items []*pbSupport.TicketResponseItem) []map[string]interface{} {
	responses := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		dateStr, timeStr := splitJalaliDateTime(item.CreatedAt)
		responses = append(responses, map[string]interface{}{
			"id":             item.Id,
			"ticket_id":      strconv.FormatUint(item.TicketId, 10),
			"response":       item.Response,
			"attachment":     item.Attachment,
			"responser_id":   item.ResponserId,
			"responser_name": item.ResponserName,
			"date":           dateStr,
			"time":           timeStr,
		})
	}
	return responses
}

// CloseTicket handles GET /api/tickets/close/{ticket}
func (h *SupportHandler) CloseTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	ticketIDStr := extractIDFromPath(r.URL.Path, "/api/tickets/close/")
	if ticketIDStr == "" {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}

	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	grpcReq := &pbSupport.CloseTicketRequest{
		TicketId: ticketID,
		UserId:   userID,
	}

	resp, err := h.ticketClient.CloseTicket(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatTicketResource(resp, false))
}

// ============================================================================
// Reports API
// ============================================================================

// ListReports handles GET /api/reports
func (h *SupportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
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

	grpcReq := &pbSupport.GetReportsRequest{
		UserId: userID,
		Pagination: &pbCommon.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.reportClient.GetReports(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	reports := make([]map[string]interface{}, 0, len(resp.Reports))
	for _, report := range resp.Reports {
		reports = append(reports, formatReportResponse(report, h.appURL))
	}

	response := map[string]interface{}{
		"data": reports,
	}
	if len(reports) == int(perPage) {
		nextURL := r.URL.Path + "?page=" + strconv.Itoa(int(page+1))
		response["next_page_url"] = nextURL
		response["links"] = map[string]interface{}{
			"next": nextURL,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateReport handles POST /api/reports
func (h *SupportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
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
		imagePaths, err = uploadReportAttachments(r, h.storageServiceAddr, h.appURL)
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
		if err := decodeRequestBody(r, &req); err != nil {
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

	grpcReq := &pbSupport.CreateReportRequest{
		UserId:         userID,
		ReportableType: subject,
		Reason:         title,
		Description:    content,
		Url:            url,
		ImagePaths:     imagePaths,
	}

	resp, err := h.reportClient.CreateReport(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, formatReportResponse(resp, h.appURL))
}

// GetReport handles GET /api/reports/{report}
func (h *SupportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	reportIDStr := extractIDFromPath(r.URL.Path, "/api/reports/")
	if reportIDStr == "" {
		writeError(w, http.StatusBadRequest, "report_id is required")
		return
	}

	reportID, err := strconv.ParseUint(reportIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report_id")
		return
	}

	grpcReq := &pbSupport.GetReportRequest{
		ReportId: reportID,
		UserId:   userID,
	}

	resp, err := h.reportClient.GetReport(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatReportResponse(resp, h.appURL))
}

// ============================================================================
// Notes API
// ============================================================================

// ListNotes handles GET /api/notes
func (h *SupportHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	grpcReq := &pbSupport.GetNotesRequest{
		UserId: userID,
	}

	resp, err := h.noteClient.GetNotes(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	notes := make([]map[string]interface{}, 0, len(resp.Notes))
	for _, note := range resp.Notes {
		notes = append(notes, formatNoteResponse(note))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": notes})
}

// CreateNote handles POST /api/notes
func (h *SupportHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	title, content, err := parseNoteFormFields(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachment := ""
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		url, _, attachErr := resolveNoteAttachmentURL(r, h.storageServiceAddr, h.appURL)
		if attachErr != nil {
			writeError(w, http.StatusBadRequest, attachErr.Error())
			return
		}
		attachment = url
	} else {
		var req struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			Attachment string `json:"attachment"`
		}
		if err := decodeRequestBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		attachment = req.Attachment
	}

	if title == "" || content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	grpcReq := &pbSupport.CreateNoteRequest{
		UserId:  userID,
		Title:   title,
		Content: content,
	}
	if attachment != "" {
		grpcReq.Attachments = []string{attachment}
	}

	resp, err := h.noteClient.CreateNote(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, formatNoteResponse(resp))
}

// GetNote handles GET /api/notes/{note}
func (h *SupportHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	noteIDStr := extractIDFromPath(r.URL.Path, "/api/notes/")
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return
	}

	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return
	}

	grpcReq := &pbSupport.GetNoteRequest{
		NoteId: noteID,
		UserId: userID,
	}

	resp, err := h.noteClient.GetNote(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

// UpdateNote handles PUT/PATCH /api/notes/{note} (also POST + _method=put|patch)
func (h *SupportHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	if m := EffectiveHTTPMethod(r); m != http.MethodPut && m != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	noteIDStr := extractIDFromPath(r.URL.Path, "/api/notes/")
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

	attachment := ""
	updateAttachment := false
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		url, clear, attachErr := resolveNoteAttachmentURL(r, h.storageServiceAddr, h.appURL)
		if attachErr != nil {
			writeError(w, http.StatusBadRequest, attachErr.Error())
			return
		}
		if clear {
			attachment = ""
			updateAttachment = true
		} else if url != "" {
			attachment = url
			updateAttachment = true
		}
	} else {
		var req struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			Attachment string `json:"attachment"`
		}
		if err := decodeRequestBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
		title = req.Title
		content = req.Content
		attachment = req.Attachment
		updateAttachment = true
	}

	grpcReq := &pbSupport.UpdateNoteRequest{
		NoteId:  noteID,
		UserId:  userID,
		Title:   title,
		Content: content,
	}
	if updateAttachment {
		if attachment != "" {
			grpcReq.Attachments = []string{attachment}
		} else {
			grpcReq.Attachments = []string{}
		}
	} else {
		existing, getErr := h.noteClient.GetNote(r.Context(), &pbSupport.GetNoteRequest{
			NoteId: noteID,
			UserId: userID,
		})
		if getErr != nil {
			writeGRPCError(w, getErr)
			return
		}
		grpcReq.Attachments = existing.Attachments
	}

	resp, err := h.noteClient.UpdateNote(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, formatNoteResponse(resp))
}

// DeleteNote handles DELETE /api/notes/{note}
func (h *SupportHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getAuthUserID(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	noteIDStr := extractIDFromPath(r.URL.Path, "/api/notes/")
	if noteIDStr == "" {
		writeError(w, http.StatusBadRequest, "note_id is required")
		return
	}

	noteID, err := strconv.ParseUint(noteIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid note_id")
		return
	}

	grpcReq := &pbSupport.DeleteNoteRequest{
		NoteId: noteID,
		UserId: userID,
	}

	_, err = h.noteClient.DeleteNote(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// formatNoteResponse converts a proto note to Laravel/frontend-compatible JSON.
// Frontend expects attachments as an array; DB stores a single attachment URL.
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

// formatReportResponse converts a proto report to Laravel ReportResource-compatible JSON.
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
