package handler

import (
	"context"
	"fmt"

	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CitizenBuildingsHandler implements features.CitizenBuildingsServiceServer.
type CitizenBuildingsHandler struct {
	pb.UnimplementedCitizenBuildingsServiceServer
	service CitizenBuildingsServicePort
}

func NewCitizenBuildingsHandler(service CitizenBuildingsServicePort) *CitizenBuildingsHandler {
	return &CitizenBuildingsHandler{service: service}
}

func (h *CitizenBuildingsHandler) GetCitizenBuildingSummary(
	ctx context.Context,
	req *pb.GetCitizenBuildingSummaryRequest,
) (*pb.GetCitizenBuildingSummaryResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	result, err := h.service.GetSummary(ctx, req.UserId, req.AllowedKarbaris)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get building summary: %v", err)
	}

	data := make([]*pb.CitizenBuildingSummaryItem, 0, len(result.Items))
	for _, item := range result.Items {
		data = append(data, &pb.CitizenBuildingSummaryItem{
			Karbari: item.Karbari,
			Label:   item.Label,
			Count:   item.Count,
		})
	}

	return &pb.GetCitizenBuildingSummaryResponse{Data: data}, nil
}

func (h *CitizenBuildingsHandler) GetCitizenBuildingChart(
	ctx context.Context,
	req *pb.GetCitizenBuildingChartRequest,
) (*pb.GetCitizenBuildingChartResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	chart, period, err := h.service.GetChart(ctx, req.UserId, req.Period, req.AllowedKarbaris)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get building chart: %v", err)
	}

	return &pb.GetCitizenBuildingChartResponse{
		Data: &pb.CitizenBuildingChartData{
			Labels:    chart.Labels,
			Completed: chart.Completed,
		},
		Period: period,
	}, nil
}

func (h *CitizenBuildingsHandler) ListCitizenBuildings(
	ctx context.Context,
	req *pb.ListCitizenBuildingsRequest,
) (*pb.ListCitizenBuildingsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}

	result, err := h.service.GetBuildings(ctx, req.UserId, req.AllowedKarbaris, page)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list citizen buildings: %v", err)
	}

	data := make([]*pb.CitizenBuildingItem, 0, len(result.Items))
	for _, item := range result.Items {
		data = append(data, mapCitizenBuildingItem(item))
	}

	basePath := result.Path
	if basePath == "" {
		basePath = models.CitizenBuildingPath
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

	return &pb.ListCitizenBuildingsResponse{
		Data:  data,
		Links: links,
		Meta:  meta,
	}, nil
}

func mapCitizenBuildingItem(item models.CitizenBuildingListItem) *pb.CitizenBuildingItem {
	out := &pb.CitizenBuildingItem{
		FeaturePropertiesId: item.FeaturePropertiesID,
		Karbari:             item.Karbari,
	}
	if item.Area != nil {
		out.Area = item.Area
	}
	if item.Visitors != nil {
		out.Visitors = item.Visitors
	}
	if item.EmptyUnits != nil {
		out.EmptyUnits = item.EmptyUnits
	}
	if item.Floors != nil {
		out.Floors = item.Floors
	}
	if item.ConstructionEndDate != nil {
		out.ConstructionEndDate = item.ConstructionEndDate
	}
	return out
}
