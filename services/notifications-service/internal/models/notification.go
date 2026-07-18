// Package models defines domain types for notifications.
package models

import "time"

// Notification represents an individual notification destined for a user.
type Notification struct {
	ID        string
	UserID    uint64
	Type      string
	Title     string
	Message   string
	Data      map[string]string
	ReadAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationResult captures the outcome of a notification dispatch request.
type NotificationResult struct {
	ID   uint64
	Sent bool
}

// NotificationFilter defines pagination and filtering information when querying notifications.
type NotificationFilter struct {
	Page       int32
	PerPage    int32
	UnreadOnly bool // Filter only unread notifications
}

// SMSPayload contains the minimal information required to send an SMS.
type SMSPayload struct {
	Phone    string
	Message  string
	Template string
	Tokens   map[string]string
}

// OTPPayload contains information needed to send an OTP via SMS.
type OTPPayload struct {
	Phone  string
	Code   string
	Reason string
}

// EmailPayload contains the minimal information required to send an email.
type EmailPayload struct {
	To       string
	Subject  string
	Body     string
	HTMLBody string
	CC       []string
	BCC      []string
}
