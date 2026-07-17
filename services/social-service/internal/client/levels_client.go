package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/levels"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LevelsClient wraps gRPC clients for Levels Service
type LevelsClient interface {
	// RecordFollower asks levels-service to update the user's followers_count
	// log and recalculate their score (Laravel UserObserver::followed).
	RecordFollower(ctx context.Context, userID uint64) error
	Close() error
}

type levelsClient struct {
	activityClient pb.ActivityServiceClient
	conn           *grpc.ClientConn
}

// NewLevelsClient creates a new Levels Service client
func NewLevelsClient(address string) (LevelsClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to levels service at %s: %w", address, err)
	}

	return &levelsClient{
		activityClient: pb.NewActivityServiceClient(conn),
		conn:           conn,
	}, nil
}

// Close closes the gRPC connection
func (c *levelsClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RecordFollower records a follower change for the given user
func (c *levelsClient) RecordFollower(ctx context.Context, userID uint64) error {
	resp, err := c.activityClient.RecordFollower(ctx, &pb.RecordFollowerRequest{UserId: userID})
	if err != nil {
		return fmt.Errorf("failed to record follower: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("record follower failed for user %d", userID)
	}
	return nil
}
