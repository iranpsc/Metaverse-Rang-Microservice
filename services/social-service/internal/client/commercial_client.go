//go:build !nocommercial
// +build !nocommercial

package client

import (
	"context"
	"fmt"
	"time"

	pb "metargb/shared/pb/commercial"
	"metargb/shared/pkg/auth"

	"google.golang.org/grpc"
)

// CommercialClient wraps gRPC clients for Commercial Service
type CommercialClient interface {
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
	Close() error
}

type commercialClient struct {
	walletClient pb.WalletServiceClient
	conn         *grpc.ClientConn
}

// NewCommercialClient creates a new Commercial Service client
func NewCommercialClient(address string) (CommercialClient, error) {
	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		append(auth.ClientDialOptions(), grpc.WithBlock())...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to commercial service at %s: %w", address, err)
	}

	return &commercialClient{
		walletClient: pb.NewWalletServiceClient(conn),
		conn:         conn,
	}, nil
}

// Close closes the gRPC connection
func (c *commercialClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// AddBalance adds balance to a user's wallet
func (c *commercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	req := &pb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	}

	resp, err := c.walletClient.AddBalance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add balance: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("add balance failed: %s", resp.Message)
	}

	return nil
}
