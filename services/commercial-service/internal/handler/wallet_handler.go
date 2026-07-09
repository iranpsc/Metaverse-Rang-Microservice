package handler

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/commercial-service/internal/service"
	pb "metargb/shared/pb/commercial"
	"metargb/shared/pkg/auth"
)

type WalletHandler struct {
	pb.UnimplementedWalletServiceServer
	walletService service.WalletService
}

func NewWalletHandler(walletService service.WalletService) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
	}
}

func RegisterWalletHandler(grpcServer *grpc.Server, walletService service.WalletService) {
	handler := NewWalletHandler(walletService)
	pb.RegisterWalletServiceServer(grpcServer, handler)
}

func (h *WalletHandler) GetWallet(ctx context.Context, req *pb.GetWalletRequest) (*pb.WalletResponse, error) {
	if !auth.IsInternalServiceCall(ctx) {
		userCtx, err := auth.GetUserFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}
		if req.UserId != userCtx.UserID {
			return nil, status.Error(codes.PermissionDenied, "access denied")
		}
	}

	wallet, err := h.walletService.GetWallet(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get wallet: %v", err)
	}

	// Parse effect from string to float64
	effect := 0.0
	if effectStr, ok := wallet["effect"]; ok && effectStr != "" {
		if parsedEffect, err := strconv.ParseFloat(effectStr, 64); err == nil {
			effect = parsedEffect
		}
	}

	return &pb.WalletResponse{
		Psc:          wallet["psc"],
		Irr:          wallet["irr"],
		Red:          wallet["red"],
		Blue:         wallet["blue"],
		Yellow:       wallet["yellow"],
		Satisfaction: wallet["satisfaction"],
		Effect:       effect,
	}, nil
}

func (h *WalletHandler) DeductBalance(ctx context.Context, req *pb.DeductBalanceRequest) (*pb.DeductBalanceResponse, error) {
	wallet, err := h.walletService.DeductBalance(ctx, req.UserId, req.Asset, req.Amount)
	if err != nil {
		return &pb.DeductBalanceResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Parse effect from string to float64
	effect := 0.0
	if effectStr, ok := wallet["effect"]; ok && effectStr != "" {
		if parsedEffect, err := strconv.ParseFloat(effectStr, 64); err == nil {
			effect = parsedEffect
		}
	}

	return &pb.DeductBalanceResponse{
		Success: true,
		Message: "Balance deducted successfully",
		Wallet: &pb.WalletResponse{
			Psc:          wallet["psc"],
			Irr:          wallet["irr"],
			Red:          wallet["red"],
			Blue:         wallet["blue"],
			Yellow:       wallet["yellow"],
			Satisfaction: wallet["satisfaction"],
			Effect:       effect,
		},
	}, nil
}

func (h *WalletHandler) AddBalance(ctx context.Context, req *pb.AddBalanceRequest) (*pb.AddBalanceResponse, error) {
	wallet, err := h.walletService.AddBalance(ctx, req.UserId, req.Asset, req.Amount)
	if err != nil {
		return &pb.AddBalanceResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Parse effect from string to float64
	effect := 0.0
	if effectStr, ok := wallet["effect"]; ok && effectStr != "" {
		if parsedEffect, err := strconv.ParseFloat(effectStr, 64); err == nil {
			effect = parsedEffect
		}
	}

	return &pb.AddBalanceResponse{
		Success: true,
		Message: "Balance added successfully",
		Wallet: &pb.WalletResponse{
			Psc:          wallet["psc"],
			Irr:          wallet["irr"],
			Red:          wallet["red"],
			Blue:         wallet["blue"],
			Yellow:       wallet["yellow"],
			Satisfaction: wallet["satisfaction"],
			Effect:       effect,
		},
	}, nil
}

func (h *WalletHandler) LockBalance(ctx context.Context, req *pb.LockBalanceRequest) (*emptypb.Empty, error) {
	err := h.walletService.LockBalance(ctx, req.UserId, req.Asset, req.Amount, req.Reason)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lock balance: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (h *WalletHandler) UnlockBalance(ctx context.Context, req *pb.UnlockBalanceRequest) (*emptypb.Empty, error) {
	err := h.walletService.UnlockBalance(ctx, req.UserId, req.Asset, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unlock balance: %v", err)
	}

	return &emptypb.Empty{}, nil
}
