package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/resilience"
	"metarang/shared/pkg/helpers"
)

type noopEmailChannel struct{}

const (
	defaultSMTPTimeout = 10 * time.Second
	smtpDialTimeout    = 5 * time.Second
)

// EmailChannelConfig holds SMTP settings (SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM_*).
type EmailChannelConfig struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromName  string
	FromEmail string
	Timeout   time.Duration
}

type smtpMailSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

type smtpDialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// netDialError marks a failure before an SMTP session exists, so the send may be retried once.
type netDialError struct {
	err error
}

func (e *netDialError) Error() string {
	return "smtp dial: " + e.err.Error()
}

func (e *netDialError) Unwrap() error { return e.err }

type smtpEmailChannel struct {
	cfg     EmailChannelConfig
	send    smtpMailSender
	dial    smtpDialFunc
	timeout time.Duration
	breaker *resilience.Breaker
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
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	return &smtpEmailChannel{
		cfg:     cfg,
		timeout: timeout,
		breaker: resilience.NewBreaker(resilience.DefaultFailureThreshold, resilience.DefaultOpenCooldown),
	}
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

	from := c.cfg.FromEmail
	if c.cfg.FromName != "" {
		fromAddr := mail.Address{Name: c.cfg.FromName, Address: c.cfg.FromEmail}
		from = fromAddr.String()
	}

	to, err := helpers.ParseEmailAddress(payload.To)
	if err != nil {
		return "", fmt.Errorf("invalid email recipient: %w", err)
	}

	ccRecipients, err := parseEmailRecipientList(payload.CC)
	if err != nil {
		return "", fmt.Errorf("invalid cc recipient: %w", err)
	}
	bccRecipients, err := parseEmailRecipientList(payload.BCC)
	if err != nil {
		return "", fmt.Errorf("invalid bcc recipient: %w", err)
	}

	contentType := "text/plain; charset=UTF-8"
	body := payload.Body
	if payload.HTMLBody != "" {
		contentType = "text/html; charset=UTF-8"
		body = payload.HTMLBody
	}

	msg := strings.Builder{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + formatEmailHeaderAddress(to) + "\r\n")
	for _, cc := range ccRecipients {
		msg.WriteString("Cc: " + formatEmailHeaderAddress(cc) + "\r\n")
	}
	msg.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", payload.Subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: " + contentType + "\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	recipients := []string{to}
	recipients = append(recipients, ccRecipients...)
	recipients = append(recipients, bccRecipients...)

	addr := net.JoinHostPort(c.cfg.Host, c.cfg.Port)
	var auth smtp.Auth
	if c.cfg.Username != "" {
		auth = smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
	}

	rawMsg := []byte(msg.String())
	err = c.deliver(ctx, addr, auth, c.cfg.FromEmail, recipients, rawMsg)
	if err != nil {
		if errors.Is(err, resilience.ErrCircuitOpen) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", fmt.Errorf("smtp send failed: %w", err)
	}
	return addr, nil
}

func (c *smtpEmailChannel) deliver(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = c.cfg.Timeout
	}
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.breaker.Execute(func() error {
		if c.send != nil {
			return c.invokeSend(ctx, addr, auth, from, to, msg)
		}
		err := c.sendOnce(ctx, addr, auth, from, to, msg)
		if err == nil || ctx.Err() != nil || !errors.As(err, new(*netDialError)) {
			return err
		}
		return c.sendOnce(ctx, addr, auth, from, to, msg)
	}, smtpTransportFailure)
}

func (c *smtpEmailChannel) invokeSend(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.send(addr, auth, from, to, msg)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *smtpEmailChannel) dialContext(ctx context.Context, addr string) (net.Conn, error) {
	if c.dial != nil {
		return c.dial(ctx, "tcp", addr)
	}
	timeout := smtpDialTimeout
	if c.timeout > 0 && c.timeout < timeout {
		timeout = c.timeout
	}
	return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
}

func (c *smtpEmailChannel) sendOnce(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := c.dialContext(ctx, addr)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &netDialError{err: err}
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		_ = conn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: c.cfg.Host}); err != nil {
			return preferContextErr(ctx, err)
		}
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return preferContextErr(ctx, err)
		}
	}
	if err := client.Mail(from); err != nil {
		return preferContextErr(ctx, err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return preferContextErr(ctx, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return preferContextErr(ctx, err)
	}
	if _, err := writer.Write(msg); err != nil {
		return preferContextErr(ctx, err)
	}
	if err := writer.Close(); err != nil {
		return preferContextErr(ctx, err)
	}
	return preferContextErr(ctx, client.Quit())
}

func preferContextErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func smtpTransportFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, resilience.ErrCircuitOpen) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, new(*netDialError)) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func formatEmailHeaderAddress(address string) string {
	return (&mail.Address{Address: address}).String()
}

func parseEmailRecipientList(addresses []string) ([]string, error) {
	if len(addresses) == 0 {
		return nil, nil
	}
	parsed := make([]string, 0, len(addresses))
	for _, raw := range addresses {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		addr, err := helpers.ParseEmailAddress(raw)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, addr)
	}
	return parsed, nil
}

func IsInvalidEmailAddress(err error) bool {
	return errors.Is(err, helpers.ErrInvalidEmailAddress)
}
