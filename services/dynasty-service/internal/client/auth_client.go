// Package client provides gRPC clients for external services.
package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/auth"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

// AuthClient wraps gRPC client for Auth Service
type AuthClient struct {
	userClient pb.UserServiceClient
	kycClient  pb.KYCServiceClient
	conn       *grpc.ClientConn
}

// NewAuthClient creates a new Auth Service client
func NewAuthClient(address string) (*AuthClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service at %s: %w", address, err)
	}

	return &AuthClient{
		userClient: pb.NewUserServiceClient(conn),
		kycClient:  pb.NewKYCServiceClient(conn),
		conn:       conn,
	}, nil
}

// Close closes the gRPC connection
func (c *AuthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUser retrieves user information by ID
func (c *AuthClient) GetUser(ctx context.Context, userID uint64) (*pb.User, error) {
	req := &pb.GetUserRequest{
		UserId: userID,
	}

	user, err := c.userClient.GetUser(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserProfile retrieves user profile with images
func (c *AuthClient) GetUserProfile(ctx context.Context, userID uint64) (*pb.GetUserProfileResponse, error) {
	req := &pb.GetUserProfileRequest{
		UserId: userID,
	}

	resp, err := c.userClient.GetUserProfile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return resp, nil
}

// ListUsers searches for users (for dynasty member search)
func (c *AuthClient) ListUsers(ctx context.Context, searchTerm string, page int32) (*pb.ListUsersResponse, error) {
	req := &pb.ListUsersRequest{
		Search: searchTerm,
		Page:   page,
	}

	resp, err := c.userClient.ListUsers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	return resp, nil
}

// GetKYC retrieves KYC information (to check verification status)
func (c *AuthClient) GetKYC(ctx context.Context, userID uint64) (*pb.KYCResponse, error) {
	req := &pb.GetKYCRequest{
		UserId: userID,
	}

	resp, err := c.kycClient.GetKYC(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get KYC: %w", err)
	}

	return resp, nil
}
