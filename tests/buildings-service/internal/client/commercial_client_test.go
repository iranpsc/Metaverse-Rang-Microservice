package client_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/buildings-service/internal/client"
	pb "metarang/shared/pb/commercial"
)

type walletStub struct {
	pb.WalletServiceClient
	addErr, deductErr, getErr, lockErr, unlockErr error
	addSuccess, deductSuccess                     bool
	wallet                                        *pb.WalletResponse
}

func (s walletStub) AddBalance(_ context.Context, _ *pb.AddBalanceRequest, _ ...grpc.CallOption) (*pb.AddBalanceResponse, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	return &pb.AddBalanceResponse{Success: s.addSuccess, Message: "rejected"}, nil
}
func (s walletStub) DeductBalance(_ context.Context, _ *pb.DeductBalanceRequest, _ ...grpc.CallOption) (*pb.DeductBalanceResponse, error) {
	if s.deductErr != nil {
		return nil, s.deductErr
	}
	return &pb.DeductBalanceResponse{Success: s.deductSuccess, Message: "rejected"}, nil
}
func (s walletStub) GetWallet(_ context.Context, _ *pb.GetWalletRequest, _ ...grpc.CallOption) (*pb.WalletResponse, error) {
	return s.wallet, s.getErr
}
func (s walletStub) LockBalance(_ context.Context, _ *pb.LockBalanceRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.lockErr
}
func (s walletStub) UnlockBalance(_ context.Context, _ *pb.UnlockBalanceRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.unlockErr
}

type transactionStub struct {
	pb.TransactionServiceClient
	err error
}

func (s transactionStub) CreateTransaction(_ context.Context, _ *pb.CreateTransactionRequest, _ ...grpc.CallOption) (*pb.Transaction, error) {
	return &pb.Transaction{Id: "9"}, s.err
}

func TestCommercialClientWalletMutations(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		err      error
		success  bool
		wantType string
	}{
		{"success", nil, true, ""},
		{"insufficient", errors.New("insufficient balance"), false, "insufficient_balance"},
		{"unavailable", status.Error(codes.Unavailable, "down"), false, "service_unavailable"},
		{"invalid", status.Error(codes.InvalidArgument, "bad"), false, "validation_error"},
		{"rejected", nil, false, "validation_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := client.NewCommercialClientFromGRPC(walletStub{addErr: tc.err, deductErr: tc.err, addSuccess: tc.success, deductSuccess: tc.success}, transactionStub{})
			for _, call := range []func() error{
				func() error { return c.AddBalance(ctx, 7, "psc", 2) },
				func() error { return c.DeductBalance(ctx, 7, "psc", 2) },
				func() error { return c.UpdateWallet(ctx, 7, "psc", 2) },
				func() error { return c.UpdateWallet(ctx, 7, "psc", -2) },
			} {
				err := call()
				if tc.wantType == "" && err != nil {
					t.Fatal(err)
				}
				if tc.wantType != "" && (err == nil || !strings.Contains(err.Error(), tc.wantType)) {
					t.Fatalf("error=%v want %s", err, tc.wantType)
				}
			}
			if err := c.UpdateWallet(ctx, 7, "psc", 0); err != nil {
				t.Fatal(err)
			}
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommercialClientReadsAndTransactions(t *testing.T) {
	ctx := context.Background()
	balances := &pb.WalletResponse{Psc: "10", Irr: "20", Red: "3", Blue: "4", Yellow: "5"}
	c := client.NewCommercialClientFromGRPC(walletStub{wallet: balances}, transactionStub{})
	if wallet, err := c.GetWallet(ctx, 7); err != nil || wallet.Psc != "10" {
		t.Fatalf("wallet=%+v err=%v", wallet, err)
	}
	for _, asset := range []string{"psc", "irr", "red", "blue", "yellow"} {
		ok, err := c.CheckBalance(ctx, 7, asset, 1)
		if err != nil || !ok {
			t.Fatalf("%s balance=%v err=%v", asset, ok, err)
		}
	}
	if ok, err := c.CheckBalance(ctx, 7, "psc", 99); err != nil || ok {
		t.Fatalf("insufficient balance=%v err=%v", ok, err)
	}
	if _, err := c.CheckBalance(ctx, 7, "unknown", 1); err == nil {
		t.Fatal("unknown asset accepted")
	}
	if tx, err := c.CreateTransaction(ctx, 7, "psc", 2, "deposit", 1, "entry", 9); err != nil || tx.Id != "9" {
		t.Fatalf("transaction=%+v err=%v", tx, err)
	}
	if err := c.RecordTransaction(ctx, 7, "psc", 2, "deposit", 1, "entry", 9); err != nil {
		t.Fatal(err)
	}
	if err := c.LockBalance(ctx, 7, "psc", 2, "entry"); err != nil {
		t.Fatal(err)
	}
	if err := c.UnlockBalance(ctx, 7, "psc", 2); err != nil {
		t.Fatal(err)
	}
	bad := client.NewCommercialClientFromGRPC(walletStub{getErr: errors.New("down"), lockErr: errors.New("down"), unlockErr: errors.New("down")}, transactionStub{err: errors.New("down")})
	if _, err := bad.GetWallet(ctx, 7); err == nil {
		t.Fatal("missing wallet error")
	}
	if _, err := bad.CheckBalance(ctx, 7, "psc", 1); err == nil {
		t.Fatal("missing balance error")
	}
	if _, err := bad.CreateTransaction(ctx, 7, "psc", 2, "deposit", 1, "entry", 9); err == nil {
		t.Fatal("missing transaction error")
	}
	if err := bad.LockBalance(ctx, 7, "psc", 2, "entry"); err == nil {
		t.Fatal("missing lock error")
	}
	if err := bad.UnlockBalance(ctx, 7, "psc", 2); err == nil {
		t.Fatal("missing unlock error")
	}
}

func TestCommercialClientErrorAndSettings(t *testing.T) {
	cause := errors.New("rpc failed")
	wrapped := &client.CommercialError{Type: "service_unavailable", Message: "down", Err: cause}
	if !errors.Is(wrapped, cause) || !wrapped.PaymentAmbiguous() {
		t.Fatalf("wrapped error=%v", wrapped)
	}
	if (&client.CommercialError{Type: "validation_error", Message: "bad"}).PaymentAmbiguous() {
		t.Fatal("validation error is ambiguous")
	}
	c := client.NewCommercialClientFromGRPC(walletStub{}, transactionStub{})
	c.SetTimeout(time.Second)
	c.SetMaxRetries(2)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
