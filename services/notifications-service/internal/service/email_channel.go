package service

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/shared/pkg/helpers"
)

type noopEmailChannel struct{}

// EmailChannelConfig holds SMTP settings (SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM_*).
type EmailChannelConfig struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromName  string
	FromEmail string
}

type smtpMailSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

type smtpEmailChannel struct {
	cfg  EmailChannelConfig
	send smtpMailSender
}

// NewEmailChannel returns a placeholder email channel implementation.
func NewEmailChannel() EmailChannel {
	return &noopEmailChannel{}
}

// NewEmailChannelFromConfig returns an SMTP channel when host and from-address are set; otherwise noop.
func NewEmailChannelFromConfig(cfg EmailChannelConfig) EmailChannel {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.FromEmail) == "" {
		log.Println("Warning: SMTP_HOST or SMTP_FROM_EMAIL is not set, using noop email channel")
		return &noopEmailChannel{}
	}
	if strings.TrimSpace(cfg.Port) == "" {
		cfg.Port = "587"
	}
	return &smtpEmailChannel{cfg: cfg, send: smtp.SendMail}
}

func (c *noopEmailChannel) SendEmail(ctx context.Context, payload models.EmailPayload) (string, error) {
	return "", errs.ErrNotImplemented
}

func (c *smtpEmailChannel) SendEmail(ctx context.Context, payload models.EmailPayload) (string, error) {
	if payload.To == "" {
		return "", fmt.Errorf("email recipient is required")
	}
	if payload.Subject == "" {
		return "", fmt.Errorf("email subject is required")
	}
	if payload.Body == "" && payload.HTMLBody == "" {
		return "", fmt.Errorf("email body is required")
	}

	toAddr, err := helpers.ParseEmailAddress(payload.To)
	if err != nil {
		return "", fmt.Errorf("invalid email recipient: %w", err)
	}
	ccAddrs, err := helpers.ParseEmailAddresses(payload.CC)
	if err != nil {
		return "", fmt.Errorf("invalid email cc recipient: %w", err)
	}
	bccAddrs, err := helpers.ParseEmailAddresses(payload.BCC)
	if err != nil {
		return "", fmt.Errorf("invalid email bcc recipient: %w", err)
	}

	from := c.cfg.FromEmail
	if c.cfg.FromName != "" {
		fromAddr := mail.Address{Name: c.cfg.FromName, Address: c.cfg.FromEmail}
		from = fromAddr.String()
	}

	contentType := "text/plain; charset=UTF-8"
	body := payload.Body
	if payload.HTMLBody != "" {
		contentType = "text/html; charset=UTF-8"
		body = payload.HTMLBody
	}

	msg := strings.Builder{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + toAddr + "\r\n")
	for _, cc := range ccAddrs {
		msg.WriteString("Cc: " + cc + "\r\n")
	}
	msg.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", payload.Subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: " + contentType + "\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	recipients := append([]string{toAddr}, ccAddrs...)
	recipients = append(recipients, bccAddrs...)

	addr := net.JoinHostPort(c.cfg.Host, c.cfg.Port)
	var auth smtp.Auth
	if c.cfg.Username != "" {
		auth = smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
	}

	if err := c.send(addr, auth, c.cfg.FromEmail, recipients, []byte(msg.String())); err != nil {
		return "", fmt.Errorf("smtp send failed: %w", err)
	}
	return addr, nil
}
