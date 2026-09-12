// Package testutil provides test doubles for notifications-service.
package testutil

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/service"
	pb "metarang/shared/pb/auth"
)

type MockNotificationService struct {
	SendNotificationFunc    func(ctx context.Context, input service.SendNotificationInput) (*models.NotificationResult, error)
	GetNotificationsFunc    func(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error)
	GetNotificationByIDFunc func(ctx context.Context, notificationID string, userID uint64) (*models.Notification, error)
	MarkAsReadFunc          func(ctx context.Context, notificationID string, userID uint64) error
	MarkAllAsReadFunc       func(ctx context.Context, userID uint64) error
}

func (m *MockNotificationService) SendNotification(ctx context.Context, input service.SendNotificationInput) (*models.NotificationResult, error) {
	if m.SendNotificationFunc != nil {
		return m.SendNotificationFunc(ctx, input)
	}
	return &models.NotificationResult{ID: 1, Sent: true}, nil
}

func (m *MockNotificationService) GetNotifications(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
	if m.GetNotificationsFunc != nil {
		return m.GetNotificationsFunc(ctx, userID, filter)
	}
	return nil, 0, nil
}

func (m *MockNotificationService) GetNotificationByID(ctx context.Context, notificationID string, userID uint64) (*models.Notification, error) {
	if m.GetNotificationByIDFunc != nil {
		return m.GetNotificationByIDFunc(ctx, notificationID, userID)
	}
	return nil, nil
}

func (m *MockNotificationService) MarkAsRead(ctx context.Context, notificationID string, userID uint64) error {
	if m.MarkAsReadFunc != nil {
		return m.MarkAsReadFunc(ctx, notificationID, userID)
	}
	return nil
}

func (m *MockNotificationService) MarkAllAsRead(ctx context.Context, userID uint64) error {
	if m.MarkAllAsReadFunc != nil {
		return m.MarkAllAsReadFunc(ctx, userID)
	}
	return nil
}

type MockEmailService struct {
	SendEmailFunc func(ctx context.Context, payload models.EmailPayload) (string, error)
}

func (m *MockEmailService) SendEmail(ctx context.Context, payload models.EmailPayload) (string, error) {
	if m.SendEmailFunc != nil {
		return m.SendEmailFunc(ctx, payload)
	}
	return "email-id", nil
}

type MockSMSService struct {
	SendSMSFunc func(ctx context.Context, payload models.SMSPayload) (string, error)
	SendOTPFunc func(ctx context.Context, payload models.OTPPayload) (string, error)
}

func (m *MockSMSService) SendSMS(ctx context.Context, payload models.SMSPayload) (string, error) {
	if m.SendSMSFunc != nil {
		return m.SendSMSFunc(ctx, payload)
	}
	return "sms-id", nil
}

func (m *MockSMSService) SendOTP(ctx context.Context, payload models.OTPPayload) (string, error) {
	if m.SendOTPFunc != nil {
		return m.SendOTPFunc(ctx, payload)
	}
	return "otp-id", nil
}

type MockSMSChannel struct {
	SendSMSFunc func(ctx context.Context, payload models.SMSPayload) (string, error)
	SendOTPFunc func(ctx context.Context, payload models.OTPPayload) (string, error)
}

func (m *MockSMSChannel) SendSMS(ctx context.Context, payload models.SMSPayload) (string, error) {
	if m.SendSMSFunc != nil {
		return m.SendSMSFunc(ctx, payload)
	}
	return "sms-id", nil
}

func (m *MockSMSChannel) SendOTP(ctx context.Context, payload models.OTPPayload) (string, error) {
	if m.SendOTPFunc != nil {
		return m.SendOTPFunc(ctx, payload)
	}
	return "otp-id", nil
}

type MockEmailChannel struct {
	SendEmailFunc func(ctx context.Context, payload models.EmailPayload) (string, error)
}

func (m *MockEmailChannel) SendEmail(ctx context.Context, payload models.EmailPayload) (string, error) {
	if m.SendEmailFunc != nil {
		return m.SendEmailFunc(ctx, payload)
	}
	return "email-id", nil
}

type MockAuthClient struct {
	ValidateTokenFunc func(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)
}

func (m *MockAuthClient) Register(context.Context, *pb.RegisterRequest, ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return nil, nil
}
func (m *MockAuthClient) Redirect(context.Context, *pb.RedirectRequest, ...grpc.CallOption) (*pb.RedirectResponse, error) {
	return nil, nil
}
func (m *MockAuthClient) Callback(context.Context, *pb.CallbackRequest, ...grpc.CallOption) (*pb.CallbackResponse, error) {
	return nil, nil
}
func (m *MockAuthClient) GetMe(context.Context, *pb.GetMeRequest, ...grpc.CallOption) (*pb.UserResponse, error) {
	return nil, nil
}
func (m *MockAuthClient) Logout(context.Context, *pb.LogoutRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *MockAuthClient) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	if m.ValidateTokenFunc != nil {
		return m.ValidateTokenFunc(ctx, req)
	}
	return &pb.ValidateTokenResponse{Valid: true, UserId: 42, Email: "user@example.com"}, nil
}
func (m *MockAuthClient) RequestAccountSecurity(context.Context, *pb.RequestAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *MockAuthClient) VerifyAccountSecurity(context.Context, *pb.VerifyAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *MockAuthClient) CheckAccountSecurity(context.Context, *pb.CheckAccountSecurityRequest, ...grpc.CallOption) (*pb.CheckAccountSecurityResponse, error) {
	return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
}
