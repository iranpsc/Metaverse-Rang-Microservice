package handler_test

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/support"

	"metarang/support-service/internal/handler"
	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
)

func reportClient(t *testing.T, repo *testutil.MockReportRepo) (pb.ReportServiceClient, func()) {
	t.Helper()
	svc := service.NewReportService(repo)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterReportHandler(s, svc)
	})
	return pb.NewReportServiceClient(conn), cleanup
}

func TestReportHandler_CreateReport_ValidationEmptySubjectTooManyImagesAndInternal(t *testing.T) {
	client, cleanup := reportClient(t, &testutil.MockReportRepo{})
	defer cleanup()
	_, err := client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "", Reason: "r", Description: "d", Url: "https://u.test",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("empty subject got %v", err)
	}

	paths := make([]string, 6)
	for i := range paths {
		paths[i] = "p"
	}
	_, err = client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "displayError", Reason: "r", Description: "d", Url: "https://u.test", ImagePaths: paths,
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("too many images got %v", err)
	}

	_, err = client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "displayError", Reason: strings.Repeat("r", 131), Description: "d", Url: "https://u.test",
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("reason max got %v", err)
	}

	repo := &testutil.MockReportRepo{
		CreateFunc: func(ctx context.Context, report *models.Report) (*models.Report, error) {
			return nil, errString("db")
		},
	}
	client, cleanup = reportClient(t, repo)
	defer cleanup()
	_, err = client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "spellingError", Reason: "r", Description: "d", Url: "https://u.test",
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("got %v", err)
	}
}

func TestReportHandler_CreateReport_DescriptionMaxLen(t *testing.T) {
	client, cleanup := reportClient(t, &testutil.MockReportRepo{})
	defer cleanup()

	_, err := client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "displayError", Reason: "r", Description: strings.Repeat("a", 5001), Url: "https://u.test",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("description over limit got %v", err)
	}

	description := strings.Repeat("a", 5000)
	repo := &testutil.MockReportRepo{
		CreateFunc: func(_ context.Context, report *models.Report) (*models.Report, error) {
			if report.Content != description {
				t.Fatalf("stored description len=%d", len(report.Content))
			}
			report.ID = 77
			return report, nil
		},
		GetByIDFunc: func(_ context.Context, reportID uint64) (*models.ReportWithImages, error) {
			return &models.ReportWithImages{Report: models.Report{ID: reportID, Content: description}}, nil
		},
	}
	client, cleanup = reportClient(t, repo)
	defer cleanup()
	resp, err := client.CreateReport(context.Background(), &pb.CreateReportRequest{
		UserId: 1, ReportableType: "displayError", Reason: "r", Description: description, Url: "https://u.test",
	})
	if err != nil || resp.Id != 77 {
		t.Fatalf("create at limit err=%v resp=%+v", err, resp)
	}
}

func TestReportHandler_GetReports_DefaultPaginationAndInternal(t *testing.T) {
	var page, per int32
	repo := &testutil.MockReportRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, p, pp int32) ([]*models.Report, int, error) {
			page, per = p, pp
			return []*models.Report{{ID: 1, UserID: userID}}, 1, nil
		},
	}
	client, cleanup := reportClient(t, repo)
	defer cleanup()
	resp, err := client.GetReports(context.Background(), &pb.GetReportsRequest{UserId: 9})
	if err != nil || len(resp.Reports) != 1 || page != 1 || per != 10 {
		t.Fatalf("err=%v page=%d per=%d", err, page, per)
	}

	repo = &testutil.MockReportRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, p, pp int32) ([]*models.Report, int, error) {
			return nil, 0, errString("list")
		},
	}
	client, cleanup = reportClient(t, repo)
	defer cleanup()
	_, err = client.GetReports(context.Background(), &pb.GetReportsRequest{
		UserId: 9, Pagination: &pbCommon.PaginationRequest{Page: 2, PerPage: 5},
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("got %v", err)
	}
}

func TestReportHandler_GetReport_NotFoundNil(t *testing.T) {
	repo := &testutil.MockReportRepo{
		GetByIDFunc: func(ctx context.Context, reportID uint64) (*models.ReportWithImages, error) {
			return nil, nil
		},
	}
	client, cleanup := reportClient(t, repo)
	defer cleanup()
	_, err := client.GetReport(context.Background(), &pb.GetReportRequest{ReportId: 9, UserId: 3})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("got %v", err)
	}
}
