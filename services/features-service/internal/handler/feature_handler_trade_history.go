package handler

import (
	"context"
	"errors"
	"fmt"

	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetFeatureTradeHistory handles GET /api/features/{feature}/trade-history
func (h *FeatureHandler) GetFeatureTradeHistory(
	ctx context.Context,
	req *pb.GetFeatureTradeHistoryRequest,
) (*pb.GetFeatureTradeHistoryResponse, error) {
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized: authentication required")
	}

	if req.FeatureId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "feature_id is required")
	}

	if h.tradeHistory == nil {
		return nil, status.Errorf(codes.Internal, "trade history service unavailable")
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}

	result, err := h.tradeHistory.Paginate(ctx, req.FeatureId, user.UserID, page)
	if err != nil {
		if errors.Is(err, models.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		if errors.Is(err, models.ErrNotFeatureOwner) {
			return nil, status.Errorf(codes.PermissionDenied, "only the feature owner can view trade history")
		}
		return nil, status.Errorf(codes.Internal, "failed to get trade history: %v", err)
	}

	items := make([]*pb.FeatureTradeHistoryItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapTradeHistoryItem(item))
	}

	basePath := result.Path
	if basePath == "" {
		basePath = fmt.Sprintf("/api/features/%d/trade-history", req.FeatureId)
	}

	links := &pb.PaginationLinks{
		First: fmt.Sprintf("%s?page=1", basePath),
		Last:  fmt.Sprintf("%s?page=%d", basePath, result.LastPage),
	}
	if result.CurrentPage > 1 {
		links.Prev = fmt.Sprintf("%s?page=%d", basePath, result.CurrentPage-1)
	}
	if result.CurrentPage < result.LastPage {
		links.Next = fmt.Sprintf("%s?page=%d", basePath, result.CurrentPage+1)
	}

	meta := &pb.FeatureTradeHistoryPaginationMeta{
		CurrentPage: int32(result.CurrentPage),
		LastPage:    int32(result.LastPage),
		Path:        basePath,
		PerPage:     int32(result.PerPage),
		Total:       int32(result.Total),
	}
	if result.From != nil {
		from := int32(*result.From)
		meta.From = &from
	}
	if result.To != nil {
		to := int32(*result.To)
		meta.To = &to
	}

	return &pb.GetFeatureTradeHistoryResponse{
		Data:  items,
		Links: links,
		Meta:  meta,
	}, nil
}

func mapTradeHistoryItem(item models.TradeHistoryItem) *pb.FeatureTradeHistoryItem {
	out := &pb.FeatureTradeHistoryItem{
		Type:             item.Type,
		ParticipantLabel: item.ParticipantLabel,
		DateTime: &pb.FeatureTradeHistoryDateTime{
			Date:      item.DateTime.Date,
			MonthName: item.DateTime.MonthName,
			Year:      int32(item.DateTime.Year),
			Time:      item.DateTime.Time,
			Formatted: item.DateTime.Formatted,
		},
		Price: &pb.FeatureTradeHistoryPrice{
			Type: item.Price.Type,
		},
	}
	if item.ID != nil {
		out.Id = item.ID
	}
	if item.ParticipantCode != nil {
		out.ParticipantCode = item.ParticipantCode
	}
	if item.Price.PricePSC != nil {
		out.Price.PricePsc = item.Price.PricePSC
	}
	if item.Price.PriceIRR != nil {
		out.Price.PriceIrr = item.Price.PriceIRR
	}
	if item.Price.Color != nil {
		out.Price.Color = item.Price.Color
	}
	if item.Price.ColorName != nil {
		out.Price.ColorName = item.Price.ColorName
	}
	if item.Price.ColorAmount != nil {
		out.Price.ColorAmount = item.Price.ColorAmount
	}
	return out
}
