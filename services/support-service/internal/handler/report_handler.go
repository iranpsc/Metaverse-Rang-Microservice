package handler

import (
	"context"
	"strings"

	"metargb/support-service/internal/models"
	"metargb/support-service/internal/service"
	"metargb/support-service/internal/utils"

	pbCommon "metargb/shared/pb/common"
	pb "metargb/shared/pb/support"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReportHandler struct {
	pb.UnimplementedReportServiceServer
	reportService service.ReportService
}

func NewReportHandler(reportService service.ReportService) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
	}
}

func RegisterReportHandler(grpcServer *grpc.Server, reportService service.ReportService) {
	handler := NewReportHandler(reportService)
	pb.RegisterReportServiceServer(grpcServer, handler)
}

func reportImagePublicURL(stored string) string {
	if stored == "" {
		return ""
	}
	if strings.HasPrefix(stored, "http://") || strings.HasPrefix(stored, "https://") {
		return stored
	}
	stored = strings.TrimPrefix(stored, "/")
	return "uploads/" + stored
}

func (h *ReportHandler) CreateReport(ctx context.Context, req *pb.CreateReportRequest) (*pb.ReportResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("user_id", req.UserId, locale),
		validateReportSubject(req.ReportableType, locale),
		validateRequired("reason", req.Reason, locale),
		validateMaxLen("reason", req.Reason, 130, locale),
		validateRequired("description", req.Description, locale),
		validateMaxLen("description", req.Description, 2000, locale),
		validateRequired("url", req.Url, locale),
	)
	if len(req.ImagePaths) > 5 {
		validationErrors = mergeValidationErrors(validationErrors, map[string]string{
			"attachments": "The attachments field must not have more than 5 items",
		})
	}
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	report, err := h.reportService.CreateReport(ctx, req.UserId, req.ReportableType, req.Reason, req.Description, req.Url, req.ImagePaths)
	if err != nil {
		return nil, MapServiceError(err)
	}

	return convertReportWithImagesToProto(report), nil
}

func (h *ReportHandler) GetReports(ctx context.Context, req *pb.GetReportsRequest) (*pb.ReportsResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := validateRequired("user_id", req.UserId, locale)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	page := int32(1)
	perPage := int32(10)
	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			perPage = req.Pagination.PerPage
		}
	}

	reports, total, err := h.reportService.GetReports(ctx, req.UserId, page, perPage)
	if err != nil {
		return nil, MapServiceError(err)
	}

	response := &pb.ReportsResponse{
		Reports: make([]*pb.ReportResponse, len(reports)),
		Pagination: &pbCommon.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       int32(total),
			LastPage:    int32((total + int(perPage) - 1) / int(perPage)),
		},
	}

	for i, report := range reports {
		response.Reports[i] = convertReportToProto(report)
	}

	return response, nil
}

func (h *ReportHandler) GetReport(ctx context.Context, req *pb.GetReportRequest) (*pb.ReportResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("report_id", req.ReportId, locale),
		validateRequired("user_id", req.UserId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	report, err := h.reportService.GetReport(ctx, req.ReportId, req.UserId)
	if err != nil {
		return nil, MapServiceError(err)
	}

	if report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	return convertReportWithImagesToProto(report), nil
}

func convertReportToProto(report *models.Report) *pb.ReportResponse {
	if report == nil {
		return nil
	}
	return &pb.ReportResponse{
		Id:             report.ID,
		UserId:         report.UserID,
		ReportableType: report.Subject,
		ReportableId:   0,
		Reason:         report.Title,
		Description:    report.Content,
		Url:            report.URL,
		CreatedAt:      utils.FormatJalaliDateTime(report.CreatedAt),
	}
}

func convertReportWithImagesToProto(report *models.ReportWithImages) *pb.ReportResponse {
	resp := convertReportToProto(&report.Report)
	if len(report.Images) > 0 {
		resp.ImagePaths = make([]string, len(report.Images))
		for i, img := range report.Images {
			resp.ImagePaths[i] = img.URL
		}
	}
	return resp
}
