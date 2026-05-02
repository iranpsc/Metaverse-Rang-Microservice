package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/commercial-service/internal/models"
	"metargb/commercial-service/internal/service"
	pb "metargb/shared/pb/commercial"
)

// ReferralHandler exposes referral commission processing over gRPC (financial-service calls after Parsian verify).
type ReferralHandler struct {
	pb.UnimplementedReferralServiceServer
	referralService service.ReferralService
}

func NewReferralHandler(referralService service.ReferralService) *ReferralHandler {
	return &ReferralHandler{referralService: referralService}
}

func RegisterReferralHandler(grpcServer *grpc.Server, referralService service.ReferralService) {
	h := NewReferralHandler(referralService)
	pb.RegisterReferralServiceServer(grpcServer, h)
}

func (h *ReferralHandler) ProcessReferral(ctx context.Context, req *pb.ProcessReferralRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request required")
	}

	order := &models.Order{
		ID:     req.OrderId,
		UserID: req.BuyerUserId,
		Asset:  req.Asset,
		Amount: req.Amount,
		Status: 0,
	}

	err := h.referralService.ProcessReferralCommission(ctx, req.BuyerUserId, order)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "referral processing failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}
