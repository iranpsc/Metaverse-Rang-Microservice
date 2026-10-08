package handler_test

import (
	"context"
	"strings"
	"testing"
	"time"

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

func ticketClient(t *testing.T, repo *testutil.MockTicketRepo) (pb.TicketServiceClient, func()) {
	t.Helper()
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	return pb.NewTicketServiceClient(conn), cleanup
}

func TestTicketHandler_TicketContentMaxLen(t *testing.T) {
	client, cleanup := ticketClient(t, &testutil.MockTicketRepo{})
	defer cleanup()

	_, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: "t", Content: strings.Repeat("a", 5001), Department: models.DeptTechnicalSupport,
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("create over limit got %v", err)
	}

	_, err = client.UpdateTicket(context.Background(), &pb.UpdateTicketRequest{
		TicketId: 1, UserId: 1, Title: "t", Content: strings.Repeat("a", 5001),
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("update over limit got %v", err)
	}

	content := strings.Repeat("a", 5000)
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			if ticket.Content != content {
				t.Fatalf("stored content len=%d", len(ticket.Content))
			}
			ticket.ID = 77
			return ticket, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return ticketRelations(ticketID, 1), nil
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	resp, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: "t", Content: content, Department: models.DeptTechnicalSupport,
	})
	if err != nil || resp.Id != 77 {
		t.Fatalf("create at limit err=%v resp=%+v", err, resp)
	}
}

func TestTicketHandler_CreateTicket_MissingFieldsBothReceiverAndDeptAndInternal(t *testing.T) {
	client, cleanup := ticketClient(t, &testutil.MockTicketRepo{})
	defer cleanup()

	_, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{UserId: 1, Title: "", Content: "c", Department: models.DeptTechnicalSupport})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("empty title got %v", err)
	}

	_, err = client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: strings.Repeat("t", 251), Content: "c", Department: models.DeptTechnicalSupport,
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("title max got %v", err)
	}

	_, err = client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: "t", Content: "c", ReceiverId: 9, Department: models.DeptTechnicalSupport,
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("both receiver and dept got %v", err)
	}

	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			return nil, errString("db down")
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	_, err = client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: "t", Content: "c", ReceiverId: 9,
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("internal got %v", err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestTicketHandler_GetTickets_DefaultPaginationAndInternal(t *testing.T) {
	var page, per int32
	repo := &testutil.MockTicketRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, p, pp int32, received bool) ([]*models.TicketWithRelations, int, error) {
			page, per = p, pp
			return []*models.TicketWithRelations{ticketRelations(1, userID)}, 1, nil
		},
	}
	client, cleanup := ticketClient(t, repo)
	defer cleanup()
	resp, err := client.GetTickets(context.Background(), &pb.GetTicketsRequest{UserId: 3})
	if err != nil || len(resp.Tickets) != 1 || page != 1 || per != 10 {
		t.Fatalf("err=%v page=%d per=%d", err, page, per)
	}
	if resp.Pagination == nil || resp.Pagination.CurrentPage != 1 || resp.Pagination.PerPage != 10 {
		t.Fatalf("pagination=%+v", resp.Pagination)
	}

	repo = &testutil.MockTicketRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, p, pp int32, received bool) ([]*models.TicketWithRelations, int, error) {
			return nil, 0, errString("list failed")
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	_, err = client.GetTickets(context.Background(), &pb.GetTicketsRequest{UserId: 3})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_GetTicket_NotFound(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 0, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return nil, nil
		},
	}
	client, cleanup := ticketClient(t, repo)
	defer cleanup()
	_, err := client.GetTicket(context.Background(), &pb.GetTicketRequest{TicketId: 9, UserId: 1})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_UpdateTicket_PermissionDeniedAndValidation(t *testing.T) {
	client, cleanup := ticketClient(t, &testutil.MockTicketRepo{})
	defer cleanup()
	_, err := client.UpdateTicket(context.Background(), &pb.UpdateTicketRequest{TicketId: 1, UserId: 1, Title: "", Content: "c"})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}

	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 6, 0, nil
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	_, err = client.UpdateTicket(context.Background(), &pb.UpdateTicketRequest{
		TicketId: 10, UserId: 99, Title: "nt", Content: "nc",
	})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_AddResponse_SuccessDefaultUserNameAndValidation(t *testing.T) {
	client, cleanup := ticketClient(t, &testutil.MockTicketRepo{})
	defer cleanup()
	_, err := client.AddResponse(context.Background(), &pb.AddResponseRequest{TicketId: 1, UserId: 1, Response: ""})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}

	tk := ticketRelations(8, 1)
	var userName string
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		CreateResponseFunc: func(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error) {
			userName = response.ResponserName
			return response, nil
		},
		UpdateStatusFunc: func(ctx context.Context, ticketID uint64, status int32) error {
			tk.Status = status
			return nil
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	resp, err := client.AddResponse(context.Background(), &pb.AddResponseRequest{
		TicketId: 8, UserId: 2, Response: "reply",
	})
	if err != nil || resp.Status != models.TicketStatusAnswered || userName != "User" {
		t.Fatalf("err=%v status=%d userName=%q", err, resp.Status, userName)
	}
}

func TestTicketHandler_CloseTicket_PermissionAlreadyClosed(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 8, 0, nil
		},
	}
	client, cleanup := ticketClient(t, repo)
	defer cleanup()
	_, err := client.CloseTicket(context.Background(), &pb.CloseTicketRequest{TicketId: 11, UserId: 99})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}

	tk := ticketRelations(11, 8)
	tk.Status = models.TicketStatusClosed
	repo = &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 8, 0, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
	}
	client, cleanup = ticketClient(t, repo)
	defer cleanup()
	_, err = client.CloseTicket(context.Background(), &pb.CloseTicketRequest{TicketId: 11, UserId: 8})
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_CreateTicket_SuccessWithReceiverJalaliFields(t *testing.T) {
	rid := uint64(9)
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			ticket.ID = 77
			return ticket, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			tr := ticketRelations(ticketID, 5)
			tr.CreatedAt = time.Date(2024, 3, 20, 10, 0, 0, 0, time.UTC)
			tr.UpdatedAt = tr.CreatedAt
			tr.ReceiverID = &rid
			dept := models.DeptTechnicalSupport
			tr.Department = &dept
			photo := "sp.jpg"
			tr.SenderProfilePhoto = &photo
			return tr, nil
		},
	}
	client, cleanup := ticketClient(t, repo)
	defer cleanup()
	resp, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 5, Title: "t", Content: "c", ReceiverId: rid,
	})
	if err != nil || resp.Receiver == nil || resp.Department != models.DeptTechnicalSupport {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
	if resp.CreatedAt == "" || !strings.Contains(resp.CreatedAt, "/") {
		t.Fatalf("expected jalali created_at %q", resp.CreatedAt)
	}
	if resp.Sender == nil || resp.Sender.ProfilePhoto != "sp.jpg" {
		t.Fatalf("sender=%+v", resp.Sender)
	}
}

func TestTicketHandler_GetTickets_CustomPagination(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error) {
			if page != 3 || perPage != 2 {
				t.Fatalf("page=%d per=%d", page, perPage)
			}
			return []*models.TicketWithRelations{ticketRelations(1, userID)}, 5, nil
		},
	}
	client, cleanup := ticketClient(t, repo)
	defer cleanup()
	resp, err := client.GetTickets(context.Background(), &pb.GetTicketsRequest{
		UserId:     3,
		Pagination: &pbCommon.PaginationRequest{Page: 3, PerPage: 2},
	})
	if err != nil || resp.Pagination.LastPage != 3 {
		t.Fatalf("err=%v pag=%+v", err, resp.Pagination)
	}
}
