package main

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

const (
	grpcReadTimeout = 5 * time.Second
	grpcSendTimeout = 15 * time.Second
)

func unaryDeadlineInterceptor(readTimeout, sendTimeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			return handler(ctx, req)
		}
		timeout := readTimeout
		if info != nil && isSendRPC(info.FullMethod) {
			timeout = sendTimeout
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

func isSendRPC(fullMethod string) bool {
	switch fullMethod {
	case "/notifications.NotificationService/SendNotification",
		"/notifications.SMSService/SendSMS",
		"/notifications.SMSService/SendOTP",
		"/notifications.EmailService/SendEmail":
		return true
	default:
		return false
	}
}
