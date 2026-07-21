package handler

import (
	"context"
	"fmt"
	"net/url"

	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IsicCodeHandler struct {
	pb.UnimplementedIsicCodeServiceServer
	service IsicCodeServicePort
}

func NewIsicCodeHandler(service IsicCodeServicePort) *IsicCodeHandler {
	return &IsicCodeHandler{service: service}
}

// ListIsicCodes returns paginated ISIC codes.
// Implements GET /api/isic-codes.
func (h *IsicCodeHandler) ListIsicCodes(
	ctx context.Context,
	req *pb.ListIsicCodesRequest,
) (*pb.ListIsicCodesResponse, error) {
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}

	result, err := h.service.Paginate(ctx, page, req.GetSearch())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list isic codes: %v", err)
	}

	items := make([]*pb.IsicCode, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapIsicCode(item))
	}

	basePath := result.Path
	if basePath == "" {
		basePath = models.IsicCodePath
	}

	links := &pb.PaginationLinks{
		First: buildIsicCodePageURL(basePath, 1, result.Search),
		Last:  buildIsicCodePageURL(basePath, result.LastPage, result.Search),
	}
	if result.CurrentPage > 1 {
		links.Prev = buildIsicCodePageURL(basePath, result.CurrentPage-1, result.Search)
	}
	if result.CurrentPage < result.LastPage {
		links.Next = buildIsicCodePageURL(basePath, result.CurrentPage+1, result.Search)
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

	return &pb.ListIsicCodesResponse{
		Data:  items,
		Links: links,
		Meta:  meta,
	}, nil
}

func mapIsicCode(item models.IsicCode) *pb.IsicCode {
	out := &pb.IsicCode{
		Id:       item.ID,
		Name:     item.Name,
		Verified: item.Verified,
	}
	if item.Code != nil {
		out.Code = item.Code
	}
	return out
}

func buildIsicCodePageURL(basePath string, page int, search string) string {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
	if search != "" {
		values.Set("search", search)
	}
	return fmt.Sprintf("%s?%s", basePath, values.Encode())
}
