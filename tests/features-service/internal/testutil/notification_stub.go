package testutil

import (
	"context"
	"sync"

	commonpb "metarang/shared/pb/common"
	pb "metarang/shared/pb/notifications"

	"google.golang.org/grpc"
)

// NotificationCall records a SendNotification RPC.
type NotificationCall struct {
	UserID      uint64
	Type        string
	Title       string
	Message     string
	Data        map[string]string
	SendSMS     bool
	SendEmail   bool
	SMSTemplate string
	SMSTokens   map[string]string
}

// NotificationStub is a configurable NotificationServiceClient for tests.
type NotificationStub struct {
	pb.NotificationServiceClient

	mu sync.Mutex

	Err   error
	Calls []NotificationCall
}

func NewNotificationStub() *NotificationStub {
	return &NotificationStub{}
}

func (s *NotificationStub) SendNotification(_ context.Context, in *pb.SendNotificationRequest, _ ...grpc.CallOption) (*pb.NotificationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := map[string]string{}
	for k, v := range in.GetData() {
		data[k] = v
	}
	tokens := map[string]string{}
	for k, v := range in.GetSmsTokens() {
		tokens[k] = v
	}
	s.Calls = append(s.Calls, NotificationCall{
		UserID:      in.GetUserId(),
		Type:        in.GetType(),
		Title:       in.GetTitle(),
		Message:     in.GetMessage(),
		Data:        data,
		SendSMS:     in.GetSendSms(),
		SendEmail:   in.GetSendEmail(),
		SMSTemplate: in.GetSmsTemplate(),
		SMSTokens:   tokens,
	})
	if s.Err != nil {
		return nil, s.Err
	}
	return &pb.NotificationResponse{Id: 1, Sent: true}, nil
}

func (s *NotificationStub) Types() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.Calls))
	for _, c := range s.Calls {
		out = append(out, c.Type)
	}
	return out
}

func (s *NotificationStub) GetNotifications(context.Context, *pb.GetNotificationsRequest, ...grpc.CallOption) (*pb.NotificationsResponse, error) {
	return &pb.NotificationsResponse{}, nil
}

func (s *NotificationStub) GetNotification(context.Context, *pb.GetNotificationRequest, ...grpc.CallOption) (*pb.Notification, error) {
	return &pb.Notification{}, nil
}

func (s *NotificationStub) MarkAsRead(context.Context, *pb.MarkAsReadRequest, ...grpc.CallOption) (*commonpb.Empty, error) {
	return &commonpb.Empty{}, nil
}

func (s *NotificationStub) MarkAllAsRead(context.Context, *pb.MarkAllAsReadRequest, ...grpc.CallOption) (*commonpb.Empty, error) {
	return &commonpb.Empty{}, nil
}
