package grpcclients_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/financial-service/internal/grpcclients"
	commercialpb "metarang/shared/pb/commercial"
)

type mockWalletClient struct {
	addBalance   func(ctx context.Context, in *commercialpb.AddBalanceRequest, opts ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error)
	createWallet func(ctx context.Context, in *commercialpb.CreateWalletRequest, opts ...grpc.CallOption) (*commercialpb.WalletResponse, error)
}

func (m *mockWalletClient) GetWallet(context.Context, *commercialpb.GetWalletRequest, ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockWalletClient) CreateWallet(ctx context.Context, in *commercialpb.CreateWalletRequest, opts ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
	if m.createWallet != nil {
		return m.createWallet(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}
func (m *mockWalletClient) DeductBalance(context.Context, *commercialpb.DeductBalanceRequest, ...grpc.CallOption) (*commercialpb.DeductBalanceResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockWalletClient) AddBalance(ctx context.Context, in *commercialpb.AddBalanceRequest, opts ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
	if m.addBalance != nil {
		return m.addBalance(ctx, in, opts...)
	}
	return &commercialpb.AddBalanceResponse{Success: true}, nil
}
func (m *mockWalletClient) LockBalance(context.Context, *commercialpb.LockBalanceRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}
func (m *mockWalletClient) UnlockBalance(context.Context, *commercialpb.UnlockBalanceRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}

var _ commercialpb.WalletServiceClient = (*mockWalletClient)(nil)

type mockReferralClient struct {
	processReferral func(ctx context.Context, in *commercialpb.ProcessReferralRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

func (m *mockReferralClient) ProcessReferral(ctx context.Context, in *commercialpb.ProcessReferralRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	if m.processReferral != nil {
		return m.processReferral(ctx, in, opts...)
	}
	return &emptypb.Empty{}, nil
}

var _ commercialpb.ReferralServiceClient = (*mockReferralClient)(nil)

func TestWalletAdapter_NilClient(t *testing.T) {
	var adapter *grpcclients.WalletAdapter
	err := adapter.AddBalance(context.Background(), 1, "psc", 10)
	if err == nil {
		t.Fatal("expected error for nil adapter")
	}

	adapter = &grpcclients.WalletAdapter{}
	err = adapter.AddBalance(context.Background(), 1, "psc", 10)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestWalletAdapter_Success(t *testing.T) {
	adapter := &grpcclients.WalletAdapter{
		Client: &mockWalletClient{
			addBalance: func(_ context.Context, req *commercialpb.AddBalanceRequest, _ ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
				if req.UserId != 5 || req.Asset != "psc" || req.Amount != 100 {
					t.Fatalf("unexpected request: %+v", req)
				}
				return &commercialpb.AddBalanceResponse{Success: true}, nil
			},
		},
	}
	if err := adapter.AddBalance(context.Background(), 5, "psc", 100); err != nil {
		t.Fatal(err)
	}
}

func TestWalletAdapter_Rejected(t *testing.T) {
	adapter := &grpcclients.WalletAdapter{
		Client: &mockWalletClient{
			addBalance: func(context.Context, *commercialpb.AddBalanceRequest, ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
				return &commercialpb.AddBalanceResponse{Success: false, Message: "insufficient"}, nil
			},
		},
	}
	err := adapter.AddBalance(context.Background(), 1, "psc", 10)
	if err == nil {
		t.Fatal("expected rejection error")
	}
}

func TestWalletAdapter_CreatesWalletWhenMissing(t *testing.T) {
	adds := 0
	created := false
	adapter := &grpcclients.WalletAdapter{
		Client: &mockWalletClient{
			addBalance: func(context.Context, *commercialpb.AddBalanceRequest, ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
				adds++
				if adds == 1 {
					return &commercialpb.AddBalanceResponse{Success: false, Message: "failed to add balance: wallet not found"}, nil
				}
				return &commercialpb.AddBalanceResponse{Success: true}, nil
			},
			createWallet: func(_ context.Context, req *commercialpb.CreateWalletRequest, _ ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
				created = true
				if req.UserId != 4 {
					t.Fatalf("user=%d", req.UserId)
				}
				return &commercialpb.WalletResponse{}, nil
			},
		},
	}
	if err := adapter.AddBalance(context.Background(), 4, "psc", 10); err != nil {
		t.Fatal(err)
	}
	if !created || adds != 2 {
		t.Fatalf("created=%v adds=%d", created, adds)
	}
}

func TestWalletAdapter_GRPCError(t *testing.T) {
	adapter := &grpcclients.WalletAdapter{
		Client: &mockWalletClient{
			addBalance: func(context.Context, *commercialpb.AddBalanceRequest, ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
				return nil, status.Error(codes.Unavailable, "down")
			},
		},
	}
	err := adapter.AddBalance(context.Background(), 1, "psc", 10)
	if err == nil {
		t.Fatal("expected gRPC error")
	}
}

func TestReferralAdapter_NilClient(t *testing.T) {
	var adapter *grpcclients.ReferralAdapter
	if err := adapter.ProcessReferral(context.Background(), 1, 2, "psc", 10); err != nil {
		t.Fatal(err)
	}

	adapter = &grpcclients.ReferralAdapter{}
	if err := adapter.ProcessReferral(context.Background(), 1, 2, "psc", 10); err != nil {
		t.Fatal(err)
	}
}

func TestReferralAdapter_Success(t *testing.T) {
	called := false
	adapter := &grpcclients.ReferralAdapter{
		Client: &mockReferralClient{
			processReferral: func(_ context.Context, req *commercialpb.ProcessReferralRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
				called = true
				if req.BuyerUserId != 9 || req.OrderId != 77 {
					t.Fatalf("unexpected request: %+v", req)
				}
				return &emptypb.Empty{}, nil
			},
		},
	}
	if err := adapter.ProcessReferral(context.Background(), 9, 77, "irr", 50); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected ProcessReferral to be called")
	}
}

func TestReferralAdapter_NonFatalError(t *testing.T) {
	adapter := &grpcclients.ReferralAdapter{
		Client: &mockReferralClient{
			processReferral: func(context.Context, *commercialpb.ProcessReferralRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
				return nil, errors.New("transient")
			},
		},
	}
	if err := adapter.ProcessReferral(context.Background(), 1, 2, "psc", 10); err != nil {
		t.Fatal("referral errors should be non-fatal")
	}
}
