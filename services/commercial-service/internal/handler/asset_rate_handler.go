package handler

import (
	"context"
	"math"

	"metarang/commercial-service/internal/repository"
	pb "metarang/shared/pb/commercial"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AssetRateHandler struct {
	pb.UnimplementedAssetRateServiceServer
	rates repository.VariableRepository
}

func NewAssetRateHandler(rates repository.VariableRepository) *AssetRateHandler {
	return &AssetRateHandler{rates: rates}
}

func (h *AssetRateHandler) GetAssetRate(ctx context.Context, req *pb.GetAssetRateRequest) (*pb.GetAssetRateResponse, error) {
	if req == nil || req.Asset == "" {
		return nil, status.Error(codes.InvalidArgument, "asset is required")
	}
	rate, err := h.rates.GetRate(ctx, req.Asset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get asset rate: %v", err)
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil, status.Error(codes.FailedPrecondition, "asset rate must be positive and finite")
	}
	return &pb.GetAssetRateResponse{RialPerUnit: rate}, nil
}
