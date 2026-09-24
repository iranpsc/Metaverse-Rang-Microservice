package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"metarang/notifications-service/internal/middleware"
	commonpb "metarang/shared/pb/common"
	notificationpb "metarang/shared/pb/notifications"
	"metarang/shared/pkg/sentry"
)

// notificationAPI is the subset of notification RPCs used by the HTTP layer.
type notificationAPI interface {
	GetNotifications(context.Context, *notificationpb.GetNotificationsRequest) (*notificationpb.NotificationsResponse, error)
	MarkAsRead(context.Context, *notificationpb.MarkAsReadRequest) (*commonpb.Empty, error)
	MarkAllAsRead(context.Context, *notificationpb.MarkAllAsReadRequest) (*commonpb.Empty, error)
}

// HTTPNotificationHandler serves Kong-facing REST routes for notifications-service.
type HTTPNotificationHandler struct {
	api notificationAPI
}

// NewHTTPNotificationHandler wraps the gRPC notification handler for local HTTP use.
func NewHTTPNotificationHandler(api notificationAPI) *HTTPNotificationHandler {
	return &HTTPNotificationHandler{api: api}
}

// RegisterHTTPRoutes registers notification REST routes and /health.
func (h *HTTPNotificationHandler) RegisterHTTPRoutes(
	mux *http.ServeMux,
	authMiddleware func(http.Handler) http.Handler,
) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/api/notifications", authMiddleware(http.HandlerFunc(h.GetNotifications)))
	mux.Handle("/api/notifications/read/all", authMiddleware(http.HandlerFunc(h.MarkAllAsRead)))
	mux.Handle("/api/notifications/read/{id}", authMiddleware(http.HandlerFunc(h.MarkAsRead)))
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(
	httpHandler *HTTPNotificationHandler,
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

// GetNotifications handles GET /api/notifications
func (h *HTTPNotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var page, perPage int32 = 1, 100
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.ParseInt(p, 10, 32); err == nil && parsed > 0 {
			page = int32(parsed)
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.ParseInt(pp, 10, 32); err == nil && parsed > 0 {
			perPage = int32(parsed)
		}
	}

	resp, err := h.api.GetNotifications(r.Context(), &notificationpb.GetNotificationsRequest{
		UserId:     userCtx.UserID,
		UnreadOnly: true,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	notifications := make([]map[string]interface{}, 0, len(resp.Notifications))
	for _, notif := range resp.Notifications {
		notifications = append(notifications, transformNotification(notif))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": notifications})
}

// MarkAsRead handles POST /api/notifications/read/{id}
func (h *HTTPNotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "notification ID is required")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if _, err := h.api.MarkAsRead(r.Context(), &notificationpb.MarkAsReadRequest{
		NotificationId: id,
		UserId:         userCtx.UserID,
	}); err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllAsRead handles POST /api/notifications/read/all
func (h *HTTPNotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if _, err := h.api.MarkAllAsRead(r.Context(), &notificationpb.MarkAllAsReadRequest{
		UserId: userCtx.UserID,
	}); err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func transformNotification(notif *notificationpb.Notification) map[string]interface{} {
	var dateStr, timeStr string
	var readAt *string

	if notif.CreatedAt != "" {
		parts := strings.Fields(notif.CreatedAt)
		if len(parts) >= 2 {
			dateStr = parts[0]
			timeStr = parts[1]
		} else if len(parts) == 1 {
			dateStr = parts[0]
			timeStr = "00:00:00"
		}
	}

	if notif.ReadAt != "" && notif.ReadAt != "null" {
		readAt = &notif.ReadAt
	}

	dataObj := make(map[string]interface{})
	for k, v := range notif.Data {
		dataObj[k] = v
	}
	if _, ok := dataObj["related-to"]; !ok {
		dataObj["related-to"] = notif.Type
	}
	if _, ok := dataObj["sender-name"]; !ok {
		dataObj["sender-name"] = "متارنگ"
	}
	if _, ok := dataObj["sender-image"]; !ok {
		dataObj["sender-image"] = ""
	}
	if _, ok := dataObj["message"]; !ok {
		dataObj["message"] = notif.Message
	}

	return map[string]interface{}{
		"id":      notif.Id,
		"data":    dataObj,
		"read_at": readAt,
		"date":    dateStr,
		"time":    timeStr,
	}
}
