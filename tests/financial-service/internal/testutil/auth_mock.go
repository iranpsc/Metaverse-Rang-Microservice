package testutil

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metarang/shared/pb/auth"
)

// MockAuthGRPCClient implements pb.AuthServiceClient for middleware tests.
type MockAuthGRPCClient struct {
	ValidateTokenFunc func(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)
}

func (m *MockAuthGRPCClient) Register(context.Context, *pb.RegisterRequest, ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) Redirect(context.Context, *pb.RedirectRequest, ...grpc.CallOption) (*pb.RedirectResponse, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) Callback(context.Context, *pb.CallbackRequest, ...grpc.CallOption) (*pb.CallbackResponse, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) GetMe(context.Context, *pb.GetMeRequest, ...grpc.CallOption) (*pb.UserResponse, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) Logout(context.Context, *pb.LogoutRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	if m.ValidateTokenFunc != nil {
		return m.ValidateTokenFunc(ctx, req)
	}
	return &pb.ValidateTokenResponse{Valid: true, UserId: 42, Email: "user@example.com"}, nil
}

func (m *MockAuthGRPCClient) RequestAccountSecurity(context.Context, *pb.RequestAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) VerifyAccountSecurity(context.Context, *pb.VerifyAccountSecurityRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

func (m *MockAuthGRPCClient) CheckAccountSecurity(context.Context, *pb.CheckAccountSecurityRequest, ...grpc.CallOption) (*pb.CheckAccountSecurityResponse, error) {
	return &pb.CheckAccountSecurityResponse{Unlocked: true}, nil
}
