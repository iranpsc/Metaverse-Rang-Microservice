// Package handler implements gRPC handlers for notifications.
package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/service"
	"metarang/shared/pkg/helpers"
)

// NotificationHandler implements the gRPC NotificationService.
type NotificationHandler struct {
	pb.UnimplementedNotificationServiceServer
	service service.NotificationService
}

// RegisterNotificationHandler registers the notification handler with the gRPC server.
func RegisterNotificationHandler(grpcServer *grpc.Server, svc service.NotificationService) {
	handler := &NotificationHandler{service: svc}
	pb.RegisterNotificationServiceServer(grpcServer, handler)
}

func (h *NotificationHandler) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.NotificationResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if req.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	input := service.SendNotificationInput{
		UserID:    req.UserId,
		Type:      req.Type,
		Title:     req.Title,
		Message:   req.Message,
		Data:      req.Data,
		SendSMS:   req.SendSms,
		SendEmail: req.SendEmail,
	}

	result, err := h.service.SendNotification(ctx, input)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &pb.NotificationResponse{
		Id:   result.ID,
		Sent: result.Sent,
	}, nil
}

func (h *NotificationHandler) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	filter := models.NotificationFilter{
		Page:       1,
		PerPage:    10,
		UnreadOnly: req.UnreadOnly, // Default to false if not specified, but API docs say GET /api/notifications returns unread only
	}

	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			filter.Page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			filter.PerPage = req.Pagination.PerPage
		}
	}

	notifications, total, err := h.service.GetNotifications(ctx, req.UserId, filter)
	if err != nil {
		return nil, handleServiceError(err)
	}

	response := &pb.NotificationsResponse{
		Notifications: make([]*pb.Notification, 0, len(notifications)),
		Pagination: &pbCommon.PaginationMeta{
			CurrentPage: filter.Page,
			PerPage:     filter.PerPage,
			Total:       int32(total),
		},
	}

	for _, notification := range notifications {
		response.Notifications = append(response.Notifications, convertNotification(notification))
	}

	return response, nil
}

func (h *NotificationHandler) GetNotification(ctx context.Context, req *pb.GetNotificationRequest) (*pb.Notification, error) {
	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	notification, err := h.service.GetNotificationByID(ctx, req.NotificationId, req.UserId)
	if err != nil {
		return nil, handleServiceError(err)
	}
	if notification == nil {
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	return convertNotification(*notification), nil
}

func (h *NotificationHandler) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pbCommon.Empty, error) {
	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.service.MarkAsRead(ctx, req.NotificationId, req.UserId); err != nil {
		return nil, handleServiceError(err)
	}

	return &pbCommon.Empty{}, nil
}

func (h *NotificationHandler) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pbCommon.Empty, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.service.MarkAllAsRead(ctx, req.UserId); err != nil {
		return nil, handleServiceError(err)
	}

	return &pbCommon.Empty{}, nil
}

func convertNotification(notification models.Notification) *pb.Notification {
	protoNotification := &pb.Notification{
		Id:      notification.ID,
		Type:    notification.Type,
		Title:   notification.Title,
		Message: notification.Message,
		Data:    notification.Data,
	}

	// Format created_at as Jalali date and time (Y/m/d H:m:s format)
	// This will be parsed by grpc-gateway to extract separate date and time fields
	if !notification.CreatedAt.IsZero() {
		// Format as "Y/m/d H:m:s" for parsing in grpc-gateway
		dateStr := helpers.FormatJalaliDate(notification.CreatedAt)
		timeStr := helpers.FormatJalaliTime(notification.CreatedAt)
		protoNotification.CreatedAt = fmt.Sprintf("%s %s", dateStr, timeStr)
	}

	// Format read_at - use null string if unread, otherwise RFC3339 timestamp
	if notification.ReadAt != nil {
		// For API compatibility, return RFC3339 format for read_at
		protoNotification.ReadAt = notification.ReadAt.Format(time.RFC3339)
	} else {
		// Unread notifications have empty read_at (proto will serialize as empty string)
		protoNotification.ReadAt = ""
	}

	return protoNotification
}

func handleServiceError(err error) error {
	if errors.Is(err, errs.ErrNotImplemented) {
		return status.Error(codes.Unimplemented, err.Error())
	}
	if errors.Is(err, errs.ErrNotificationNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Errorf(codes.Internal, "service error: %v", err)
}
