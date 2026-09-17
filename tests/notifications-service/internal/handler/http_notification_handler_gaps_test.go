package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/handler"
	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"
)

func passThroughAuth(next http.Handler) http.Handler { return next }

func TestHTTP_GetNotifications_PaginationDefaults(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantPage    int32
		wantPerPage int32
	}{
		{name: "omitted", query: "", wantPage: 1, wantPerPage: 100},
		{name: "invalid page", query: "?page=abc&per_page=20", wantPage: 1, wantPerPage: 20},
		{name: "zero page", query: "?page=0&per_page=20", wantPage: 1, wantPerPage: 20},
		{name: "negative page", query: "?page=-2&per_page=20", wantPage: 1, wantPerPage: 20},
		{name: "invalid per_page", query: "?page=3&per_page=nope", wantPage: 3, wantPerPage: 100},
		{name: "zero per_page", query: "?page=3&per_page=0", wantPage: 3, wantPerPage: 100},
		{name: "negative per_page", query: "?page=3&per_page=-5", wantPage: 3, wantPerPage: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := muxWithAPI(&mockNotificationAPI{
				GetNotificationsFunc: func(_ context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
					require.NotNil(t, req.Pagination)
					assert.Equal(t, tt.wantPage, req.Pagination.Page)
					assert.Equal(t, tt.wantPerPage, req.Pagination.PerPage)
					return &pb.NotificationsResponse{}, nil
				},
			}, 42)

			rr := serveRequest(mux, http.MethodGet, "/api/notifications"+tt.query, 42)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestHTTP_UnauthorizedWithoutUser(t *testing.T) {
	httpH := newHTTPMuxPassThrough(&mockNotificationAPI{})

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/notifications"},
		{http.MethodPost, "/api/notifications/read/n1"},
		{http.MethodPost, "/api/notifications/read/all"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := serveRequest(httpH, tc.method, tc.path, 0)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestHTTP_MarkAsRead_ExtractsPathValue(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "simple id", id: "notif-abc"},
		{name: "uuid with hyphens", id: "550e8400-e29b-41d4-a716-446655440000"},
		{name: "id containing all", id: "all-except-literal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID string
			var gotUser uint64
			mux := muxWithAPI(&mockNotificationAPI{
				MarkAsReadFunc: func(_ context.Context, req *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
					gotID = req.NotificationId
					gotUser = req.UserId
					return &pbCommon.Empty{}, nil
				},
			}, 42)

			rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/"+tt.id, 42)
			assert.Equal(t, http.StatusNoContent, rr.Code)
			assert.Equal(t, tt.id, gotID)
			assert.Equal(t, uint64(42), gotUser)
		})
	}
}

func TestHTTP_MarkAllAsRead_LiteralRouteWinsOverWildcard(t *testing.T) {
	markAsReadCalled := false
	markAllCalled := false
	mux := muxWithAPI(&mockNotificationAPI{
		MarkAsReadFunc: func(_ context.Context, req *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
			markAsReadCalled = true
			t.Errorf("MarkAsRead must not run for /read/all; got id=%q", req.NotificationId)
			return &pbCommon.Empty{}, nil
		},
		MarkAllAsReadFunc: func(_ context.Context, req *pb.MarkAllAsReadRequest) (*pbCommon.Empty, error) {
			markAllCalled = true
			assert.Equal(t, uint64(42), req.UserId)
			return &pbCommon.Empty{}, nil
		},
	}, 42)

	rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/all", 42)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, markAllCalled)
	assert.False(t, markAsReadCalled)
}

func TestHTTP_RemainingRoutes_MethodNotAllowed(t *testing.T) {
	mux := muxWithAPI(&mockNotificationAPI{}, 42)
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/notifications"},
		{http.MethodPut, "/api/notifications"},
		{http.MethodDelete, "/api/notifications"},
		{http.MethodPatch, "/api/notifications"},
		{http.MethodGet, "/api/notifications/read/n1"},
		{http.MethodPut, "/api/notifications/read/n1"},
		{http.MethodDelete, "/api/notifications/read/n1"},
		{http.MethodPatch, "/api/notifications/read/n1"},
		{http.MethodGet, "/api/notifications/read/all"},
		{http.MethodPut, "/api/notifications/read/all"},
		{http.MethodDelete, "/api/notifications/read/all"},
		{http.MethodPatch, "/api/notifications/read/all"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := serveRequest(mux, tc.method, tc.path, 42)
			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		})
	}
}

func TestHTTP_MarkAsRead_MethodNotAllowed(t *testing.T) {
	mux := muxWithAPI(&mockNotificationAPI{}, 42)
	rr := serveRequest(mux, http.MethodGet, "/api/notifications/read/n1", 42)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_MarkAsRead_EmptyPathValue(t *testing.T) {
	httpH := handler.NewHTTPNotificationHandler(&mockNotificationAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	httpH.MarkAsRead(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "notification ID is required", body["error"])
}

func TestHTTP_TransformNotification(t *testing.T) {
	tests := []struct {
		name  string
		notif *pb.Notification
		check func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "single created_at token defaults time",
			notif: &pb.Notification{
				Id: "n1", Type: "system", Message: "hello", CreatedAt: "1403/01/15",
			},
			check: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "1403/01/15", body["date"])
				assert.Equal(t, "00:00:00", body["time"])
			},
		},
		{
			name: "empty created_at",
			notif: &pb.Notification{
				Id: "n2", Type: "system", Message: "hello",
			},
			check: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "", body["date"])
				assert.Equal(t, "", body["time"])
				assert.Nil(t, body["read_at"])
			},
		},
		{
			name: "read_at null string",
			notif: &pb.Notification{
				Id: "n3", Type: "system", Message: "hello", ReadAt: "null",
			},
			check: func(t *testing.T, body map[string]interface{}) {
				assert.Nil(t, body["read_at"])
			},
		},
		{
			name: "read_at set",
			notif: &pb.Notification{
				Id: "n4", Type: "system", Message: "hello", ReadAt: "2024-03-10T14:30:00Z",
			},
			check: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "2024-03-10T14:30:00Z", body["read_at"])
			},
		},
		{
			name: "existing data keys are preserved",
			notif: &pb.Notification{
				Id:      "n5",
				Type:    "trade",
				Message: "fallback-message",
				Data: map[string]string{
					"related-to":   "custom-related",
					"sender-name":  "Alice",
					"sender-image": "alice.png",
					"message":      "keep-me",
				},
			},
			check: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "custom-related", data["related-to"])
				assert.Equal(t, "Alice", data["sender-name"])
				assert.Equal(t, "alice.png", data["sender-image"])
				assert.Equal(t, "keep-me", data["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := muxWithAPI(&mockNotificationAPI{
				GetNotificationsFunc: func(context.Context, *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
					return &pb.NotificationsResponse{Notifications: []*pb.Notification{tt.notif}}, nil
				},
			}, 42)

			rr := serveRequest(mux, http.MethodGet, "/api/notifications", 42)
			require.Equal(t, http.StatusOK, rr.Code)

			var wrapped map[string][]map[string]interface{}
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&wrapped))
			require.Len(t, wrapped["data"], 1)
			tt.check(t, wrapped["data"][0])
		})
	}
}

func newHTTPMuxPassThrough(api *mockNotificationAPI) *http.ServeMux {
	mux := http.NewServeMux()
	handler.NewHTTPNotificationHandler(api).RegisterHTTPRoutes(mux, passThroughAuth)
	return mux
}
