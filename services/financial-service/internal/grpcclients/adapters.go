// Package grpcclients provides gRPC client adapters for the financial service.
package grpcclients

import (
	"context"
	"fmt"
	"strings"

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
	err := w.addBalance(ctx, userID, asset, amount)
	if err == nil || !isWalletMissing(err) {
		return err
	}

	if _, createErr := w.Client.CreateWallet(ctx, &commercialpb.CreateWalletRequest{UserId: userID}); createErr != nil {
		return fmt.Errorf("%w; create wallet: %v", err, createErr)
	}
	return w.addBalance(ctx, userID, asset, amount)
}

func (w *WalletAdapter) addBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
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

func (w *WalletAdapter) ReverseBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if w == nil || w.Client == nil {
		return fmt.Errorf("wallet client not configured")
	}
	resp, err := w.Client.DeductBalance(ctx, &commercialpb.DeductBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	})
	if err != nil {
		return fmt.Errorf("wallet DeductBalance gRPC failed: %w", err)
	}
	if resp != nil && !resp.Success {
		msg := "unknown error"
		if resp.Message != "" {
			msg = resp.Message
		}
		return fmt.Errorf("wallet DeductBalance rejected: %s", msg)
	}
	return nil
}

func isWalletMissing(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "wallet not found")
}

// ReferralAdapter calls commercial-service ReferralService.ProcessReferral.
// Payment completion treats a returned error as non-fatal.
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
		return fmt.Errorf("ProcessReferral gRPC failed: %w", err)
	}
	return nil
}
