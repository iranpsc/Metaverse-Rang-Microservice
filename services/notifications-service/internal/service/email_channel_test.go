package service

import (
	"context"
	"errors"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/resilience"
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
	if !strings.Contains(gotMsg, "Subject:") || !strings.Contains(gotMsg, "<p>html</p>") {
		t.Fatalf("unexpected message: %s", gotMsg)
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

func TestSMTPEmailChannel_RejectsHeaderInjection(t *testing.T) {
	ch := &smtpEmailChannel{
		cfg: EmailChannelConfig{FromEmail: "from@example.com", Host: "h", Port: "25"},
		send: func(string, smtp.Auth, string, []string, []byte) error {
			t.Fatal("send should not be called for invalid recipients")
			return nil
		},
	}

	cases := []models.EmailPayload{
		{To: "user@example.com\nCc: attacker@evil.com", Subject: "s", Body: "b"},
		{To: "user@example.com", Subject: "s", Body: "b", CC: []string{"cc@example.com\r\nBcc: attacker@evil.com"}},
		{To: "user@example.com", Subject: "s", Body: "b", BCC: []string{"bcc@example.com\nTo: attacker@evil.com"}},
	}
	for _, payload := range cases {
		if _, err := ch.SendEmail(context.Background(), payload); err == nil {
			t.Fatalf("expected rejection for payload to=%q cc=%v bcc=%v", payload.To, payload.CC, payload.BCC)
		}
	}
}

func TestSMTPEmailChannel_RetriesDialOnce(t *testing.T) {
	var dials int
	ch := &smtpEmailChannel{
		cfg: EmailChannelConfig{
			Host:      "smtp.example.com",
			Port:      "25",
			FromEmail: "from@example.com",
		},
		timeout: time.Second,
		breaker: resilience.NewBreaker(5, time.Minute),
		dial: func(context.Context, string, string) (net.Conn, error) {
			dials++
			return nil, errors.New("connection refused")
		},
	}

	_, err := ch.SendEmail(context.Background(), models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
	if err == nil {
		t.Fatal("expected dial error")
	}
	if dials != 2 {
		t.Fatalf("dials=%d", dials)
	}
}

func TestSMTPEmailChannel_OpenBreakerSkipsDial(t *testing.T) {
	var dials int
	ch := &smtpEmailChannel{
		cfg: EmailChannelConfig{
			Host:      "smtp.example.com",
			Port:      "25",
			FromEmail: "from@example.com",
		},
		timeout: time.Second,
		breaker: resilience.NewBreaker(1, time.Minute),
		dial: func(context.Context, string, string) (net.Conn, error) {
			dials++
			return nil, errors.New("connection refused")
		},
	}
	payload := models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"}
	if _, err := ch.SendEmail(context.Background(), payload); err == nil {
		t.Fatal("expected dial error")
	}
	if _, err := ch.SendEmail(context.Background(), payload); !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("err=%v", err)
	}
	if dials != 2 {
		t.Fatalf("dials=%d", dials)
	}
}

func TestSMTPEmailChannel_CancelClosesConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		accepted <- conn
		_, _ = conn.Read(make([]byte, 1))
	}()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	ch := NewEmailChannelFromConfig(EmailChannelConfig{
		Host:      tcpAddr.IP.String(),
		Port:      strconv.Itoa(tcpAddr.Port),
		FromEmail: "from@example.com",
		Timeout:   5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, sendErr := ch.SendEmail(ctx, models.EmailPayload{To: "a@b.com", Subject: "s", Body: "b"})
		errCh <- sendErr
	}()

	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("smtp connection was not accepted")
	}
	defer conn.Close()

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("smtp send did not return after cancel")
	}

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	if err == nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		t.Fatalf("expected server connection to close, got %v", err)
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
