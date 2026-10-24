package handler_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/buildings-service/internal/handler"
	pb "metarang/shared/pb/features"
	authpkg "metarang/shared/pkg/auth"
)

type mockEntryService struct{ err error }

func (m mockEntryService) SetConfig(_ context.Context, r *pb.SetBuildingEntryConfigRequest) (*pb.BuildingEntryConfig, error) {
	return &pb.BuildingEntryConfig{FeatureId: r.FeatureId}, m.err
}
func (m mockEntryService) GetConfig(_ context.Context, id uint64) (*pb.BuildingEntryConfig, error) {
	return &pb.BuildingEntryConfig{FeatureId: id}, m.err
}
func (m mockEntryService) CreateCoupon(_ context.Context, r *pb.CreateBuildingEntryCouponRequest) (*pb.BuildingEntryCoupon, error) {
	return &pb.BuildingEntryCoupon{FeatureId: r.FeatureId}, m.err
}
func (m mockEntryService) ListCoupons(_ context.Context, id uint64) ([]*pb.BuildingEntryCoupon, error) {
	return []*pb.BuildingEntryCoupon{{FeatureId: id}}, m.err
}
func (m mockEntryService) Enter(context.Context, *pb.EnterBuildingRequest) (string, error) {
	return "entered", m.err
}
func (m mockEntryService) Exit(context.Context, uint64) (string, error) {
	return "exited", m.err
}

func TestGRPCEntryOperations(t *testing.T) {
	ctx := context.Background()
	h := handler.NewBuildingHandler(&mockBuildingPort{}, nil)
	operations := []struct {
		name string
		call func(uint64) error
	}{
		{"set", func(id uint64) error {
			_, err := h.SetBuildingEntryConfig(ctx, &pb.SetBuildingEntryConfigRequest{FeatureId: id})
			return err
		}},
		{"get", func(id uint64) error {
			_, err := h.GetBuildingEntryConfig(ctx, &pb.GetBuildingEntryConfigRequest{FeatureId: id})
			return err
		}},
		{"create coupon", func(id uint64) error {
			_, err := h.CreateBuildingEntryCoupon(ctx, &pb.CreateBuildingEntryCouponRequest{FeatureId: id})
			return err
		}},
		{"list coupons", func(id uint64) error {
			_, err := h.ListBuildingEntryCoupons(ctx, &pb.ListBuildingEntryCouponsRequest{FeatureId: id})
			return err
		}},
		{"enter", func(id uint64) error {
			_, err := h.EnterBuilding(ctx, &pb.EnterBuildingRequest{FeatureId: id})
			return err
		}},
		{"exit", func(id uint64) error {
			_, err := h.ExitBuilding(ctx, &pb.ExitBuildingRequest{FeatureId: id})
			return err
		}},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			if code := status.Code(op.call(12)); code != codes.Unavailable {
				t.Fatalf("unavailable code=%v", code)
			}
			h.SetEntryService(mockEntryService{})
			if code := status.Code(op.call(0)); code != codes.InvalidArgument {
				t.Fatalf("invalid id code=%v", code)
			}
			if err := op.call(12); err != nil {
				t.Fatal(err)
			}
			h.SetEntryService(mockEntryService{err: errors.New("entry not found")})
			if code := status.Code(op.call(12)); code != codes.NotFound {
				t.Fatalf("not found code=%v", code)
			}
			h.SetEntryService(nil)
		})
	}
}

func TestGRPCEntryErrorMapping(t *testing.T) {
	h := handler.NewBuildingHandler(&mockBuildingPort{}, nil)
	for _, tc := range []struct {
		message string
		code    codes.Code
	}{
		{"unauthorized", codes.PermissionDenied},
		{"already exists", codes.AlreadyExists},
		{"wallet unavailable", codes.Unavailable},
		{"invalid coupon", codes.InvalidArgument},
		{"payment outcome unknown", codes.FailedPrecondition},
		{"insufficient funds", codes.FailedPrecondition},
		{"database exploded", codes.Internal},
	} {
		h.SetEntryService(mockEntryService{err: errors.New(tc.message)})
		_, err := h.GetBuildingEntryConfig(context.Background(), &pb.GetBuildingEntryConfigRequest{FeatureId: 12})
		if code := status.Code(err); code != tc.code {
			t.Fatalf("%q code=%v want=%v", tc.message, code, tc.code)
		}
	}
}

type stubSecurityChecker struct {
	unlocked bool
	err      error
}

func (s stubSecurityChecker) CheckAccountSecurity(context.Context, uint64) (bool, error) {
	return s.unlocked, s.err
}

func TestGRPCEntryAccountSecurity(t *testing.T) {
	h := handler.NewBuildingHandler(&mockBuildingPort{}, nil)
	h.SetEntryService(mockEntryService{})
	h.SetAccountSecurity(stubSecurityChecker{unlocked: false})
	if _, err := h.EnterBuilding(context.Background(), &pb.EnterBuildingRequest{FeatureId: 7}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous code=%v", status.Code(err))
	}
	ctx := context.WithValue(context.Background(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: 8})
	if _, err := h.EnterBuilding(ctx, &pb.EnterBuildingRequest{FeatureId: 7}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("locked code=%v", status.Code(err))
	}
	h.SetAccountSecurity(stubSecurityChecker{err: errors.New("auth down")})
	if _, err := h.ExitBuilding(ctx, &pb.ExitBuildingRequest{FeatureId: 7}); status.Code(err) != codes.Internal {
		t.Fatalf("check error code=%v", status.Code(err))
	}
	h.SetAccountSecurity(stubSecurityChecker{unlocked: true})
	if _, err := h.EnterBuilding(ctx, &pb.EnterBuildingRequest{FeatureId: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ExitBuilding(ctx, &pb.ExitBuildingRequest{FeatureId: 7}); err != nil {
		t.Fatal(err)
	}
	h.SetAccountSecurity(stubSecurityChecker{unlocked: false})
	walletCtx := context.WithValue(context.Background(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: 8, WalletLogin: true})
	if _, err := h.EnterBuilding(walletCtx, &pb.EnterBuildingRequest{FeatureId: 7}); err != nil {
		t.Fatal(err)
	}
}
