package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/repository"

	pbNotification "metarang/shared/pb/notifications"
	grpcutil "metarang/shared/pkg/grpc"
)

type TicketService interface {
	CreateTicket(ctx context.Context, userID uint64, title, content, attachment string, receiverID *uint64, department *string) (*models.TicketWithRelations, error)
	GetTickets(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error)
	GetTicket(ctx context.Context, ticketID, userID uint64) (*models.TicketWithRelations, error)
	UpdateTicket(ctx context.Context, ticketID, userID uint64, title, content, attachment string) (*models.TicketWithRelations, error)
	AddResponse(ctx context.Context, ticketID, userID uint64, response, attachment, userName string) (*models.TicketWithRelations, error)
	CloseTicket(ctx context.Context, ticketID, userID uint64) (*models.TicketWithRelations, error)
	CheckAuthorization(ctx context.Context, ticketID, userID uint64, action string) error
}

type ticketService struct {
	ticketRepo              repository.TicketRepository
	notificationServiceAddr string
}

func NewTicketService(ticketRepo repository.TicketRepository, notificationAddr string) TicketService {
	return &ticketService{
		ticketRepo:              ticketRepo,
		notificationServiceAddr: notificationAddr,
	}
}

func (s *ticketService) CreateTicket(ctx context.Context, userID uint64, title, content, attachment string, receiverID *uint64, department *string) (*models.TicketWithRelations, error) {
	code := rand.Int31n(900000) + 100000

	ticket := &models.Ticket{
		Title:      title,
		Content:    content,
		Attachment: attachment,
		Status:     models.TicketStatusNew,
		Department: department,
		Importance: 0,
		Code:       code,
		UserID:     userID,
		ReceiverID: receiverID,
	}

	createdTicket, err := s.ticketRepo.Create(ctx, ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Get full ticket with relations
	fullTicket, err := s.ticketRepo.GetByID(ctx, createdTicket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created ticket: %w", err)
	}

	if receiverID != nil {
		go s.sendTicketNotification(*receiverID, fullTicket, fullTicket.SenderName, senderImage(fullTicket))
	}

	return fullTicket, nil
}

func (s *ticketService) GetTickets(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if page <= 0 {
		page = 1
	}

	return s.ticketRepo.GetByUserID(ctx, userID, page, perPage, received)
}

func (s *ticketService) GetTicket(ctx context.Context, ticketID, userID uint64) (*models.TicketWithRelations, error) {
	// Check authorization
	if err := s.CheckAuthorization(ctx, ticketID, userID, "view"); err != nil {
		return nil, err
	}

	return s.ticketRepo.GetByID(ctx, ticketID)
}

func (s *ticketService) UpdateTicket(ctx context.Context, ticketID, userID uint64, title, content, attachment string) (*models.TicketWithRelations, error) {
	// Check authorization - only sender can update
	if err := s.CheckAuthorization(ctx, ticketID, userID, "update"); err != nil {
		return nil, err
	}

	// Get existing ticket
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	// Update fields
	ticket.Title = title
	ticket.Content = content
	if attachment != "" {
		ticket.Attachment = attachment
	}
	ticket.Status = models.TicketStatusNew

	err = s.ticketRepo.Update(ctx, &ticket.Ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Return updated ticket
	return s.ticketRepo.GetByID(ctx, ticketID)
}

func (s *ticketService) AddResponse(ctx context.Context, ticketID, userID uint64, response, attachment, userName string) (*models.TicketWithRelations, error) {
	// Check authorization - sender or receiver can respond if ticket is open
	if err := s.CheckAuthorization(ctx, ticketID, userID, "respond"); err != nil {
		return nil, err
	}

	// Get ticket to check if open
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	if ticket.IsClosed() {
		return nil, fmt.Errorf("cannot respond to closed ticket")
	}

	userName = responderDisplayName(ticket, userID, userName)

	ticketResponse := &models.TicketResponse{
		TicketID:      ticketID,
		Response:      response,
		Attachment:    attachment,
		ResponserName: userName,
		ResponserID:   userID,
	}

	_, err = s.ticketRepo.CreateResponse(ctx, ticketResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	status := ticket.Status
	if userID != ticket.UserID {
		status = models.TicketStatusAnswered
	}
	err = s.ticketRepo.UpdateStatus(ctx, ticketID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket status: %w", err)
	}

	updatedTicket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated ticket: %w", err)
	}

	if otherID := ticket.OtherParticipantID(userID); otherID != 0 {
		go s.sendTicketNotification(otherID, updatedTicket, userName, actorImage(ticket, userID))
	}

	return updatedTicket, nil
}

func (s *ticketService) CloseTicket(ctx context.Context, ticketID, userID uint64) (*models.TicketWithRelations, error) {
	// Check authorization - only sender can close if ticket is open
	if err := s.CheckAuthorization(ctx, ticketID, userID, "close"); err != nil {
		return nil, err
	}

	// Get ticket to check if open
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	if ticket.IsClosed() {
		return nil, fmt.Errorf("ticket is already closed")
	}

	// Update status to CLOSED
	err = s.ticketRepo.UpdateStatus(ctx, ticketID, models.TicketStatusClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to close ticket: %w", err)
	}

	return s.ticketRepo.GetByID(ctx, ticketID)
}

func (s *ticketService) CheckAuthorization(ctx context.Context, ticketID, userID uint64, action string) error {
	senderID, receiverID, err := s.ticketRepo.GetTicketSenderReceiver(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to get ticket info: %w", err)
	}

	switch action {
	case "view":
		if senderID != userID && receiverID != userID {
			return fmt.Errorf("unauthorized: you don't have permission to view this ticket")
		}
	case "update":
		if senderID != userID {
			return fmt.Errorf("unauthorized: only ticket sender can update")
		}
	case "respond":
		if senderID != userID && receiverID != userID {
			return fmt.Errorf("unauthorized: you don't have permission to respond to this ticket")
		}
	case "close":
		if senderID != userID {
			return fmt.Errorf("unauthorized: only ticket sender can close")
		}
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	return nil
}

func responderDisplayName(ticket *models.TicketWithRelations, userID uint64, fallback string) string {
	if userID == ticket.UserID && ticket.SenderName != "" {
		return ticket.SenderName
	}
	if ticket.ReceiverID != nil && *ticket.ReceiverID == userID && ticket.ReceiverName != nil && *ticket.ReceiverName != "" {
		return *ticket.ReceiverName
	}
	if name := strings.TrimSpace(fallback); name != "" {
		return name
	}
	return "User"
}

func senderImage(ticket *models.TicketWithRelations) string {
	if ticket != nil && ticket.SenderProfilePhoto != nil && *ticket.SenderProfilePhoto != "" {
		return *ticket.SenderProfilePhoto
	}
	return "uploads/img/logo.png"
}

func actorImage(ticket *models.TicketWithRelations, userID uint64) string {
	if ticket.UserID == userID {
		return senderImage(ticket)
	}
	if ticket.ReceiverProfilePhoto != nil && *ticket.ReceiverProfilePhoto != "" {
		return *ticket.ReceiverProfilePhoto
	}
	return "uploads/img/logo.png"
}

func (s *ticketService) sendTicketNotification(userID uint64, ticket *models.TicketWithRelations, actorName, actorImage string) {
	conn, err := grpcutil.NewClient(s.notificationServiceAddr)
	if err != nil {
		fmt.Printf("Failed to connect to notification service: %v\n", err)
		return
	}
	defer func() { _ = conn.Close() }()

	client := pbNotification.NewNotificationServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if actorName == "" {
		actorName = ticket.SenderName
	}
	if actorImage == "" {
		actorImage = senderImage(ticket)
	}

	message := fmt.Sprintf("تیکتی از طرف %s دریافت شده است", actorName)

	_, err = client.SendNotification(ctx, &pbNotification.SendNotificationRequest{
		UserId:  userID,
		Type:    "ticket_received",
		Title:   "تیکت جدید",
		Message: message,
		Data: map[string]string{
			"related-to":   "tickets",
			"sender-image": actorImage,
			"sender-name":  actorName,
			"ticket-id":    fmt.Sprintf("%d", ticket.ID),
		},
	})

	if err != nil {
		fmt.Printf("Failed to send notification: %v\n", err)
	}
}
