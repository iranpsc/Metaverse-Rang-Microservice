package middleware_test

import (
	"context"
	"errors"
	"testing"

	"metarang/buildings-service/internal/middleware"
	pb "metarang/shared/pb/auth"
)

func TestAccountSecurityChecker(t *testing.T) {
	if checker := middleware.NewAccountSecurityChecker(nil); checker != nil {
		t.Fatal("nil auth client returned checker")
	}
	client := &mockAuthServiceClient{}
	client.CheckAccountSecurityFunc = func(_ context.Context, req *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		if req.UserId != 7 {
			t.Fatalf("user id=%d", req.UserId)
		}
		return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
	}
	checker := middleware.NewAccountSecurityChecker(client)
	if unlocked, err := checker.CheckAccountSecurity(context.Background(), 7); err != nil || !unlocked {
		t.Fatalf("unlocked=%v err=%v", unlocked, err)
	}
	client.CheckAccountSecurityFunc = func(context.Context, *pb.CheckAccountSecurityRequest) (*pb.CheckAccountSecurityResponse, error) {
		return nil, errors.New("auth unavailable")
	}
	if _, err := checker.CheckAccountSecurity(context.Background(), 7); err == nil {
		t.Fatal("auth error ignored")
	}
}
