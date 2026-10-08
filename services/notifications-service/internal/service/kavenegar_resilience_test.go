package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/resilience"
)

type blockingRoundTripper struct{}

func (blockingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

type statusRoundTripper struct {
	status int
	body   string
	calls  int
}

func (s *statusRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls++
	return &http.Response{
		StatusCode: s.status,
		Status:     http.StatusText(s.status),
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func TestKavenegarSendSMSHonorsTimeout(t *testing.T) {
	ch := &kavenegarSMSChannel{
		apiKey:    "key",
		sender:    "1000",
		timeout:   40 * time.Millisecond,
		transport: blockingRoundTripper{},
		breaker:   resilience.NewBreaker(5, time.Minute),
	}

	start := time.Now()
	_, err := ch.SendSMS(context.Background(), models.SMSPayload{Phone: "09120000000", Message: "hi"})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("call hung: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestKavenegarBreakerSkipsCallAfterServerError(t *testing.T) {
	transport := &statusRoundTripper{status: http.StatusBadGateway, body: "down"}
	ch := &kavenegarSMSChannel{
		apiKey:    "key",
		sender:    "1000",
		timeout:   time.Second,
		transport: transport,
		breaker:   resilience.NewBreaker(1, time.Minute),
	}
	payload := models.SMSPayload{Phone: "09120000000", Message: "hi"}

	if _, err := ch.SendSMS(context.Background(), payload); err == nil {
		t.Fatal("expected provider error")
	}
	if _, err := ch.SendSMS(context.Background(), payload); !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected open breaker, got %v", err)
	}
	if transport.calls != 1 {
		t.Fatalf("calls=%d", transport.calls)
	}
}

func TestKavenegarClientErrorDoesNotOpenBreaker(t *testing.T) {
	transport := &statusRoundTripper{
		status: http.StatusForbidden,
		body:   `{"return":{"status":403,"message":"forbidden"}}`,
	}
	ch := &kavenegarSMSChannel{
		apiKey:    "key",
		sender:    "1000",
		timeout:   time.Second,
		transport: transport,
		breaker:   resilience.NewBreaker(1, time.Minute),
	}
	payload := models.SMSPayload{Phone: "09120000000", Message: "hi"}

	for i := 0; i < 2; i++ {
		_, err := ch.SendSMS(context.Background(), payload)
		if err == nil || errors.Is(err, resilience.ErrCircuitOpen) {
			t.Fatalf("call %d err=%v", i, err)
		}
	}
	if transport.calls != 2 {
		t.Fatalf("calls=%d", transport.calls)
	}
}
