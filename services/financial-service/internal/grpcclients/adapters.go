// Package grpcclients provides gRPC client adapters for the financial service.
package grpcclients

import (
	"context"
	"fmt"
	"log"

	commercialpb "metarang/shared/pb/commercial"
)

// WalletAdapter calls commercial-service WalletService.AddBalance.
type WalletAdapter struct {
	Client commercialpb.WalletServiceClient
}

func (w *WalletAdapter) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if w == nil || w.Client == nil {
		return fmt.Errorf("wallet client not configured")
	}
	resp, err := w.Client.AddBalance(ctx, &commercialpb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	})
	if err != nil {
		return fmt.Errorf("wallet AddBalance gRPC failed: %w", err)
	}
	if resp != nil && !resp.Success {
		msg := "unknown error"
		if resp.Message != "" {
			msg = resp.Message
		}
		return fmt.Errorf("wallet AddBalance rejected: %s", msg)
	}
	return nil
}

// ReferralAdapter calls commercial-service ReferralService.ProcessReferral (non-fatal).
type ReferralAdapter struct {
	Client commercialpb.ReferralServiceClient
}

func (r *ReferralAdapter) ProcessReferral(ctx context.Context, buyerUserID, orderID uint64, asset string, amount float64) error {
	if r == nil || r.Client == nil {
		return nil
	}
	_, err := r.Client.ProcessReferral(ctx, &commercialpb.ProcessReferralRequest{
		BuyerUserId: buyerUserID,
		OrderId:     orderID,
		Asset:       asset,
		Amount:      amount,
	})
	if err != nil {
		log.Printf("financial-service: ProcessReferral gRPC error (non-fatal): %v", err)
	}
	return nil
}
