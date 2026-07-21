package handler_test

import (
	"context"
	"errors"
	"metarang/auth-service/internal/handler"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

func TestAuthHandler_RequestAccountSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("successful request", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return nil
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err != nil {
			t.Fatalf("RequestAccountSecurity failed: %v", err)
		}
	})

	t.Run("invalid unlock duration", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrInvalidUnlockDuration
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 3,
			Phone:       "09123456789",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("phone required when not verified", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrPhoneRequired
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("invalid phone format", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrInvalidPhoneFormat
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "123456",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("phone already taken", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrPhoneAlreadyTaken
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      1,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.requestAccountSecurityFunc = func(ctx context.Context, userID uint64, minutes int32, phone string) error {
			return service.ErrUserNotFound
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.RequestAccountSecurityRequest{
			UserId:      999,
			TimeMinutes: 15,
			Phone:       "09123456789",
		}

		_, err := h.RequestAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})
}

func TestAuthHandler_VerifyAccountSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("successful verification", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return nil
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "192.168.1.1",
			UserAgent: "Mozilla/5.0",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err != nil {
			t.Fatalf("VerifyAccountSecurity failed: %v", err)
		}
	})

	t.Run("invalid OTP code format", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrInvalidOTPCode
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "abc123",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("invalid OTP code - wrong value", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrInvalidOTPCode
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "000000",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("account security not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrAccountSecurityNotFound
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("account security already unlocked", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrAccountSecurityAlreadyUnlocked
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.FailedPrecondition {
			t.Errorf("Expected FailedPrecondition error code, got %v", st.Code())
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return service.ErrUserNotFound
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    999,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("internal service error", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyAccountSecurityFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			return errors.New("database connection failed")
		}

		tokenRepo := &mockTokenRepository{}
		h := handler.NewAuthHandler(mockAuthService, tokenRepo, nil, "")

		req := &pb.VerifyAccountSecurityRequest{
			UserId:    1,
			Code:      "123456",
			Ip:        "",
			UserAgent: "",
		}

		_, err := h.VerifyAccountSecurity(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Internal {
			t.Errorf("Expected Internal error code, got %v", st.Code())
		}
	})
}
