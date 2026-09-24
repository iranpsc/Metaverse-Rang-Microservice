package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/notifications"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

// NotificationClient wraps gRPC client for Notifications Service
type NotificationClient struct {
	notificationClient pb.NotificationServiceClient
	conn               *grpc.ClientConn
}

// NewNotificationClient creates a new Notifications Service client
func NewNotificationClient(address string) (*NotificationClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notifications service at %s: %w", address, err)
	}

	return &NotificationClient{
		notificationClient: pb.NewNotificationServiceClient(conn),
		conn:               conn,
	}, nil
}

// NewNotificationClientFromGRPC builds a NotificationClient from an existing gRPC stub (used in tests).
func NewNotificationClientFromGRPC(notificationClient pb.NotificationServiceClient, conn *grpc.ClientConn) *NotificationClient {
	return &NotificationClient{
		notificationClient: notificationClient,
		conn:               conn,
	}
}

// Close closes the gRPC connection
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendNotification sends a notification to a user
func (c *NotificationClient) SendNotification(ctx context.Context, userID uint64, notificationType, title, message string, data map[string]string, sendSMS, sendEmail bool) error {
	req := &pb.SendNotificationRequest{
		UserId:    userID,
		Type:      notificationType,
		Title:     title,
		Message:   message,
		Data:      data,
		SendSms:   sendSMS,
		SendEmail: sendEmail,
	}

	resp, err := c.notificationClient.SendNotification(ctx, req, grpc.WaitForReady(true))
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	if !resp.Sent {
		return fmt.Errorf("notification was not sent")
	}

	return nil
}
