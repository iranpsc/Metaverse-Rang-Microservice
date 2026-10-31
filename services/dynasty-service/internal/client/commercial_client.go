package client

import (
	"context"
	"fmt"
	"time"

	pb "metarang/shared/pb/commercial"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

// CommercialClient wraps gRPC client for Commercial Service (wallet operations)
type CommercialClient struct {
	walletClient pb.WalletServiceClient
	rateClient   pb.AssetRateServiceClient
	conn         *grpc.ClientConn
}

// NewCommercialClient creates a new Commercial Service client
func NewCommercialClient(address string) (*CommercialClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to commercial service at %s: %w", address, err)
	}

	return &CommercialClient{
		walletClient: pb.NewWalletServiceClient(conn),
		rateClient:   pb.NewAssetRateServiceClient(conn),
		conn:         conn,
	}, nil
}

// GetPriceByAsset fetches the current rial price per asset unit.
func (c *CommercialClient) GetPriceByAsset(ctx context.Context, asset string) (float64, error) {
	if c.rateClient == nil {
		return 0, fmt.Errorf("commercial rate client not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := c.rateClient.GetAssetRate(ctx, &pb.GetAssetRateRequest{Asset: asset})
	if err != nil {
		return 0, fmt.Errorf("get commercial asset rate: %w", err)
	}
	return resp.GetRialPerUnit(), nil
}

// NewCommercialClientFromGRPC builds a CommercialClient from an existing gRPC stub (used in tests).
func NewCommercialClientFromGRPC(walletClient pb.WalletServiceClient, conn *grpc.ClientConn) *CommercialClient {
	c := &CommercialClient{
		walletClient: walletClient,
		conn:         conn,
	}
	if conn != nil {
		c.rateClient = pb.NewAssetRateServiceClient(conn)
	}
	return c
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
