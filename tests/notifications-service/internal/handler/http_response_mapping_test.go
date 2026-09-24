package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/notifications-service/internal/handler"
	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"
)

type mockNotificationAPI struct {
	GetNotificationsFunc func(context.Context, *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error)
	MarkAsReadFunc       func(context.Context, *pb.MarkAsReadRequest) (*pbCommon.Empty, error)
	MarkAllAsReadFunc    func(context.Context, *pb.MarkAllAsReadRequest) (*pbCommon.Empty, error)
}

func (m *mockNotificationAPI) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
	if m.GetNotificationsFunc != nil {
		return m.GetNotificationsFunc(ctx, req)
	}
	return &pb.NotificationsResponse{}, nil
}

func (m *mockNotificationAPI) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
	if m.MarkAsReadFunc != nil {
		return m.MarkAsReadFunc(ctx, req)
	}
	return &pbCommon.Empty{}, nil
}

func (m *mockNotificationAPI) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pbCommon.Empty, error) {
	if m.MarkAllAsReadFunc != nil {
		return m.MarkAllAsReadFunc(ctx, req)
	}
	return &pbCommon.Empty{}, nil
}

func muxWithAPI(api *mockNotificationAPI, userID uint64) *http.ServeMux {
	httpH := handler.NewHTTPNotificationHandler(api)
	mux := http.NewServeMux()
	httpH.RegisterHTTPRoutes(mux, testAuthMiddleware(userID))
	return mux
}

func TestHTTP_HandlerErrorStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "unauthenticated",
			err:        status.Error(codes.Unauthenticated, "login required"),
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "login required",
		},
		{
			name:       "not found",
			err:        status.Error(codes.NotFound, "missing"),
			wantStatus: http.StatusNotFound,
			wantMsg:    "missing",
		},
		{
			name:       "invalid argument",
			err:        status.Error(codes.InvalidArgument, "bad input"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "bad input",
		},
		{
			name:       "permission denied",
			err:        status.Error(codes.PermissionDenied, "forbidden"),
			wantStatus: http.StatusForbidden,
			wantMsg:    "forbidden",
		},
		{
			name:       "already exists",
			err:        status.Error(codes.AlreadyExists, "duplicate"),
			wantStatus: http.StatusConflict,
			wantMsg:    "duplicate",
		},
		{
			name:       "failed precondition",
			err:        status.Error(codes.FailedPrecondition, "not ready"),
			wantStatus: http.StatusPreconditionFailed,
			wantMsg:    "not ready",
		},
		{
			name:       "unavailable",
			err:        status.Error(codes.Unavailable, "down"),
			wantStatus: http.StatusServiceUnavailable,
			wantMsg:    "service temporarily unavailable: down",
		},
		{
			name:       "internal",
			err:        status.Error(codes.Internal, "boom"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "boom",
		},
		{
			name:       "non status error",
			err:        errors.New("plain"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := muxWithAPI(&mockNotificationAPI{
				GetNotificationsFunc: func(context.Context, *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
					return nil, tt.err
				},
			}, 42)

			rr := serveRequest(mux, http.MethodGet, "/api/notifications", 42)
			assert.Equal(t, tt.wantStatus, rr.Code)

			var body map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
			assert.Equal(t, tt.wantMsg, body["error"])
		})
	}
}

func TestHTTP_MarkAsRead_ErrorStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "not found",
			err:        status.Error(codes.NotFound, "notification not found"),
			wantStatus: http.StatusNotFound,
			wantMsg:    "notification not found",
		},
		{
			name:       "invalid argument",
			err:        status.Error(codes.InvalidArgument, "bad id"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "bad id",
		},
		{
			name:       "permission denied",
			err:        status.Error(codes.PermissionDenied, "forbidden"),
			wantStatus: http.StatusForbidden,
			wantMsg:    "forbidden",
		},
		{
			name:       "plain error",
			err:        errors.New("mark failed"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := muxWithAPI(&mockNotificationAPI{
				MarkAsReadFunc: func(context.Context, *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
					return nil, tt.err
				},
			}, 42)

			rr := serveRequest(mux, http.MethodPost, "/api/notifications/read/notif-1", 42)
			assert.Equal(t, tt.wantStatus, rr.Code)

			var body map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
			assert.Equal(t, tt.wantMsg, body["error"])
		})
	}
}
