package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"metarang/dynasty-service/internal/middleware"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	"metarang/shared/pkg/sentry"
)

type dynastyAPI interface {
	GetUserDynasty(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error)
	CreateDynasty(context.Context, *dynastypb.CreateDynastyRequest) (*dynastypb.DynastyResponse, error)
	UpdateDynastyFeature(context.Context, *dynastypb.UpdateDynastyFeatureRequest) (*dynastypb.DynastyResponse, error)
}

type joinRequestAPI interface {
	SendJoinRequest(context.Context, *dynastypb.SendJoinRequestRequest) (*dynastypb.JoinRequestResponse, error)
	GetSentRequests(context.Context, *dynastypb.GetSentRequestsRequest) (*dynastypb.JoinRequestsResponse, error)
	GetReceivedRequests(context.Context, *dynastypb.GetReceivedRequestsRequest) (*dynastypb.JoinRequestsResponse, error)
	GetJoinRequest(context.Context, *dynastypb.GetJoinRequestRequest) (*dynastypb.JoinRequestResponse, error)
	AcceptJoinRequest(context.Context, *dynastypb.AcceptJoinRequestRequest) (*commonpb.Empty, error)
	RejectJoinRequest(context.Context, *dynastypb.RejectJoinRequestRequest) (*commonpb.Empty, error)
	DeleteJoinRequest(context.Context, *dynastypb.DeleteJoinRequestRequest) (*commonpb.Empty, error)
	GetDefaultPermissions(context.Context, *dynastypb.GetDefaultPermissionsRequest) (*dynastypb.DefaultPermissionsResponse, error)
	SearchUsers(context.Context, *dynastypb.SearchUsersRequest) (*dynastypb.SearchUsersResponse, error)
}

type familyAPI interface {
	GetFamily(context.Context, *dynastypb.GetFamilyRequest) (*dynastypb.FamilyResponse, error)
	SetChildPermissions(context.Context, *dynastypb.SetChildPermissionsRequest) (*commonpb.Empty, error)
	UpdateNamedChildPermission(context.Context, uint64, uint64, string, bool) error
}

type prizeAPI interface {
	GetPrizes(context.Context, *dynastypb.GetPrizesRequest) (*dynastypb.PrizesResponse, error)
	GetPrize(context.Context, *dynastypb.GetPrizeRequest) (*dynastypb.PrizeResponse, error)
	ClaimPrize(context.Context, *dynastypb.ClaimPrizeRequest) (*commonpb.Empty, error)
}

// HTTPDynastyHandler serves Kong-facing REST routes for dynasty-service.
type HTTPDynastyHandler struct {
	dynasty     dynastyAPI
	joinRequest joinRequestAPI
	family      familyAPI
	prize       prizeAPI
}

// NewHTTPDynastyHandler wraps local gRPC handlers for HTTP use.
func NewHTTPDynastyHandler(
	dynasty dynastyAPI,
	joinRequest joinRequestAPI,
	family familyAPI,
	prize prizeAPI,
) *HTTPDynastyHandler {
	return &HTTPDynastyHandler{
		dynasty:     dynasty,
		joinRequest: joinRequest,
		family:      family,
		prize:       prize,
	}
}

// RegisterHTTPRoutes registers dynasty REST routes and /health.
func (h *HTTPDynastyHandler) RegisterHTTPRoutes(
	mux *http.ServeMux,
	authMiddleware func(http.Handler) http.Handler,
) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/api/dynasty", authMiddleware(http.HandlerFunc(h.GetDynasty)))
	mux.Handle("/api/dynasty/create/", authMiddleware(http.HandlerFunc(h.CreateDynasty)))

	mux.Handle("/api/dynasty/requests/sent", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetSentRequests(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	mux.Handle("/api/dynasty/requests/sent/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetSentRequest(w, r)
		case http.MethodDelete:
			h.DeleteJoinRequest(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/api/dynasty/requests/recieved", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetReceivedRequests(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	mux.Handle("/api/dynasty/requests/recieved/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetReceivedRequest(w, r)
		case http.MethodPost:
			h.AcceptJoinRequest(w, r)
		case http.MethodDelete:
			h.RejectJoinRequest(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/api/dynasty/add/member/get/permissions", authMiddleware(http.HandlerFunc(h.GetDefaultPermissions)))
	mux.Handle("/api/dynasty/add/member", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.SendJoinRequest(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	mux.Handle("/api/dynasty/search", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.SearchUsers(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	mux.Handle("/api/dynasty/prizes", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetPrizes(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	mux.Handle("/api/dynasty/prizes/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetPrize(w, r)
		case http.MethodPost:
			h.ClaimPrize(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/api/dynasty/children/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.UpdateChildPermissions(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	// Catch-all for /api/dynasty/{id}/update/{feature} and /api/dynasty/{id}/family/{family}.
	mux.Handle("/api/dynasty/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/update/") {
			if r.Method == http.MethodPost {
				h.UpdateDynastyFeature(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		if strings.Contains(path, "/family/") {
			if r.Method == http.MethodGet {
				h.GetFamily(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	})))
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(
	httpHandler *HTTPDynastyHandler,
	port string,
	authMiddleware func(http.Handler) http.Handler,
) error {
	mux := http.NewServeMux()
	httpHandler.RegisterHTTPRoutes(mux, authMiddleware)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}

// GetDynasty handles GET /api/dynasty
func (h *HTTPDynastyHandler) GetDynasty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resp, err := h.dynasty.GetUserDynasty(r.Context(), &dynastypb.GetUserDynastyRequest{
		UserId: userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildDynastyHTTPResponse(resp)})
}

// CreateDynasty handles POST /api/dynasty/create/{feature}
func (h *HTTPDynastyHandler) CreateDynasty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	featureIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/create/")
	if featureIDStr == "" {
		writeError(w, http.StatusBadRequest, "feature_id is required")
		return
	}

	featureID, err := strconv.ParseUint(featureIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature_id")
		return
	}

	resp, err := h.dynasty.CreateDynasty(r.Context(), &dynastypb.CreateDynastyRequest{
		UserId:    userCtx.UserID,
		FeatureId: featureID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": buildDynastyHTTPResponse(resp)})
}

// UpdateDynastyFeature handles POST /api/dynasty/{dynasty}/update/{feature}
func (h *HTTPDynastyHandler) UpdateDynastyFeature(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/dynasty/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "update" {
		writeError(w, http.StatusBadRequest, "invalid path format: expected /api/dynasty/{dynasty}/update/{feature}")
		return
	}

	dynastyID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dynasty_id")
		return
	}

	featureID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature_id")
		return
	}

	resp, err := h.dynasty.UpdateDynastyFeature(r.Context(), &dynastypb.UpdateDynastyFeatureRequest{
		DynastyId: dynastyID,
		FeatureId: featureID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildDynastyHTTPResponse(resp)})
}

// GetFamily handles GET /api/dynasty/{dynasty}/family/{family}
func (h *HTTPDynastyHandler) GetFamily(w http.ResponseWriter, r *http.Request) {
	if _, err := middleware.GetUserFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/dynasty/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "family" {
		writeError(w, http.StatusBadRequest, "invalid path format")
		return
	}

	dynastyID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dynasty_id")
		return
	}

	familyID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid family_id")
		return
	}

	resp, err := h.family.GetFamily(r.Context(), &dynastypb.GetFamilyRequest{
		DynastyId: dynastyID,
		FamilyId:  familyID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildFamilyMembersHTTPResponse(resp)})
}

// GetSentRequests handles GET /api/dynasty/requests/sent
func (h *HTTPDynastyHandler) GetSentRequests(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	resp, err := h.joinRequest.GetSentRequests(r.Context(), &dynastypb.GetSentRequestsRequest{
		UserId: userCtx.UserID,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: 10,
		},
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildSentJoinRequestsHTTPResponse(resp)})
}

// GetReceivedRequests handles GET /api/dynasty/requests/recieved
func (h *HTTPDynastyHandler) GetReceivedRequests(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	resp, err := h.joinRequest.GetReceivedRequests(r.Context(), &dynastypb.GetReceivedRequestsRequest{
		UserId: userCtx.UserID,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: 10,
		},
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildReceivedJoinRequestsHTTPResponse(resp)})
}

// SendJoinRequest handles POST /api/dynasty/add/member
func (h *HTTPDynastyHandler) SendJoinRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		User         uint64          `json:"user"`
		Relationship string          `json:"relationship"`
		Message      string          `json:"message,omitempty"`
		Permissions  map[string]bool `json:"permissions,omitempty"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	var permissions *dynastypb.ChildPermissions
	if req.Relationship == "offspring" && req.Permissions != nil {
		permissions = &dynastypb.ChildPermissions{
			BFR:  req.Permissions["BFR"],
			SF:   req.Permissions["SF"],
			W:    req.Permissions["W"],
			JU:   req.Permissions["JU"],
			DM:   req.Permissions["DM"],
			PIUP: req.Permissions["PIUP"],
			PITC: req.Permissions["PITC"],
			PIC:  req.Permissions["PIC"],
			ESOO: req.Permissions["ESOO"],
			COTB: req.Permissions["COTB"],
		}
	}

	resp, err := h.joinRequest.SendJoinRequest(r.Context(), &dynastypb.SendJoinRequestRequest{
		FromUserId:   userCtx.UserID,
		ToUserId:     req.User,
		Relationship: req.Relationship,
		Message:      req.Message,
		Permissions:  permissions,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": buildSentJoinRequestHTTP(resp)})
}

// AcceptJoinRequest handles POST /api/dynasty/requests/recieved/{joinRequest}
func (h *HTTPDynastyHandler) AcceptJoinRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	requestIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/requests/recieved/")
	if requestIDStr == "" {
		writeError(w, http.StatusBadRequest, "request_id is required")
		return
	}

	requestID, err := strconv.ParseUint(requestIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	_, err = h.joinRequest.AcceptJoinRequest(r.Context(), &dynastypb.AcceptJoinRequestRequest{
		RequestId: requestID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RejectJoinRequest handles DELETE /api/dynasty/requests/recieved/{joinRequest}
func (h *HTTPDynastyHandler) RejectJoinRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	requestIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/requests/recieved/")
	if requestIDStr == "" {
		writeError(w, http.StatusBadRequest, "request_id is required")
		return
	}

	requestID, err := strconv.ParseUint(requestIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	_, err = h.joinRequest.RejectJoinRequest(r.Context(), &dynastypb.RejectJoinRequestRequest{
		RequestId: requestID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetSentRequest handles GET /api/dynasty/requests/sent/{joinRequest}
func (h *HTTPDynastyHandler) GetSentRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	requestIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/requests/sent/")
	if requestIDStr == "" {
		writeError(w, http.StatusBadRequest, "request_id is required")
		return
	}

	requestID, err := strconv.ParseUint(requestIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	resp, err := h.joinRequest.GetJoinRequest(r.Context(), &dynastypb.GetJoinRequestRequest{
		RequestId: requestID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildSentJoinRequestHTTP(resp)})
}

// GetReceivedRequest handles GET /api/dynasty/requests/recieved/{joinRequest}
func (h *HTTPDynastyHandler) GetReceivedRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	requestIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/requests/recieved/")
	if requestIDStr == "" {
		writeError(w, http.StatusBadRequest, "request_id is required")
		return
	}

	requestID, err := strconv.ParseUint(requestIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	resp, err := h.joinRequest.GetJoinRequest(r.Context(), &dynastypb.GetJoinRequestRequest{
		RequestId: requestID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildReceivedJoinRequestHTTP(resp)})
}

// DeleteJoinRequest handles DELETE /api/dynasty/requests/sent/{joinRequest}
func (h *HTTPDynastyHandler) DeleteJoinRequest(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	requestIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/requests/sent/")
	if requestIDStr == "" {
		writeError(w, http.StatusBadRequest, "request_id is required")
		return
	}

	requestID, err := strconv.ParseUint(requestIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request_id")
		return
	}

	_, err = h.joinRequest.DeleteJoinRequest(r.Context(), &dynastypb.DeleteJoinRequestRequest{
		RequestId: requestID,
		UserId:    userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetPrizes handles GET /api/dynasty/prizes
func (h *HTTPDynastyHandler) GetPrizes(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	resp, err := h.prize.GetPrizes(r.Context(), &dynastypb.GetPrizesRequest{
		UserId: userCtx.UserID,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: 10,
		},
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": resp})
}

// ClaimPrize handles POST /api/dynasty/prizes/{recievedPrize}
func (h *HTTPDynastyHandler) ClaimPrize(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	prizeIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/prizes/")
	if prizeIDStr == "" {
		writeError(w, http.StatusBadRequest, "prize_id is required")
		return
	}

	prizeID, err := strconv.ParseUint(prizeIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid prize_id")
		return
	}

	_, err = h.prize.ClaimPrize(r.Context(), &dynastypb.ClaimPrizeRequest{
		PrizeId: prizeID,
		UserId:  userCtx.UserID,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetPrize handles GET /api/dynasty/prizes/{recievedPrize}
func (h *HTTPDynastyHandler) GetPrize(w http.ResponseWriter, r *http.Request) {
	if _, err := middleware.GetUserFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	prizeIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/prizes/")
	if prizeIDStr == "" {
		writeError(w, http.StatusBadRequest, "prize_id is required")
		return
	}

	prizeID, err := strconv.ParseUint(prizeIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid prize_id")
		return
	}

	resp, err := h.prize.GetPrize(r.Context(), &dynastypb.GetPrizeRequest{PrizeId: prizeID})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildReceivedPrizeHTTP(resp)})
}

// UpdateChildPermissions handles POST /api/dynasty/children/{user}
func (h *HTTPDynastyHandler) UpdateChildPermissions(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	childUserIDStr := extractIDFromPath(r.URL.Path, "/api/dynasty/children/")
	if childUserIDStr == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	childUserID, err := strconv.ParseUint(childUserIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		Permission string `json:"permission"`
		Status     bool   `json:"status"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	validPermissions := map[string]bool{
		"BFR": true, "SF": true, "W": true, "JU": true, "DM": true,
		"PIUP": true, "PITC": true, "PIC": true, "ESOO": true, "COTB": true,
	}
	if !validPermissions[req.Permission] {
		writeError(w, http.StatusUnprocessableEntity, "invalid permission code")
		return
	}

	if err := h.family.UpdateNamedChildPermission(r.Context(), userCtx.UserID, childUserID, req.Permission, req.Status); err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}})
}

// SearchUsers handles POST /api/dynasty/search
func (h *HTTPDynastyHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := middleware.GetUserFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		SearchTerm string `json:"searchTerm"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.SearchTerm == "" {
		writeError(w, http.StatusUnprocessableEntity, "searchTerm is required")
		return
	}

	resp, err := h.joinRequest.SearchUsers(r.Context(), &dynastypb.SearchUsersRequest{
		SearchTerm: req.SearchTerm,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": buildUserSearchHTTPResponse(resp)})
}

// GetDefaultPermissions handles POST /api/dynasty/add/member/get/permissions
func (h *HTTPDynastyHandler) GetDefaultPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, err := middleware.GetUserFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Relationship string `json:"relationship"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Relationship != "offspring" {
		writeError(w, http.StatusUnprocessableEntity, "relationship must be 'offspring'")
		return
	}

	resp, err := h.joinRequest.GetDefaultPermissions(r.Context(), &dynastypb.GetDefaultPermissionsRequest{
		Relationship: req.Relationship,
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": resp.Permissions})
}
