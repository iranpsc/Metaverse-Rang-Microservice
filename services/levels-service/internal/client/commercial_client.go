package client

import (
	"context"
	"fmt"
	"time"

	pb "metargb/shared/pb/commercial"
	"metargb/shared/pkg/auth"

	"google.golang.org/grpc"
)

// CommercialClient defines wallet operations needed by levels service.
type CommercialClient interface {
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
	Close() error
}

type commercialClient struct {
	walletClient pb.WalletServiceClient
	conn         *grpc.ClientConn
}

// NewCommercialClient creates a gRPC client to commercial-service.
func NewCommercialClient(address string) (CommercialClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		address,
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

func (c *commercialClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *commercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	req := &pb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	}

	resp, err := c.walletClient.AddBalance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add %s balance: %w", asset, err)
	}
	if !resp.Success {
		return fmt.Errorf("commercial add balance failed: %s", resp.Message)
	}

	return nil
}
