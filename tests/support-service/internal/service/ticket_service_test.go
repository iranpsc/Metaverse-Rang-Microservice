package service_test

import (
	"context"
	"testing"
	"time"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
)

func ticketFull(id, userID uint64, status int32) *models.TicketWithRelations {
	return &models.TicketWithRelations{
		Ticket: models.Ticket{
			ID:        id,
			UserID:    userID,
			Status:    status,
			Title:     "t",
			Content:   "c",
			Code:      111111,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		SenderName: "Alice",
	}
}

func TestTicketService_CreateAndList(t *testing.T) {
	var createdID uint64
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			createdID = 42
			ticket.ID = createdID
			return ticket, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return ticketFull(ticketID, 10, models.TicketStatusNew), nil
		},
		GetByUserIDFunc: func(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error) {
			if !received {
				return []*models.TicketWithRelations{ticketFull(1, userID, models.TicketStatusNew)}, 1, nil
			}
			return nil, 0, nil
		},
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 10, 20, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	dept := models.DeptTechnicalSupport
	got, err := svc.CreateTicket(context.Background(), 10, "t", "c", "", nil, &dept)
	if err != nil || got.ID != createdID {
		t.Fatalf("create err=%v id=%d", err, got.ID)
	}
	list, total, err := svc.GetTickets(context.Background(), 10, 1, 10, false)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("list err=%v total=%d n=%d", err, total, len(list))
	}
}

func TestTicketService_GetTicketViewDenied(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.GetTicket(context.Background(), 9, 99)
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestTicketService_UpdateTicketSenderOnly(t *testing.T) {
	tk := ticketFull(5, 7, models.TicketStatusNew)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 7, 8, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		UpdateFunc: func(ctx context.Context, ticket *models.Ticket) error {
			tk.Title = ticket.Title
			return nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.UpdateTicket(context.Background(), 5, 8, "x", "y", "")
	if err == nil {
		t.Fatal("expected non-sender denied")
	}
	_, err = svc.UpdateTicket(context.Background(), 5, 7, "x", "y", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestTicketService_AddResponseClosedTicket(t *testing.T) {
	tk := ticketFull(3, 1, models.TicketStatusClosed)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.AddResponse(context.Background(), 3, 1, "hi", "", "Bob")
	if err == nil {
		t.Fatal("expected error on closed ticket")
	}
}

func TestTicketService_CloseTicketAlreadyClosed(t *testing.T) {
	tk := ticketFull(4, 9, models.TicketStatusClosed)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 9, 0, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.CloseTicket(context.Background(), 4, 9)
	if err == nil {
		t.Fatal("expected already closed")
	}
}

func TestTicketService_AddResponseOK(t *testing.T) {
	tk := ticketFull(8, 1, models.TicketStatusNew)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		CreateResponseFunc: func(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error) {
			return response, nil
		},
		UpdateStatusFunc: func(ctx context.Context, ticketID uint64, status int32) error {
			tk.Status = status
			return nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	out, err := svc.AddResponse(context.Background(), 8, 2, "reply", "", "Support")
	if err != nil || out.Status != models.TicketStatusAnswered {
		t.Fatalf("err=%v st=%d", err, out.Status)
	}
}

func TestTicketService_AddResponse_SenderKeepsStatusAndUsesNames(t *testing.T) {
	rid := uint64(2)
	rname := "Support"
	tk := ticketFull(8, 1, models.TicketStatusNew)
	tk.ReceiverID = &rid
	tk.ReceiverName = &rname
	var storedName string
	var storedStatus int32
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 1, 2, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			return tk, nil
		},
		CreateResponseFunc: func(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error) {
			storedName = response.ResponserName
			return response, nil
		},
		UpdateStatusFunc: func(ctx context.Context, ticketID uint64, status int32) error {
			storedStatus = status
			tk.Status = status
			return nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	out, err := svc.AddResponse(context.Background(), 8, 1, "follow-up", "", "")
	if err != nil || out.Status != models.TicketStatusNew || storedStatus != models.TicketStatusNew || storedName != "Alice" {
		t.Fatalf("sender reply err=%v st=%d stored=%d name=%q", err, out.Status, storedStatus, storedName)
	}

	out, err = svc.AddResponse(context.Background(), 8, 2, "reply", "", "Ignored")
	if err != nil || out.Status != models.TicketStatusAnswered || storedName != "Support" {
		t.Fatalf("receiver reply err=%v st=%d name=%q", err, out.Status, storedName)
	}
}

func TestTicketService_CloseTicketOK(t *testing.T) {
	tk := ticketFull(6, 9, models.TicketStatusNew)
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 9, 0, nil
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
	out, err := svc.CloseTicket(context.Background(), 6, 9)
	if err != nil || out.Status != models.TicketStatusClosed {
		t.Fatalf("err=%v status=%d", err, out.Status)
	}
}
