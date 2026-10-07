package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/service"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
)

// PrizeHandler handles DynastyPrizeService gRPC methods
type PrizeHandler struct {
	dynastypb.UnimplementedDynastyPrizeServiceServer
	prizeService *service.PrizeService
}

// NewPrizeHandler creates a new prize handler
func NewPrizeHandler(prizeService *service.PrizeService) *PrizeHandler {
	return &PrizeHandler{
		prizeService: prizeService,
	}
}

// GetPrizes retrieves all unclaimed prizes for a user
func (h *PrizeHandler) GetPrizes(ctx context.Context, req *dynastypb.GetPrizesRequest) (*dynastypb.PrizesResponse, error) {
	if h.prizeService == nil {
		return nil, status.Errorf(codes.Internal, "prize service not initialized")
	}

	page := int32(1)
	perPage := int32(10)
	if req.Pagination != nil {
		page = req.Pagination.Page
		perPage = req.Pagination.PerPage
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	prizes, total, err := h.prizeService.GetUserReceivedPrizes(ctx, req.UserId, page, perPage)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var protoPrizes []*dynastypb.DynastyPrize
	for _, prize := range prizes {
		if prize.Prize != nil {
			item := buildDynastyPrize(prize.Prize)
			item.Id = prize.ID
			protoPrizes = append(protoPrizes, item)
		}
	}

	return &dynastypb.PrizesResponse{
		Prizes: protoPrizes,
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}, nil
}

// GetPrize retrieves a specific prize by ID
func (h *PrizeHandler) GetPrize(ctx context.Context, req *dynastypb.GetPrizeRequest) (*dynastypb.PrizeResponse, error) {
	if h.prizeService == nil {
		return nil, status.Errorf(codes.Internal, "prize service not initialized")
	}

	receivedPrize, err := h.prizeService.GetReceivedPrize(ctx, req.PrizeId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	if receivedPrize == nil || receivedPrize.Prize == nil {
		return nil, status.Errorf(codes.NotFound, "prize not found")
	}

	prize := buildDynastyPrize(receivedPrize.Prize)
	prize.Id = receivedPrize.ID
	return &dynastypb.PrizeResponse{
		Prize: prize,
	}, nil
}

// ClaimPrize redeems a prize for a user
func (h *PrizeHandler) ClaimPrize(ctx context.Context, req *dynastypb.ClaimPrizeRequest) (*commonpb.Empty, error) {
	if h.prizeService == nil {
		return nil, status.Errorf(codes.Internal, "prize service not initialized")
	}

	err := h.prizeService.ClaimPrize(ctx, req.PrizeId, req.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}
