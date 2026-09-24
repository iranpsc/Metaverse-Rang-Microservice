package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/repository"
)

// SendNotificationInput represents the information required to dispatch a notification.
type SendNotificationInput struct {
	UserID    uint64
	Type      string
	Title     string
	Message   string
	Data      map[string]string
	SendSMS   bool
	SendEmail bool

	SMSTemplate string
	SMSTokens   map[string]string
	HTMLBody    string

	SMSPayload   *models.SMSPayload
	EmailPayload *models.EmailPayload
}

// NotificationService encapsulates business logic for notifications.
type NotificationService interface {
	SendNotification(ctx context.Context, input SendNotificationInput) (*models.NotificationResult, error)
	GetNotifications(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error)
	GetNotificationByID(ctx context.Context, notificationID string, userID uint64) (*models.Notification, error)
	MarkAsRead(ctx context.Context, notificationID string, userID uint64) error
	MarkAllAsRead(ctx context.Context, userID uint64) error
}

type notificationService struct {
	repo         *repository.NotificationRepository
	smsChannel   SMSChannel
	emailChannel EmailChannel
}

// NewNotificationService creates a notification service implementation.
func NewNotificationService(
	repo *repository.NotificationRepository,
	smsChannel SMSChannel,
	emailChannel EmailChannel,
) NotificationService {
	return &notificationService{
		repo:         repo,
		smsChannel:   smsChannel,
		emailChannel: emailChannel,
	}
}

func (s *notificationService) SendNotification(ctx context.Context, input SendNotificationInput) (*models.NotificationResult, error) {
	notification := &models.Notification{
		UserID:    input.UserID,
		Type:      input.Type,
		Title:     input.Title,
		Message:   input.Message,
		Data:      input.Data,
		CreatedAt: time.Now(),
	}

	id, err := s.repo.CreateNotification(ctx, notification)
	if err != nil {
		return nil, err
	}

	s.resolveChannelPayloads(ctx, &input)

	smsErr := s.deliverSMS(ctx, input)
	emailErr := s.deliverEmail(ctx, input)
	if err := channelDeliveryError(input.SendSMS, input.SendEmail, smsErr, emailErr); err != nil {
		return &models.NotificationResult{ID: id, Sent: false}, err
	}

	return &models.NotificationResult{
		ID:   id,
		Sent: true,
	}, nil
}

// channelDeliveryError keeps SMS and email independent.
// A Kavenegar failure must not skip or fail the whole notification when email succeeded.
func channelDeliveryError(sendSMS, sendEmail bool, smsErr, emailErr error) error {
	if sendEmail && emailErr == nil && sendSMS && smsErr != nil {
		return nil
	}
	if smsErr != nil && emailErr != nil {
		return errors.Join(smsErr, emailErr)
	}
	if smsErr != nil {
		return smsErr
	}
	return emailErr
}

func (s *notificationService) resolveChannelPayloads(ctx context.Context, input *SendNotificationInput) {
	needSMS := input.SendSMS && input.SMSPayload == nil
	needEmail := input.SendEmail && input.EmailPayload == nil
	if !needSMS && !needEmail {
		return
	}

	contact, err := s.repo.GetUserContact(ctx, input.UserID)
	if err != nil || contact == nil {
		return
	}

	if needSMS {
		if phone := strings.TrimSpace(contact.Phone); phone != "" {
			input.SMSPayload = &models.SMSPayload{
				Phone:    phone,
				Message:  input.Message,
				Template: input.SMSTemplate,
				Tokens:   input.SMSTokens,
			}
		}
	}
	if needEmail {
		if email := strings.TrimSpace(contact.Email); email != "" {
			htmlBody := input.HTMLBody
			if htmlBody == "" {
				emailData := enrichEmailData(input.Data, contact)
				if rendered, err := RenderNotificationEmail(input.Type, input.Title, emailData, contact.Name); err == nil && rendered != "" {
					htmlBody = rendered
				} else if err != nil {
					// Keep delivery working even if a template fails to render.
					fmt.Printf("Warning: email template render failed type=%s: %v\n", input.Type, err)
				}
			}
			if htmlBody == "" && input.Message != "" {
				htmlBody = fmt.Sprintf(`<div dir="rtl" style="font-family:Tahoma,sans-serif">%s</div>`, html.EscapeString(input.Message))
			}
			input.EmailPayload = &models.EmailPayload{
				To:       email,
				Subject:  input.Title,
				Body:     input.Message,
				HTMLBody: htmlBody,
			}
		}
	}
}

func (s *notificationService) deliverSMS(ctx context.Context, input SendNotificationInput) error {
	if !input.SendSMS || s.smsChannel == nil || input.SMSPayload == nil {
		return nil
	}
	_, err := s.smsChannel.SendSMS(ctx, *input.SMSPayload)
	return ignoreUnimplemented(err)
}

func (s *notificationService) deliverEmail(ctx context.Context, input SendNotificationInput) error {
	if !input.SendEmail || s.emailChannel == nil || input.EmailPayload == nil {
		return nil
	}
	_, err := s.emailChannel.SendEmail(ctx, *input.EmailPayload)
	return ignoreUnimplemented(err)
}

func ignoreUnimplemented(err error) error {
	if err == nil || errors.Is(err, errs.ErrNotImplemented) {
		return nil
	}
	return err
}

// enrichEmailData fills missing template fields from the user contact (e.g. citizen code).
func enrichEmailData(data map[string]string, contact *models.UserContact) map[string]string {
	out := make(map[string]string, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	if contact == nil {
		return out
	}
	if strings.TrimSpace(out["user_code"]) == "" && strings.TrimSpace(out["UserCode"]) == "" {
		if code := strings.TrimSpace(contact.Code); code != "" {
			out["user_code"] = code
		}
	}
	return out
}

func (s *notificationService) GetNotifications(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
	result, total, err := s.repo.ListNotifications(ctx, userID, filter)

	return result, total, err
}

func (s *notificationService) MarkAsRead(ctx context.Context, notificationID string, userID uint64) error {
	return s.repo.MarkAsRead(ctx, notificationID, userID)
}

func (s *notificationService) MarkAllAsRead(ctx context.Context, userID uint64) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

func (s *notificationService) GetNotificationByID(ctx context.Context, notificationID string, userID uint64) (*models.Notification, error) {
	notification, err := s.repo.GetNotificationByID(ctx, notificationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	if notification == nil {
		return nil, errs.ErrNotificationNotFound
	}
	return notification, nil
}
