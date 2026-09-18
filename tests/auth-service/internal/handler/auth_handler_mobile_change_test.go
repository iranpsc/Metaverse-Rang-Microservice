package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

func TestAuthHandler_SendMobileChangeCode(t *testing.T) {
	ctx := authenticatedContext(1)

	t.Run("successful request", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(ctx context.Context, userID uint64, mobile string) error {
			if userID != 1 || mobile != "09121112233" {
				t.Fatalf("unexpected args userID=%d mobile=%q", userID, mobile)
			}
			return nil
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: "09121112233"})
		if err != nil {
			t.Fatalf("SendMobileChangeCode failed: %v", err)
		}
	})

	t.Run("mobile required", func(t *testing.T) {
		called := false
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			called = true
			return nil
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: ""})
		if err == nil {
			t.Fatal("expected error")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
		if called {
			t.Fatal("service must not be called when mobile is missing")
		}
	})

	t.Run("invalid mobile format", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrInvalidPhoneFormat
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: "08123456789"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
		if status.Code(err) == codes.ResourceExhausted {
			t.Fatal("invalid mobile must not be mapped as rate limited")
		}
	})

	t.Run("mobile already taken", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrPhoneAlreadyTaken
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: "09121112233"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("send rate limited", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrVerificationRequestRateLimited
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: "09121112233"})
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("expected ResourceExhausted, got %v", err)
		}
	})

	t.Run("mobile reset limit exceeded", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrMobileResetLimitExceeded
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "en")
		_, err := h.SendMobileChangeCode(ctx, &pb.SendMobileChangeCodeRequest{Mobile: "09121112233"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
		if status.Code(err) == codes.ResourceExhausted {
			t.Fatal("reset limit must not be mapped as rate limited")
		}
	})
}

func TestAuthHandler_VerifyMobileChange(t *testing.T) {
	ctx := authenticatedContext(1)

	t.Run("successful verification", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyMobileChangeFunc = func(ctx context.Context, userID uint64, code, ip, userAgent string) error {
			if code != "123456" {
				t.Fatalf("unexpected code %q", code)
			}
			return nil
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: "123456"})
		if err != nil {
			t.Fatalf("VerifyMobileChange failed: %v", err)
		}
	})

	t.Run("code required", func(t *testing.T) {
		h := handler.NewAuthHandler(&mockAuthService{}, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("code must be 6 digits", func(t *testing.T) {
		h := handler.NewAuthHandler(&mockAuthService{}, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: "12345"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("invalid code", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyMobileChangeFunc = func(context.Context, uint64, string, string, string) error {
			return service.ErrInvalidOTPCode
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: "000000"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("expired code", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyMobileChangeFunc = func(context.Context, uint64, string, string, string) error {
			return service.ErrOTPExpired
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: "123456"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("attempt rate limited", func(t *testing.T) {
		mockAuthService := &mockAuthService{}
		mockAuthService.verifyMobileChangeFunc = func(context.Context, uint64, string, string, string) error {
			return service.ErrVerificationAttemptRateLimited
		}
		h := handler.NewAuthHandler(mockAuthService, &mockTokenRepository{}, nil, "")
		_, err := h.VerifyMobileChange(ctx, &pb.VerifyMobileChangeRequest{Code: "123456"})
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("expected ResourceExhausted, got %v", err)
		}
	})
}

func TestHTTPAuthHandler_MobileChangeRoutes(t *testing.T) {
	authSvc := &mockAuthService{}
	authServer := handler.NewAuthHandler(authSvc, &mockTokenRepository{}, &mockProfilePhotoService{}, "en")
	clients := handler.NewLocalClients(
		authServer,
		&pb.UnimplementedUserServiceServer{},
		&pb.UnimplementedKYCServiceServer{},
		&pb.UnimplementedCitizenServiceServer{},
		&pb.UnimplementedPersonalInfoServiceServer{},
		&pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{},
		&pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	t.Run("send success", func(t *testing.T) {
		var gotMobile string
		authSvc.sendMobileChangeCodeFunc = func(_ context.Context, userID uint64, mobile string) error {
			if userID != 1 {
				t.Fatalf("userID=%d", userID)
			}
			gotMobile = mobile
			return nil
		}
		body := bytes.NewBufferString(`{"mobile":"09121112233"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/send", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.SendMobileChangeCode(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		if gotMobile != "09121112233" {
			t.Fatalf("mobile=%q", gotMobile)
		}
	})

	t.Run("send validation error is not rate limited", func(t *testing.T) {
		authSvc.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrPhoneAlreadyTaken
		}
		body := bytes.NewBufferString(`{"mobile":"09121112233"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/send", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.SendMobileChangeCode(rr, r)
		if rr.Code == http.StatusTooManyRequests {
			t.Fatal("uniqueness failure must not return 429")
		}
		if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
			t.Fatalf("expected validation status, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("send reset limit is validation error not 429", func(t *testing.T) {
		authSvc.sendMobileChangeCodeFunc = func(context.Context, uint64, string) error {
			return service.ErrMobileResetLimitExceeded
		}
		body := bytes.NewBufferString(`{"mobile":"09121112233"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/send", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.SendMobileChangeCode(rr, r)
		if rr.Code == http.StatusTooManyRequests {
			t.Fatal("reset limit must not return 429")
		}
		if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
			t.Fatalf("expected validation status, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("verify success", func(t *testing.T) {
		authSvc.verifyMobileChangeFunc = func(_ context.Context, userID uint64, code, ip, userAgent string) error {
			if code != "123456" {
				t.Fatalf("code=%q", code)
			}
			return nil
		}
		body := bytes.NewBufferString(`{"code":"123456"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/verify", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.VerifyMobileChange(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("verify attempt limit is 429", func(t *testing.T) {
		authSvc.verifyMobileChangeFunc = func(context.Context, uint64, string, string, string) error {
			return service.ErrVerificationAttemptRateLimited
		}
		body := bytes.NewBufferString(`{"code":"123456"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/verify", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.VerifyMobileChange(rr, r)
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d body=%s", rr.Code, rr.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
	})
}

func TestHTTPAuthHandler_MobileChangeUnauthorized(t *testing.T) {
	httpH := handler.NewHTTPAuthHandler(handler.NewLocalClients(
		handler.NewAuthHandler(&mockAuthService{}, &mockTokenRepository{}, nil, "en"),
		&pb.UnimplementedUserServiceServer{},
		&pb.UnimplementedKYCServiceServer{},
		&pb.UnimplementedCitizenServiceServer{},
		&pb.UnimplementedPersonalInfoServiceServer{},
		&pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{},
		&pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	), nil, "en")

	rr := httptest.NewRecorder()
	httpH.SendMobileChangeCode(rr, httptest.NewRequest(http.MethodPost, "/api/mobile/send", bytes.NewBufferString(`{}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("send code=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	httpH.VerifyMobileChange(rr, httptest.NewRequest(http.MethodPost, "/api/mobile/verify", bytes.NewBufferString(`{}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("verify code=%d", rr.Code)
	}
}
