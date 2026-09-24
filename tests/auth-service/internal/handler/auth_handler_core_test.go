package handler_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

func TestAuthHandler_RegisterRedirectCallbackGetMeLogoutValidate(t *testing.T) {
	photo := &mockProfilePhotoService{gatewayURL: "https://cdn.example"}
	tokenRepo := &mockTokenRepository{}

	t.Run("register validation", func(t *testing.T) {
		h := handler.NewAuthHandler(&mockAuthService{}, tokenRepo, photo, "en")
		_, err := h.Register(context.Background(), &pb.RegisterRequest{})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("register referral error", func(t *testing.T) {
		m := &mockAuthService{}
		m.registerFunc = func(context.Context, string, string) (string, error) {
			return "", errors.New("referral code does not exist")
		}
		h := handler.NewAuthHandler(m, tokenRepo, photo, "en")
		_, err := h.Register(context.Background(), &pb.RegisterRequest{BackUrl: "https://app"})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("register success and internal", func(t *testing.T) {
		m := &mockAuthService{}
		m.registerFunc = func(context.Context, string, string) (string, error) {
			return "https://oauth", nil
		}
		h := handler.NewAuthHandler(m, tokenRepo, photo, "en")
		resp, err := h.Register(context.Background(), &pb.RegisterRequest{BackUrl: "https://app", Referral: "r"})
		if err != nil || resp.Url != "https://oauth" {
			t.Fatalf("resp=%v err=%v", resp, err)
		}

		m.registerFunc = func(context.Context, string, string) (string, error) {
			return "", errors.New("boom")
		}
		_, err = h.Register(context.Background(), &pb.RegisterRequest{BackUrl: "https://app"})
		st, _ := status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("redirect", func(t *testing.T) {
		m := &mockAuthService{}
		m.redirectFunc = func(context.Context, string, string) (string, string, error) {
			return "https://r", "state", nil
		}
		h := handler.NewAuthHandler(m, tokenRepo, photo, "en")
		resp, err := h.Redirect(context.Background(), &pb.RedirectRequest{RedirectTo: "a", BackUrl: "b"})
		if err != nil || resp.Url != "https://r" {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
		m.redirectFunc = func(context.Context, string, string) (string, string, error) {
			return "", "", errors.New("fail")
		}
		_, err = h.Redirect(context.Background(), &pb.RedirectRequest{})
		st, _ := status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("callback", func(t *testing.T) {
		m := &mockAuthService{}
		m.callbackFunc = func(_ context.Context, _, _, ip string, _ bool) (*service.CallbackResult, error) {
			if ip != "9.9.9.9" {
				t.Fatalf("ip=%s", ip)
			}
			return &service.CallbackResult{Token: "t", ExpiresAt: 55, RedirectURL: "u"}, nil
		}
		h := handler.NewAuthHandler(m, tokenRepo, photo, "en")
		md := metadata.Pairs("x-forwarded-for", "9.9.9.9, 1.1.1.1")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		resp, err := h.Callback(ctx, &pb.CallbackRequest{State: "s", Code: "c"})
		if err != nil || resp.Token != "t" {
			t.Fatalf("resp=%v err=%v", resp, err)
		}

		m.callbackFunc = func(context.Context, string, string, string, bool) (*service.CallbackResult, error) {
			return nil, errors.New("invalid state value")
		}
		_, err = h.Callback(context.Background(), &pb.CallbackRequest{})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}

		m.callbackFunc = func(context.Context, string, string, string, bool) (*service.CallbackResult, error) {
			return nil, errors.New("other")
		}
		_, err = h.Callback(context.Background(), &pb.CallbackRequest{})
		st, _ = status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}

		md = metadata.Pairs("x-real-ip", "8.8.8.8")
		ctx = metadata.NewIncomingContext(context.Background(), md)
		m.callbackFunc = func(_ context.Context, _, _, ip string, _ bool) (*service.CallbackResult, error) {
			if ip != "8.8.8.8" {
				t.Fatalf("ip=%s", ip)
			}
			return &service.CallbackResult{}, nil
		}
		_, err = h.Callback(ctx, &pb.CallbackRequest{})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("get me", func(t *testing.T) {
		m := &mockAuthService{}
		m.getMeFunc = func(context.Context, string) (*service.UserDetails, error) {
			return nil, errors.New("bad token")
		}
		h := handler.NewAuthHandler(m, tokenRepo, photo, "en")
		_, err := h.GetMe(context.Background(), &pb.GetMeRequest{Token: "x"})
		st, _ := status.FromError(err)
		if st.Code() != codes.Unauthenticated {
			t.Fatalf("code=%v", st.Code())
		}

		m.getMeFunc = func(context.Context, string) (*service.UserDetails, error) {
			return &service.UserDetails{
				ID: 1, Name: "n", Code: "c", AutomaticLogout: 0,
				Image: "/p.jpg", WalletLogin: true,
				Level: &service.LevelInfo{ID: 2, Title: "t", Description: "d", Score: 1, Slug: "s"},
			}, nil
		}
		resp, err := h.GetMe(context.Background(), &pb.GetMeRequest{Token: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.AutomaticLogout != 55 || resp.Level == nil || resp.Level.Id != 2 {
			t.Fatalf("%+v", resp)
		}
		if !resp.WalletLogin {
			t.Fatal("expected wallet_login to be preserved on GetMe")
		}
	})

	t.Run("logout", func(t *testing.T) {
		tokenRepo.validateTokenFunc = func(context.Context, string) (*models.User, error) {
			return nil, errors.New("invalid")
		}
		h := handler.NewAuthHandler(&mockAuthService{}, tokenRepo, photo, "en")
		_, err := h.Logout(context.Background(), &pb.LogoutRequest{Token: "x"})
		st, _ := status.FromError(err)
		if st.Code() != codes.Unauthenticated {
			t.Fatalf("code=%v", st.Code())
		}

		tokenRepo.validateTokenFunc = func(context.Context, string) (*models.User, error) {
			return &models.User{ID: 3}, nil
		}
		m := &mockAuthService{}
		m.logoutFunc = func(context.Context, uint64, string, string) error {
			return errors.New("fail")
		}
		h = handler.NewAuthHandler(m, tokenRepo, photo, "en")
		_, err = h.Logout(context.Background(), &pb.LogoutRequest{Token: "x"})
		st, _ = status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}

		m.logoutFunc = func(_ context.Context, id uint64, _, _ string) error {
			if id != 3 {
				t.Fatalf("id=%d", id)
			}
			return nil
		}
		_, err = h.Logout(context.Background(), &pb.LogoutRequest{Token: "x"})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("validate token", func(t *testing.T) {
		tokenRepo := &mockTokenRepository{}
		tokenRepo.validateTokenSessionFunc = func(context.Context, string) (*repository.ValidatedToken, error) {
			return nil, errors.New("bad")
		}
		h := handler.NewAuthHandler(&mockAuthService{}, tokenRepo, photo, "en")
		resp, err := h.ValidateToken(context.Background(), &pb.ValidateTokenRequest{Token: "x"})
		if err != nil || resp.Valid {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
		tokenRepo.validateTokenSessionFunc = func(context.Context, string) (*repository.ValidatedToken, error) {
			return &repository.ValidatedToken{
				User:        &models.User{ID: 5, Email: "e@x.com"},
				WalletLogin: true,
			}, nil
		}
		resp, err = h.ValidateToken(context.Background(), &pb.ValidateTokenRequest{Token: "x"})
		if err != nil || !resp.Valid || resp.UserId != 5 || !resp.WalletLogin {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
	})

	t.Run("register handler wires server", func(t *testing.T) {
		s := grpc.NewServer()
		defer s.Stop()
		h := handler.RegisterAuthHandler(s, &mockAuthService{}, tokenRepo, photo, "en")
		if h == nil {
			t.Fatal("nil handler")
		}
	})
}
