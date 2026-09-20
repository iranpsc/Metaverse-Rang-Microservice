package handler_test

import (
	"context"
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

func ticketRelations(id, userID uint64) *models.TicketWithRelations {
	return &models.TicketWithRelations{
		Ticket: models.Ticket{
			ID:        id,
			UserID:    userID,
			Status:    models.TicketStatusNew,
			Title:     "t",
			Content:   "c",
			Code:      222222,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		SenderName: "S",
	}
}

func TestTicketHandler_CreateTicket_ValidationNoReceiverOrDept(t *testing.T) {
	repo := &testutil.MockTicketRepo{}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	_, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId: 1, Title: "a", Content: "b",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_CreateTicket_Success(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			ticket.ID = 77
			return ticket, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return ticketRelations(ticketID, 5), nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	dept := models.DeptTechnicalSupport
	resp, err := client.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		UserId:     5,
		Title:      "t",
		Content:    "c",
		Department: dept,
	})
	if err != nil || resp.Id != 77 {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
	if len(resp.Messages) != 1 || resp.Messages[0].Kind != models.TicketMessageKindOpening || !resp.Messages[0].IsMine {
		t.Fatalf("create thread=%+v", resp.Messages)
	}
}

func TestTicketHandler_GetTicket_PermissionDenied(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	_, err := client.GetTicket(context.Background(), &pb.GetTicketRequest{TicketId: 9, UserId: 99})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestTicketHandler_GetTickets_WithReceived(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetByUserIDFunc: func(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error) {
			if received {
				return []*models.TicketWithRelations{ticketRelations(1, userID)}, 1, nil
			}
			return nil, 0, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	resp, err := client.GetTickets(context.Background(), &pb.GetTicketsRequest{
		UserId:   3,
		Received: true,
		Pagination: &pbCommon.PaginationRequest{
			Page: 1, PerPage: 10,
		},
	})
	if err != nil || len(resp.Tickets) != 1 {
		t.Fatalf("err=%v tickets=%d", err, len(resp.Tickets))
	}
	if len(resp.Tickets[0].Messages) != 0 {
		t.Fatalf("list payload should omit chat thread, got %d messages", len(resp.Tickets[0].Messages))
	}
}

func TestTicketHandler_GetTicket_Success(t *testing.T) {
	rid := uint64(8)
	rname := "Recv"
	rcode := "R1"
	rphoto := "p.jpg"
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 5, rid, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			tr := ticketRelations(ticketID, 5)
			tr.ReceiverID = &rid
			tr.ReceiverName = &rname
			tr.ReceiverCode = &rcode
			tr.ReceiverProfilePhoto = &rphoto
			tr.Responses = []models.TicketResponse{
				{ID: 1, TicketID: ticketID, Response: "hi", ResponserName: "Bob", ResponserID: 5, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: 2, TicketID: ticketID, Response: "ack", ResponserName: "Recv", ResponserID: 8, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			}
			return tr, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	resp, err := client.GetTicket(context.Background(), &pb.GetTicketRequest{TicketId: 3, UserId: 5})
	if err != nil || resp.Id != 3 || len(resp.Responses) != 2 || resp.Receiver == nil {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
	if resp.CurrentUserId != 5 || len(resp.Messages) != 3 {
		t.Fatalf("viewer=%d messages=%d", resp.CurrentUserId, len(resp.Messages))
	}
	if resp.Messages[0].Kind != models.TicketMessageKindOpening || !resp.Messages[0].IsMine {
		t.Fatalf("opening=%+v", resp.Messages[0])
	}
	if !resp.Messages[1].IsMine || resp.Messages[1].Role != models.TicketMessageRoleSender {
		t.Fatalf("sender reply=%+v", resp.Messages[1])
	}
	if resp.Messages[2].IsMine || resp.Messages[2].Role != models.TicketMessageRoleReceiver || resp.Messages[2].Author.Id != 8 {
		t.Fatalf("receiver reply=%+v", resp.Messages[2])
	}

	asReceiver, err := client.GetTicket(context.Background(), &pb.GetTicketRequest{TicketId: 3, UserId: 8})
	if err != nil || asReceiver.CurrentUserId != 8 {
		t.Fatalf("receiver err=%v viewer=%d", err, asReceiver.GetCurrentUserId())
	}
	if asReceiver.Messages[0].IsMine || asReceiver.Messages[1].IsMine || !asReceiver.Messages[2].IsMine {
		t.Fatalf("receiver-side is_mine=[%v %v %v]", asReceiver.Messages[0].IsMine, asReceiver.Messages[1].IsMine, asReceiver.Messages[2].IsMine)
	}
}

func TestTicketHandler_UpdateTicket_Success(t *testing.T) {
	tk := ticketRelations(10, 6)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 6, 0, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		UpdateFunc: func(ctx context.Context, ticket *models.Ticket) error {
			tk.Title = ticket.Title
			tk.Content = ticket.Content
			return nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	resp, err := client.UpdateTicket(context.Background(), &pb.UpdateTicketRequest{
		TicketId: 10, UserId: 6, Title: "nt", Content: "nc", Attachment: "",
	})
	if err != nil || resp.Title != "nt" {
		t.Fatalf("err=%v %+v", err, resp)
	}
}

func TestTicketHandler_CloseTicket_Success(t *testing.T) {
	tk := ticketRelations(11, 8)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 8, 0, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		UpdateStatusFunc: func(ctx context.Context, ticketID uint64, status int32) error {
			tk.Status = status
			return nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	resp, err := client.CloseTicket(context.Background(), &pb.CloseTicketRequest{TicketId: 11, UserId: 8})
	if err != nil || resp.Status != models.TicketStatusClosed {
		t.Fatalf("err=%v st=%d", err, resp.Status)
	}
}

func TestTicketHandler_AddResponse_FailedPreconditionClosed(t *testing.T) {
	tk := ticketRelations(4, 1)
	tk.Status = models.TicketStatusClosed
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterTicketHandler(s, svc)
	})
	defer cleanup()
	client := pb.NewTicketServiceClient(conn)
	_, err := client.AddResponse(context.Background(), &pb.AddResponseRequest{
		TicketId: 4, UserId: 1, Response: "hi", UserName: "Bob",
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("got %v", err)
	}
}
