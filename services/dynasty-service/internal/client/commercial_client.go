package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	pb "metargb/shared/pb/commercial"
	"metargb/shared/pkg/auth"
)

// CommercialClient wraps gRPC client for Commercial Service (wallet operations)
type CommercialClient struct {
	walletClient pb.WalletServiceClient
	conn         *grpc.ClientConn
}

// NewCommercialClient creates a new Commercial Service client
func NewCommercialClient(address string) (*CommercialClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		append(auth.ClientDialOptions(), grpc.WithBlock())...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to commercial service at %s: %w", address, err)
	}

	return &CommercialClient{
		walletClient: pb.NewWalletServiceClient(conn),
		conn:         conn,
	}, nil
}

// Close closes the gRPC connection
func (c *CommercialClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IncrementWalletPSC adds PSC to user's wallet
func (c *CommercialClient) IncrementWalletPSC(ctx context.Context, userID uint64, amount float64) error {
	req := &pb.AddBalanceRequest{
		UserId: userID,
		Asset:  "psc",
		Amount: amount,
	}

	resp, err := c.walletClient.AddBalance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add PSC balance: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("add balance failed: %s", resp.Message)
	}

	return nil
}

// IncrementSatisfaction adds satisfaction to user's wallet
func (c *CommercialClient) IncrementSatisfaction(ctx context.Context, userID uint64, amount float64) error {
	req := &pb.AddBalanceRequest{
		UserId: userID,
		Asset:  "satisfaction",
		Amount: amount,
	}

	resp, err := c.walletClient.AddBalance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add satisfaction: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("add satisfaction failed: %s", resp.Message)
	}

	return nil
}

// GetWallet retrieves user's wallet
func (c *CommercialClient) GetWallet(ctx context.Context, userID uint64) (*pb.WalletResponse, error) {
	req := &pb.GetWalletRequest{
		UserId: userID,
	}

	resp, err := c.walletClient.GetWallet(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	return resp, nil
}

