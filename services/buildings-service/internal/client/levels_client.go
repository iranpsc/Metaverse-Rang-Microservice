package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/levels"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

// LevelsClient wraps the levels-service RPCs used for building entry scopes.
type LevelsClient struct {
	levelClient pb.LevelServiceClient
	conn        *grpc.ClientConn
}

func NewLevelsClient(address string) (*LevelsClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to levels service at %s: %w", address, err)
	}
	return &LevelsClient{
		levelClient: pb.NewLevelServiceClient(conn),
		conn:        conn,
	}, nil
}

// NewLevelsClientFromGRPC builds a LevelsClient from an existing stub (tests).
func NewLevelsClientFromGRPC(levelClient pb.LevelServiceClient) *LevelsClient {
	return &LevelsClient{levelClient: levelClient}
}

func (c *LevelsClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *LevelsClient) GetUserLevel(ctx context.Context, userID uint64) (*pb.UserLevelResponse, error) {
	resp, err := c.levelClient.GetUserLevel(ctx, &pb.GetUserLevelRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get user level: %w", err)
	}
	return resp, nil
}

func (c *LevelsClient) GetAllLevels(ctx context.Context) (*pb.LevelsResponse, error) {
	resp, err := c.levelClient.GetAllLevels(ctx, &pb.GetAllLevelsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get levels: %w", err)
	}
	return resp, nil
}
