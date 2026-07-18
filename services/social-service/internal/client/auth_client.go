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

// AuthClient checks auth-service authorization data needed by social-service.
type AuthClient interface {
	CanFollow(ctx context.Context, callerUserID, targetUserID uint64) (bool, error)
	// GetLatestProfilePhotoURL returns the newest profile photo URL for a user.
	// Empty string means the user has no profile photo.
	GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error)
	Close() error
}

type authClient struct {
	userClient         pb.UserServiceClient
	profilePhotoClient pb.ProfilePhotoServiceClient
	conn               *grpc.ClientConn
}

// NewAuthClient creates a new Auth Service client.
func NewAuthClient(address string) (AuthClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service at %s: %w", address, err)
	}

	return &authClient{
		userClient:         pb.NewUserServiceClient(conn),
		profilePhotoClient: pb.NewProfilePhotoServiceClient(conn),
		conn:               conn,
	}, nil
}

func (c *authClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CanFollow mirrors Laravel UserPolicy::follow profile-limitation checks.
// It checks both a target-to-caller limitation and the target's global
// target-to-self limitation.
func (c *authClient) CanFollow(ctx context.Context, callerUserID, targetUserID uint64) (bool, error) {
	allowed, err := c.checkFollowLimitation(ctx, callerUserID, targetUserID, callerUserID)
	if err != nil || !allowed {
		return allowed, err
	}

	return c.checkFollowLimitation(ctx, targetUserID, targetUserID, targetUserID)
}

func (c *authClient) checkFollowLimitation(
	ctx context.Context,
	callerUserID, targetUserID, expectedLimitedUserID uint64,
) (bool, error) {
	resp, err := c.userClient.GetProfileLimitations(ctx, &pb.GetProfileLimitationsRequest{
		CallerUserId: callerUserID,
		TargetUserId: targetUserID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get profile limitations: %w", err)
	}
	if resp == nil || resp.Data == nil || resp.Data.Options == nil {
		return true, nil
	}

	limitation := resp.Data
	if limitation.LimiterUserId != targetUserID ||
		limitation.LimitedUserId != expectedLimitedUserID {
		return true, nil
	}

	follow := limitation.Options.Follow
	return follow == nil || *follow, nil
}

// GetLatestProfilePhotoURL lists profile photos from auth-service and returns
// the newest URL. Photos are ordered oldest-first by auth-service.
func (c *authClient) GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error) {
	resp, err := c.profilePhotoClient.ListProfilePhotos(ctx, &pb.ListProfilePhotosRequest{
		UserId: userID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list profile photos: %w", err)
	}
	if resp == nil || len(resp.Data) == 0 {
		return "", nil
	}
	return resp.Data[len(resp.Data)-1].Url, nil
}
