package testutil

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"metarang/dynasty-service/internal/client"
	pb "metarang/shared/pb/commercial"
	"metarang/shared/pkg/auth"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Commercial runs the real dynasty client against a local gRPC dependency.
type Commercial struct {
	pb.UnimplementedAssetRateServiceServer
	pb.UnimplementedWalletServiceServer
	Rate      float64
	Err       error
	RateCalls atomic.Int32
	Credits   chan *pb.AddBalanceRequest
}

func (s *Commercial) GetAssetRate(_ context.Context, req *pb.GetAssetRateRequest) (*pb.GetAssetRateResponse, error) {
	s.RateCalls.Add(1)
	if req.Asset != "psc" {
		return nil, status.Error(codes.InvalidArgument, "expected psc")
	}
	return &pb.GetAssetRateResponse{RialPerUnit: s.Rate}, s.Err
}

func (s *Commercial) AddBalance(_ context.Context, req *pb.AddBalanceRequest) (*pb.AddBalanceResponse, error) {
	s.Credits <- req
	return &pb.AddBalanceResponse{Success: true}, nil
}

func StartCommercial(t *testing.T, rate float64, rateErr error) (*client.CommercialClient, *Commercial) {
	t.Helper()
	t.Setenv("GRPC_TLS_ENABLED", "false")
	t.Setenv("INTERNAL_SERVICE_SECRET", "dynasty-test-token")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.UnaryInterceptor(auth.UnaryServerInterceptor(nil)))
	dependency := &Commercial{Rate: rate, Err: rateErr, Credits: make(chan *pb.AddBalanceRequest, 10)}
	pb.RegisterAssetRateServiceServer(server, dependency)
	pb.RegisterWalletServiceServer(server, dependency)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	c, err := client.NewCommercialClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c, dependency
}
