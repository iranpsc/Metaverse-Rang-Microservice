package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/service"
	"metarang/notifications-service/tests/internal/testutil"
	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"
)

func TestNotificationHandler_SendNotification_ChannelFlags(t *testing.T) {
	h := newNotificationHandler(t, &testutil.MockNotificationService{
		SendNotificationFunc: func(_ context.Context, input service.SendNotificationInput) (*models.NotificationResult, error) {
			assert.True(t, input.SendSMS)
			assert.True(t, input.SendEmail)
			assert.Equal(t, map[string]string{"k": "v"}, input.Data)
			assert.Equal(t, "buy-land-metarang", input.SMSTemplate)
			assert.Equal(t, "p1", input.SMSTokens["token"])
			assert.Equal(t, "buyer", input.SMSTokens["token20"])
			return &models.NotificationResult{ID: 7, Sent: true}, nil
		},
	})

	resp, err := h.SendNotification(context.Background(), &pb.SendNotificationRequest{
		UserId: 1, Type: "system", Title: "T", Message: "M",
		Data: map[string]string{"k": "v"}, SendSms: true, SendEmail: true,
		SmsTemplate: "buy-land-metarang",
		SmsTokens:   map[string]string{"token": "p1", "token20": "buyer"},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(7), resp.Id)
	assert.True(t, resp.Sent)
}

func TestNotificationHandler_GetNotifications_DefaultsAndErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("missing pagination uses defaults", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			GetNotificationsFunc: func(_ context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
				assert.Equal(t, uint64(4), userID)
				assert.Equal(t, int32(1), filter.Page)
				assert.Equal(t, int32(10), filter.PerPage)
				assert.False(t, filter.UnreadOnly)
				return nil, 0, nil
			},
		})
		resp, err := h.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: 4})
		require.NoError(t, err)
		assert.Equal(t, int32(1), resp.Pagination.CurrentPage)
		assert.Equal(t, int32(10), resp.Pagination.PerPage)
	})

	t.Run("zero pagination values keep defaults", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			GetNotificationsFunc: func(_ context.Context, _ uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
				assert.Equal(t, int32(1), filter.Page)
				assert.Equal(t, int32(10), filter.PerPage)
				return nil, 0, nil
			},
		})
		_, err := h.GetNotifications(ctx, &pb.GetNotificationsRequest{
			UserId:     4,
			Pagination: &pbCommon.PaginationRequest{Page: 0, PerPage: 0},
		})
		require.NoError(t, err)
	})

	t.Run("service error", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			GetNotificationsFunc: func(context.Context, uint64, models.NotificationFilter) ([]models.Notification, int64, error) {
				return nil, 0, errors.New("list failed")
			},
		})
		_, err := h.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: 1})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})

	t.Run("unimplemented", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			GetNotificationsFunc: func(context.Context, uint64, models.NotificationFilter) ([]models.Notification, int64, error) {
				return nil, 0, errs.ErrNotImplemented
			},
		})
		_, err := h.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: 1})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unimplemented, st.Code())
	})
}

func TestNotificationHandler_GetNotification_Gaps(t *testing.T) {
	ctx := context.Background()

	t.Run("missing user_id", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{})
		_, err := h.GetNotification(ctx, &pb.GetNotificationRequest{NotificationId: "n1"})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("nil notification without error", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{})
		_, err := h.GetNotification(ctx, &pb.GetNotificationRequest{NotificationId: "n1", UserId: 1})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestNotificationHandler_MarkAsRead_Gaps(t *testing.T) {
	ctx := context.Background()

	t.Run("missing user_id", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{})
		_, err := h.MarkAsRead(ctx, &pb.MarkAsReadRequest{NotificationId: "n1"})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("not found", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			MarkAsReadFunc: func(context.Context, string, uint64) error {
				return errs.ErrNotificationNotFound
			},
		})
		_, err := h.MarkAsRead(ctx, &pb.MarkAsReadRequest{NotificationId: "n1", UserId: 1})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("service error", func(t *testing.T) {
		h := newNotificationHandler(t, &testutil.MockNotificationService{
			MarkAsReadFunc: func(context.Context, string, uint64) error {
				return errors.New("mark failed")
			},
		})
		_, err := h.MarkAsRead(ctx, &pb.MarkAsReadRequest{NotificationId: "n1", UserId: 1})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestNotificationHandler_MarkAllAsRead_ServiceError(t *testing.T) {
	h := newNotificationHandler(t, &testutil.MockNotificationService{
		MarkAllAsReadFunc: func(context.Context, uint64) error {
			return errors.New("mark all failed")
		},
	})
	_, err := h.MarkAllAsRead(context.Background(), &pb.MarkAllAsReadRequest{UserId: 1})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestNotificationHandler_ConvertNotification_ZeroCreatedAt(t *testing.T) {
	h := newNotificationHandler(t, &testutil.MockNotificationService{
		GetNotificationByIDFunc: func(context.Context, string, uint64) (*models.Notification, error) {
			return &models.Notification{ID: "n1", Type: "system", Title: "T", Message: "M"}, nil
		},
	})
	resp, err := h.GetNotification(context.Background(), &pb.GetNotificationRequest{NotificationId: "n1", UserId: 1})
	require.NoError(t, err)
	assert.Empty(t, resp.CreatedAt)
	assert.Empty(t, resp.ReadAt)
}
