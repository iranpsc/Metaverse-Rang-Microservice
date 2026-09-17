package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/handler"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/service"
	"metarang/notifications-service/tests/internal/testutil"
	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"
	authpkg "metarang/shared/pkg/auth"
)

type notificationAPIAdapter struct {
	h *handler.NotificationHandler
}

func (a *notificationAPIAdapter) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
	return a.h.GetNotifications(ctx, req)
}
func (a *notificationAPIAdapter) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
	return a.h.MarkAsRead(ctx, req)
}
func (a *notificationAPIAdapter) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pbCommon.Empty, error) {
	return a.h.MarkAllAsRead(ctx, req)
}

func testAuthMiddleware(userID uint64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx := &authpkg.UserContext{UserID: userID}
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newHTTPHandler(t *testing.T, svc service.NotificationService, userID uint64) *http.ServeMux {
	t.Helper()
	s := testutil.NewBufGRPCTestServer()
	grpcHandler := handler.RegisterNotificationHandler(s.Server, svc)
	s.Start(t)
	httpH := handler.NewHTTPNotificationHandler(&notificationAPIAdapter{h: grpcHandler})
	mux := http.NewServeMux()
	httpH.RegisterHTTPRoutes(mux, testAuthMiddleware(userID))
	return mux
}

func serveRequest(mux *http.ServeMux, method, path string, userID uint64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if userID != 0 {
		userCtx := &authpkg.UserContext{UserID: userID}
		req = req.WithContext(context.WithValue(req.Context(), authpkg.UserContextKey{}, userCtx))
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestHTTP_Health(t *testing.T) {
	mux := newHTTPHandler(t, &testutil.MockNotificationService{}, 42)
	rr := serveRequest(mux, http.MethodGet, "/health", 42)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetNotifications_Success(t *testing.T) {
	now := time.Now()
	svc := &testutil.MockNotificationService{
		GetNotificationsFunc: func(_ context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
			assert.Equal(t, uint64(42), userID)
			assert.True(t, filter.UnreadOnly)
			return []models.Notification{{
				ID: "n1", Type: "system", Title: "T", Message: "Hello", CreatedAt: now,
				Data: map[string]string{"custom": "val"},
			}}, 1, nil
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications?page=2&per_page=5", 42)
	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string][]map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	require.Len(t, body["data"], 1)
	assert.Equal(t, "n1", body["data"][0]["id"])
}

func TestHTTP_GetNotifications_Unauthorized(t *testing.T) {
	s := testutil.NewBufGRPCTestServer()
	grpcHandler := handler.RegisterNotificationHandler(s.Server, &testutil.MockNotificationService{})
	s.Start(t)
	httpH := handler.NewHTTPNotificationHandler(&notificationAPIAdapter{h: grpcHandler})
	mux := http.NewServeMux()
	passThrough := func(next http.Handler) http.Handler { return next }
	httpH.RegisterHTTPRoutes(mux, passThrough)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications", 0)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_GetNotifications_MethodNotAllowed(t *testing.T) {
	mux := newHTTPHandler(t, &testutil.MockNotificationService{}, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications", 42)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_RemovedRoutes_NotFound(t *testing.T) {
	mux := newHTTPHandler(t, &testutil.MockNotificationService{}, 42)
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/notifications/notif-abc"},
		{http.MethodGet, "/api/notifications/missing"},
		{http.MethodGet, "/api/notifications/550e8400-e29b-41d4-a716-446655440000"},
		{http.MethodPost, "/api/notifications/notif-abc"},
		{http.MethodPost, "/api/notifications/mark-all-read"},
		{http.MethodGet, "/api/notifications/mark-all-read"},
		{http.MethodPost, "/api/notifications/mark-read"},
		{http.MethodGet, "/api/notifications/mark-read"},
		{http.MethodPost, "/api/notifications/"},
		{http.MethodGet, "/api/notifications/read"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := serveRequest(mux, tc.method, tc.path, 42)
			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

func TestHTTP_MarkAsRead_Success(t *testing.T) {
	called := false
	svc := &testutil.MockNotificationService{
		MarkAsReadFunc: func(_ context.Context, id string, userID uint64) error {
			called = true
			assert.Equal(t, "notif-1", id)
			assert.Equal(t, uint64(42), userID)
			return nil
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/notif-1", 42)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, called)
}

func TestHTTP_MarkAsRead_UUID(t *testing.T) {
	const id = "550e8400-e29b-41d4-a716-446655440000"
	called := false
	svc := &testutil.MockNotificationService{
		MarkAsReadFunc: func(_ context.Context, gotID string, userID uint64) error {
			called = true
			assert.Equal(t, id, gotID)
			assert.Equal(t, uint64(42), userID)
			return nil
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/"+id, 42)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, called)
}

func TestHTTP_MarkAllAsRead_Success(t *testing.T) {
	svc := &testutil.MockNotificationService{
		MarkAllAsReadFunc: func(_ context.Context, userID uint64) error {
			assert.Equal(t, uint64(42), userID)
			return nil
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/all", 42)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_MarkAsRead_MissingID(t *testing.T) {
	mux := newHTTPHandler(t, &testutil.MockNotificationService{}, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/", 42)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_GetNotifications_ServiceError(t *testing.T) {
	svc := &testutil.MockNotificationService{
		GetNotificationsFunc: func(context.Context, uint64, models.NotificationFilter) ([]models.Notification, int64, error) {
			return nil, 0, errors.New("service down")
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications", 42)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_MarkAsRead_ServiceError(t *testing.T) {
	svc := &testutil.MockNotificationService{
		MarkAsReadFunc: func(context.Context, string, uint64) error {
			return errors.New("mark failed")
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/notif-1", 42)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_MarkAllAsRead_ServiceError(t *testing.T) {
	svc := &testutil.MockNotificationService{
		MarkAllAsReadFunc: func(context.Context, uint64) error {
			return errors.New("mark all failed")
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/all", 42)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_MarkAllAsRead_MethodNotAllowed(t *testing.T) {
	mux := newHTTPHandler(t, &testutil.MockNotificationService{}, 42)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications/read/all", 42)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_TransformNotificationDefaults(t *testing.T) {
	svc := &testutil.MockNotificationService{
		GetNotificationsFunc: func(context.Context, uint64, models.NotificationFilter) ([]models.Notification, int64, error) {
			return []models.Notification{{
				ID: "n2", Type: "trade", Title: "T", Message: "Body text",
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			}}, 1, nil
		},
	}
	mux := newHTTPHandler(t, svc, 42)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications", 42)
	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string][]map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"][0]["data"].(map[string]interface{})
	assert.Equal(t, "trade", data["related-to"])
	assert.Equal(t, "متارنگ", data["sender-name"])
}
