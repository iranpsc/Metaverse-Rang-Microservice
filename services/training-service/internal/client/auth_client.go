// Package client provides gRPC clients for external services used by the training service.
package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/auth"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

// AuthClient wraps gRPC client for Auth Service user operations
type AuthClient struct {
	userClient pb.UserServiceClient
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

// GetUserByCode retrieves user information by code
// Note: This uses ListUsers with search as auth-service doesn't have GetUserByCode
// TODO: Add GetUserByCode to auth-service proto for better performance
func (c *AuthClient) GetUserByCode(ctx context.Context, code string) (*pb.User, error) {
	// Use ListUsers with search parameter to find user by code
	req := &pb.ListUsersRequest{
		Search: code,
		Page:   1,
	}

	resp, err := c.userClient.ListUsers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search user by code: %w", err)
	}

	// Find exact match by code (case-insensitive)
	for _, item := range resp.Data {
		if item.Code == code {
			// Get full user details by ID
			return c.GetUser(ctx, item.Id)
		}
	}

	return nil, fmt.Errorf("user not found with code: %s", code)
}

// GetUserProfile retrieves user profile with profile images
// This can be used to get the latest profile photo
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
