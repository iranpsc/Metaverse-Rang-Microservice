package service

import (
	"testing"

	"metarang/support-service/internal/models"
)

func TestTicketNotificationText(t *testing.T) {
	ticket := &models.TicketWithRelations{Ticket: models.Ticket{Code: 123456}}

	tests := []struct {
		name    string
		kind    ticketNotificationKind
		title   string
		message string
	}{
		{
			name:    "received ticket",
			kind:    ticketNotificationReceived,
			title:   "تیکت جدید",
			message: "Alice یک تیکت جدید با کد 123456 برای شما ارسال کرده است",
		},
		{
			name:    "ticket response",
			kind:    ticketNotificationResponse,
			title:   "پاسخ جدید به تیکت",
			message: "Alice به تیکت شما با کد 123456 پاسخ داده است",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, message := ticketNotificationText(ticket, "Alice", tt.kind)
			if title != tt.title || message != tt.message {
				t.Fatalf("got title=%q message=%q, want title=%q message=%q", title, message, tt.title, tt.message)
			}
		})
	}
}
