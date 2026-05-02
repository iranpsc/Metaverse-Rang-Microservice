package handler

import (
	"context"
	"errors"
	"testing"

	"metargb/commercial-service/internal/models"
	pb "metargb/shared/pb/commercial"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubReferralService struct {
	err     error
	lastUID uint64
	lastOID uint64
}

func (s *stubReferralService) ProcessReferralCommission(ctx context.Context, userID uint64, order *models.Order) error {
	s.lastUID = userID
	if order != nil {
		s.lastOID = order.ID
	}
	return s.err
}

func TestReferralHandler_ProcessReferral(t *testing.T) {
	t.Run("maps request and returns empty on success", func(t *testing.T) {
		stub := &stubReferralService{}
		h := NewReferralHandler(stub)
		_, err := h.ProcessReferral(context.Background(), &pb.ProcessReferralRequest{
			BuyerUserId: 7,
			OrderId:     42,
			Asset:       "psc",
			Amount:      10,
		})
		if err != nil {
			t.Fatal(err)
		}
		if stub.lastUID != 7 || stub.lastOID != 42 {
			t.Fatalf("stub got uid=%d oid=%d", stub.lastUID, stub.lastOID)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		h := NewReferralHandler(&stubReferralService{})
		_, err := h.ProcessReferral(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("service error becomes internal", func(t *testing.T) {
		stub := &stubReferralService{err: errors.New("boom")}
		h := NewReferralHandler(stub)
		_, err := h.ProcessReferral(context.Background(), &pb.ProcessReferralRequest{
			BuyerUserId: 1,
			OrderId:     1,
			Asset:       "psc",
			Amount:      1,
		})
		if status.Code(err) != codes.Internal {
			t.Fatalf("got %v", err)
		}
	})
}
