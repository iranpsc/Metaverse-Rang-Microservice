// Package grpcclients provides gRPC client adapters for the financial service.
package grpcclients

import (
	"context"
	"fmt"
	"log"

	commercialpb "metarang/shared/pb/commercial"
	notificationpb "metarang/shared/pb/notifications"
)

// WalletAdapter calls commercial-service WalletService.AddBalance (non-fatal errors are logged, nil returned).
type WalletAdapter struct {
	Client commercialpb.WalletServiceClient
}

func (w *WalletAdapter) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if w == nil || w.Client == nil {
		return nil
	}
	resp, err := w.Client.AddBalance(ctx, &commercialpb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	})
	if err != nil {
		log.Printf("financial-service: AddBalance gRPC error (non-fatal): %v", err)
		return nil
	}
	if resp != nil && !resp.Success {
		log.Printf("financial-service: AddBalance declined (non-fatal): %s", resp.Message)
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

// NotifyAdapter sends an in-app notification after successful payment (non-fatal).
type NotifyAdapter struct {
	Client notificationpb.NotificationServiceClient
}

func (n *NotifyAdapter) NotifyPurchaseSuccess(ctx context.Context, userID, orderID uint64, asset string, amount float64) error {
	if n == nil || n.Client == nil {
		return nil
	}
	_, err := n.Client.SendNotification(ctx, &notificationpb.SendNotificationRequest{
		UserId:  userID,
		Type:    "transaction",
		Title:   "Transaction",
		Message: fmt.Sprintf("Deposit completed for %s", asset),
		Data: map[string]string{
			"order_id": fmt.Sprintf("%d", orderID),
			"asset":    asset,
			"amount":   fmt.Sprintf("%g", amount),
		},
	})
	if err != nil {
		log.Printf("financial-service: SendNotification error (non-fatal): %v", err)
	}
	return nil
}
