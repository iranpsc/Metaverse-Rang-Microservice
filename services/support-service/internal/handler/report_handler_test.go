package handler_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbCommon "metargb/shared/pb/common"
	pb "metargb/shared/pb/support"

	"metargb/support-service/internal/handler"
	"metargb/support-service/internal/models"
	"metargb/support-service/internal/service"
	"metargb/support-service/internal/testutil"
)

func TestReportHandler_CreateReport_InvalidSubject(t *testing.T) {
	repo := &testutil.MockReportRepo{}
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewReportServiceClient(conn)
	_, err := client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId:         1,
		ReportableType: "not_a_subject",
		Reason:         "r",
		Description:    "d",
		Url:            "https://u.test",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func TestReportHandler_CreateReport_Success(t *testing.T) {
	repo := &testutil.MockReportRepo{
		CreateFunc: func(ctx context.Context, report *models.Report) (*models.Report, error) {
			r := *report
			r.ID = 12
			r.CreatedAt = time.Now()
			r.UpdatedAt = r.CreatedAt
			return &r, nil
		},
		CreateImageFunc: func(ctx context.Context, reportID uint64, url string) error {
			return nil
		},
		GetByIDFunc: func(ctx context.Context, reportID uint64) (*models.ReportWithImages, error) {
			return &models.ReportWithImages{
				Report: models.Report{
					ID:        12,
					UserID:    1,
					Subject:   "displayError",
					Title:     "r",
					Content:   "d",
					URL:       "https://u.test",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Images: []models.Image{{URL: "pic.png"}},
			}, nil
		},
	}
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewReportServiceClient(conn)
	resp, err := client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId:         1,
		ReportableType: "displayError",
		Reason:         "r",
		Description:    "d",
		Url:            "https://u.test",
		ImagePaths:     []string{"pic.png"},
	})
	if err != nil || resp.Id != 12 || resp.Url != "https://u.test" {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestReportHandler_GetReport_PermissionDenied(t *testing.T) {
	repo := &testutil.MockReportRepo{
		GetByIDFunc: func(ctx context.Context, reportID uint64) (*models.ReportWithImages, error) {
			return &models.ReportWithImages{
				Report: models.Report{ID: 1, UserID: 100},
			}, nil
		},
	}
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewReportServiceClient(conn)
	_, err := client.GetReport(context.Background(), &pb.GetReportRequest{ReportId: 1, UserId: 2})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestReportHandler_GetReport_Success(t *testing.T) {
	repo := &testutil.MockReportRepo{
		GetByIDFunc: func(ctx context.Context, reportID uint64) (*models.ReportWithImages, error) {
			return &models.ReportWithImages{
				Report: models.Report{
					ID:        9,
					UserID:    3,
					Subject:   "FPSError",
					Title:     "t",
					Content:   "c",
					URL:       "https://x",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Images: []models.Image{{URL: "/storage/a.png"}},
			}, nil
		},
	}
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewReportServiceClient(conn)
	resp, err := client.GetReport(context.Background(), &pb.GetReportRequest{ReportId: 9, UserId: 3})
	if err != nil || resp.Id != 9 || len(resp.ImagePaths) != 1 {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestReportHandler_GetReports_Success(t *testing.T) {
	repo := &testutil.MockReportRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, page, perPage int32) ([]*models.Report, int, error) {
			return []*models.Report{{ID: 1, UserID: userID, Title: "x"}}, 1, nil
		},
	}
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewReportServiceClient(conn)
	resp, err := client.GetReports(context.Background(), &pb.GetReportsRequest{
		UserId:     9,
		Pagination: &pbCommon.PaginationRequest{Page: 1, PerPage: 5},
	})
	if err != nil || len(resp.Reports) != 1 {
		t.Fatalf("err=%v", err)
	}
}
