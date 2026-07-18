// Package service implements notification delivery and business logic.
package service

import (
	"context"

	"metarang/notifications-service/internal/models"
)

// NotificationChannel persists notifications for in-app consumption.
type NotificationChannel interface {
	CreateNotification(ctx context.Context, notification *models.Notification) (string, error)
	ListNotifications(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error)
	MarkAsRead(ctx context.Context, notificationID string, userID uint64) error
	MarkAllAsRead(ctx context.Context, userID uint64) error
}

// SMSChannel abstracts SMS delivery providers.
type SMSChannel interface {
	SendSMS(ctx context.Context, payload models.SMSPayload) (string, error)
	SendOTP(ctx context.Context, payload models.OTPPayload) (string, error)
}

// EmailChannel abstracts email delivery providers.
type EmailChannel interface {
	SendEmail(ctx context.Context, payload models.EmailPayload) (string, error)
}
