package handler

import (
	"context"
	"fmt"
	"time"

	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CitizenFeaturesHandler implements features.CitizenFeaturesServiceServer.
type CitizenFeaturesHandler struct {
	pb.UnimplementedCitizenFeaturesServiceServer
	service CitizenFeaturesServicePort
	now     func() time.Time
}

func NewCitizenFeaturesHandler(service CitizenFeaturesServicePort) *CitizenFeaturesHandler {
	return &CitizenFeaturesHandler{
		service: service,
		now:     time.Now,
	}
}

func (h *CitizenFeaturesHandler) GetCitizenFeatureSummary(
	ctx context.Context,
	req *pb.GetCitizenFeatureSummaryRequest,
) (*pb.GetCitizenFeatureSummaryResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	result, err := h.service.GetSummary(ctx, req.UserId, req.Period, req.AllowedKarbaris, h.now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feature summary: %v", err)
	}

	data := make([]*pb.CitizenFeatureSummaryItem, 0, len(result.Items))
	for _, item := range result.Items {
		data = append(data, &pb.CitizenFeatureSummaryItem{
			Karbari:      item.Karbari,
			Label:        item.Label,
			CurrentCount: item.CurrentCount,
			BoughtCount:  item.BoughtCount,
			SoldCount:    item.SoldCount,
		})
	}

	return &pb.GetCitizenFeatureSummaryResponse{
		Data:   data,
		Period: result.Period,
	}, nil
}

func (h *CitizenFeaturesHandler) GetCitizenFeatureChart(
	ctx context.Context,
	req *pb.GetCitizenFeatureChartRequest,
) (*pb.GetCitizenFeatureChartResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	chart, err := h.service.GetChart(ctx, req.UserId, req.Period, req.AllowedKarbaris, h.now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get feature chart: %v", err)
	}

	return &pb.GetCitizenFeatureChartResponse{
		Data: &pb.CitizenFeatureChartData{
			Labels: chart.Labels,
			Bought: chart.Bought,
			Sold:   chart.Sold,
		},
	}, nil
}

func (h *CitizenFeaturesHandler) ListCitizenFeatures(
	ctx context.Context,
	req *pb.ListCitizenFeaturesRequest,
) (*pb.ListCitizenFeaturesResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	perPage := int(req.PerPage)
	if perPage < 1 {
		perPage = 15
	}

	result, err := h.service.GetFeatures(ctx, req.UserId, req.AllowedKarbaris, req.Search, page, perPage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list citizen features: %v", err)
	}

	data := make([]*pb.CitizenFeatureItem, 0, len(result.Items))
	for _, item := range result.Items {
		data = append(data, mapCitizenFeatureItem(item))
	}

	markers := make([]*pb.CitizenFeatureMapMarker, 0, len(result.MapMarkers))
	for _, marker := range result.MapMarkers {
		markers = append(markers, mapCitizenFeatureMapMarker(marker))
	}

	basePath := result.Path
	if basePath == "" {
		basePath = "/api/citizen/features"
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

	return &pb.ListCitizenFeaturesResponse{
		Data:       data,
		Links:      links,
		Meta:       meta,
		MapMarkers: markers,
	}, nil
}

func mapCitizenFeatureItem(item models.CitizenFeatureListItem) *pb.CitizenFeatureItem {
	out := &pb.CitizenFeatureItem{
		Id:        item.ID,
		VodId:     item.VodID,
		Address:   item.Address,
		Area:      item.Area,
		Density:   item.Density,
		Karbari:   item.Karbari,
		OwnerCode: item.OwnerCode,
		PricePsc:  item.PricePSC,
		PriceIrr:  item.PriceIRR,
		Label:     item.Label,
	}
	if item.Center != nil {
		out.Center = &pb.CitizenFeatureCenter{X: item.Center.X, Y: item.Center.Y}
	}
	images := make([]*pb.Image, 0, len(item.Images))
	for _, img := range item.Images {
		images = append(images, &pb.Image{Id: img.ID, Url: img.URL})
	}
	out.Images = images
	return out
}

func mapCitizenFeatureMapMarker(marker models.CitizenFeatureMapMarker) *pb.CitizenFeatureMapMarker {
	out := &pb.CitizenFeatureMapMarker{
		Id:      marker.ID,
		Karbari: marker.Karbari,
	}
	if marker.Center != nil {
		out.Center = &pb.CitizenFeatureCenter{X: marker.Center.X, Y: marker.Center.Y}
	}
	return out
}
