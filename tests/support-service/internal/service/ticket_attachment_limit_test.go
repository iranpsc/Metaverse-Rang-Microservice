package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
)

func ticketAttachmentList(n int) string {
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("http://files.example/%d.pdf", i+1)
	}
	raw, err := json.Marshal(urls)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestTicketService_CreateTicket_RejectsMoreThanFiveAttachments(t *testing.T) {
	created := false
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			created = true
			return ticket, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.CreateTicket(context.Background(), 1, "title", "content", ticketAttachmentList(6), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}
	if created {
		t.Fatal("ticket must not be stored when it has more than 5 attachments")
	}
}

func TestTicketService_CreateTicket_AcceptsFiveAttachments(t *testing.T) {
	encoded := ticketAttachmentList(5)
	var stored string
	repo := &testutil.MockTicketRepo{
		CreateFunc: func(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
			stored = ticket.Attachment
			ticket.ID = 8
			return ticket, nil
		},
		GetByIDFunc: func(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
			tk := ticketFull(ticketID, 1, models.TicketStatusNew)
			tk.Attachment = stored
			return tk, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	got, err := svc.CreateTicket(context.Background(), 1, "title", "content", encoded, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Attachment != encoded {
		t.Fatalf("attachment=%s", got.Attachment)
	}
}

func TestTicketService_AddResponse_RejectsMoreThanFiveAttachments(t *testing.T) {
	repo := &testutil.MockTicketRepo{
		GetTicketSenderReceiverFunc: func(ctx context.Context, ticketID uint64) (uint64, uint64, error) {
			return 7, 8, nil
		},
		CreateResponseFunc: func(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error) {
			t.Fatal("response must not be stored")
			return response, nil
		},
	}
	svc := service.NewTicketService(repo, "127.0.0.1:1")
	_, err := svc.AddResponse(context.Background(), 5, 7, "reply", ticketAttachmentList(6), "Alice")
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}
}
