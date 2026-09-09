package service

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/shared/pkg/helpers"
)

func TestNewEmailChannelFromConfig_NoopWhenUnconfigured(t *testing.T) {
	ch := NewEmailChannelFromConfig(EmailChannelConfig{})
	_, err := ch.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
	if !errors.Is(err, errs.ErrNotImplemented) {
		t.Fatalf("expected unimplemented, got %v", err)
	}
}

func TestSMTPEmailChannel_SendEmail(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	var gotMsg string

	ch := &smtpEmailChannel{
		cfg: EmailChannelConfig{
			Host:      "smtp.example.com",
			Port:      "587",
			Username:  "user",
			Password:  "pass",
			FromName:  "metarang",
			FromEmail: "noreply@example.com",
		},
		send: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			gotAddr = addr
			gotFrom = from
			gotTo = append([]string{}, to...)
			gotMsg = string(msg)
			if a == nil {
				t.Fatal("expected smtp auth")
			}
			return nil
		},
	}

	id, err := ch.SendEmail(context.Background(), models.EmailPayload{
		To:       "member@example.com",
		Subject:  "Dynasty",
		Body:     "join request body",
		CC:       []string{"cc@example.com"},
		BCC:      []string{"bcc@example.com"},
		HTMLBody: "<p>html</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "smtp.example.com:587" {
		t.Fatalf("id=%q", id)
	}
	if gotAddr != "smtp.example.com:587" {
		t.Fatalf("addr=%q", gotAddr)
	}
	if gotFrom != "noreply@example.com" {
		t.Fatalf("from=%q", gotFrom)
	}
	if strings.Join(gotTo, ",") != "member@example.com,cc@example.com,bcc@example.com" {
		t.Fatalf("to=%v", gotTo)
	}
	if !strings.Contains(gotMsg, "To: member@example.com\r\n") {
		t.Fatalf("unexpected To header: %s", gotMsg)
	}
	if !strings.Contains(gotMsg, "Subject:") || !strings.Contains(gotMsg, "<p>html</p>") {
		t.Fatalf("unexpected message: %s", gotMsg)
	}
}

func TestSMTPEmailChannel_RejectsHeaderInjection(t *testing.T) {
	ch := &smtpEmailChannel{
		cfg: EmailChannelConfig{FromEmail: "from@example.com", Host: "h", Port: "25"},
		send: func(string, smtp.Auth, string, []string, []byte) error {
			t.Fatal("send should not be called for invalid addresses")
			return nil
		},
	}

	_, err := ch.SendEmail(context.Background(), models.EmailPayload{
		To:      "user@example.com\r\nBcc: evil@example.com",
		Subject: "s",
		Body:    "b",
	})
	if !errors.Is(err, helpers.ErrInvalidEmailAddress) {
		t.Fatalf("expected invalid email error, got %v", err)
	}

	_, err = ch.SendEmail(context.Background(), models.EmailPayload{
		To:      "user@example.com",
		Subject: "s",
		Body:    "b",
		CC:      []string{"cc@example.com\nBcc: evil@example.com"},
	})
	if !errors.Is(err, helpers.ErrInvalidEmailAddress) {
		t.Fatalf("expected invalid cc error, got %v", err)
	}
}

func TestSMTPEmailChannel_Validation(t *testing.T) {
	ch := &smtpEmailChannel{cfg: EmailChannelConfig{FromEmail: "from@example.com", Host: "h", Port: "25"}, send: func(string, smtp.Auth, string, []string, []byte) error {
		return nil
	}}
	if _, err := ch.SendEmail(context.Background(), models.EmailPayload{Subject: "s", Body: "b"}); err == nil {
		t.Fatal("expected recipient required")
	}
	if _, err := ch.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Body: "b"}); err == nil {
		t.Fatal("expected subject required")
	}
	if _, err := ch.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s"}); err == nil {
		t.Fatal("expected body required")
	}
}

func TestNewEmailChannelFromConfig_DefaultPort(t *testing.T) {
	ch := NewEmailChannelFromConfig(EmailChannelConfig{Host: "smtp.example.com", FromEmail: "from@example.com"})
	smtpCh, ok := ch.(*smtpEmailChannel)
	if !ok {
		t.Fatal("expected smtp channel")
	}
	if smtpCh.cfg.Port != "587" {
		t.Fatalf("port=%q", smtpCh.cfg.Port)
	}
}
