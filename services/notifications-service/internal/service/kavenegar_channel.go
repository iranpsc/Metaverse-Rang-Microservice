package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/resilience"

	"github.com/kavenegar/kavenegar-go"
)

const (
	kavenegarOTPTemplate = "verify"
	defaultSMSTimeout    = 8 * time.Second
	kavenegarDialTimeout = 5 * time.Second
)

var errEmptyKavenegarResponse = errors.New("no response entries from Kavenegar")

type kavenegarSMSChannel struct {
	apiKey    string
	sender    string
	timeout   time.Duration
	transport http.RoundTripper
	breaker   *resilience.Breaker
}

// NewKavenegarSMSChannel creates a new Kavenegar SMS channel implementation.
func NewKavenegarSMSChannel(apiKey, sender string) SMSChannel {
	return newKavenegarSMSChannel(apiKey, sender, defaultSMSTimeout)
}

func newKavenegarSMSChannel(apiKey, sender string, timeout time.Duration) SMSChannel {
	if apiKey == "" {
		log.Println("Warning: Kavenegar API key is empty, SMS sending will fail")
		return &noopSMSChannel{}
	}
	if timeout <= 0 {
		timeout = defaultSMSTimeout
	}
	return &kavenegarSMSChannel{
		apiKey:    apiKey,
		sender:    sender,
		timeout:   timeout,
		transport: newKavenegarTransport(timeout),
		breaker:   resilience.NewBreaker(resilience.DefaultFailureThreshold, resilience.DefaultOpenCooldown),
	}
}

func newKavenegarTransport(timeout time.Duration) http.RoundTripper {
	dialTimeout := kavenegarDialTimeout
	if timeout > 0 && timeout < dialTimeout {
		dialTimeout = timeout
	}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   dialTimeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
	}
}

type contextRoundTripper struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	ctx := t.ctx
	if ctx == nil {
		ctx = req.Context()
	}
	return base.RoundTrip(req.WithContext(ctx))
}

func (c *kavenegarSMSChannel) client(ctx context.Context) *kavenegar.Kavenegar {
	kc := kavenegar.NewClient(c.apiKey)
	kc.BaseClient = &http.Client{Transport: contextRoundTripper{ctx: ctx, base: c.transport}}
	return kavenegar.NewWithClient(kc)
}

func (c *kavenegarSMSChannel) call(ctx context.Context, fn func(api *kavenegar.Kavenegar) error) error {
	if c.breaker == nil {
		c.breaker = resilience.NewBreaker(resilience.DefaultFailureThreshold, resilience.DefaultOpenCooldown)
	}
	timeout := c.timeout
	if timeout <= 0 {
		timeout = defaultSMSTimeout
	}
	return c.breaker.Execute(func() error {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return fn(c.client(callCtx))
	}, kavenegarFailure)
}

func (c *kavenegarSMSChannel) SendSMS(ctx context.Context, payload models.SMSPayload) (string, error) {
	if payload.Phone == "" {
		return "", fmt.Errorf("phone number is required")
	}

	if payload.Template == "" && payload.Message == "" {
		return "", fmt.Errorf("message is required when template is not provided")
	}

	var messageID string
	err := c.call(ctx, func(api *kavenegar.Kavenegar) error {
		if payload.Template != "" {
			token := extractTemplateToken(payload.Tokens)
			res, err := api.Verify.Lookup(payload.Phone, payload.Template, token, buildVerifyLookupParam(payload.Tokens))
			if err != nil {
				return err
			}
			messageID = fmt.Sprintf("%d", res.MessageID)
			return nil
		}

		res, err := api.Message.Send(c.sender, []string{payload.Phone}, payload.Message, nil)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return errEmptyKavenegarResponse
		}
		messageID = fmt.Sprintf("%d", res[0].MessageID)
		return nil
	})
	if err != nil {
		return "", normalizeKavenegarErr(err)
	}
	return messageID, nil
}

func (c *kavenegarSMSChannel) SendOTP(ctx context.Context, payload models.OTPPayload) (string, error) {
	if payload.Phone == "" {
		return "", fmt.Errorf("phone number is required")
	}
	if payload.Code == "" {
		return "", fmt.Errorf("OTP code is required")
	}

	var messageID string
	err := c.call(ctx, func(api *kavenegar.Kavenegar) error {
		res, err := api.Verify.Lookup(payload.Phone, kavenegarOTPTemplate, payload.Code, nil)
		if err != nil {
			return err
		}
		messageID = fmt.Sprintf("%d", res.MessageID)
		return nil
	})
	if err != nil {
		return "", normalizeKavenegarErr(err)
	}
	return messageID, nil
}

func normalizeKavenegarErr(err error) error {
	if errors.Is(err, resilience.ErrCircuitOpen) || errors.Is(err, errEmptyKavenegarResponse) {
		return err
	}
	if kavenegarDeadline(err) {
		return context.DeadlineExceeded
	}
	return mapKavenegarError(err)
}

func kavenegarDeadline(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var httpErr *kavenegar.HTTPError
	if errors.As(err, &httpErr) && httpErr.Err != nil && (errors.Is(httpErr.Err, context.DeadlineExceeded) || isNetTimeout(httpErr.Err)) {
		return true
	}
	return isNetTimeout(err)
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func kavenegarFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, resilience.ErrCircuitOpen) {
		return false
	}
	var apiErr *kavenegar.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500
	}
	var httpErr *kavenegar.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status >= 500 || httpErr.Status == 0
	}
	if errors.Is(err, errEmptyKavenegarResponse) {
		return false
	}
	return true
}

func extractTemplateToken(tokens map[string]string) string {
	if tokens == nil {
		return ""
	}
	if val, ok := tokens["token"]; ok && val != "" {
		return val
	}
	if val, ok := tokens["code"]; ok && val != "" {
		return val
	}
	return ""
}

// buildVerifyLookupParam maps Laravel verifyLookup tokens onto the Kavenegar SDK param.
// Empty extra tokens return nil so the SDK does not send blank Token2/Token3/Type fields.
func buildVerifyLookupParam(tokens map[string]string) *kavenegar.VerifyLookupParam {
	if tokens == nil {
		return nil
	}
	params := &kavenegar.VerifyLookupParam{}
	has := false
	if v := strings.TrimSpace(tokens["token2"]); v != "" {
		params.Token2 = v
		has = true
	}
	if v := strings.TrimSpace(tokens["token3"]); v != "" {
		params.Token3 = v
		has = true
	}
	extra := map[string]string{}
	for _, key := range []string{"token10", "token20"} {
		if v := strings.TrimSpace(tokens[key]); v != "" {
			extra[key] = v
			has = true
		}
	}
	if len(extra) > 0 {
		params.Tokens = extra
	}
	if !has {
		return nil
	}
	return params
}

func mapKavenegarError(err error) error {
	switch e := err.(type) {
	case *kavenegar.APIError:
		if hint := kavenegarAPIErrorHint(e.Status); hint != "" {
			return fmt.Errorf("kavenegar API error: %w (%s)", e, hint)
		}
		return fmt.Errorf("kavenegar API error: %w", e)
	case *kavenegar.HTTPError:
		return fmt.Errorf("kavenegar HTTP error: %w", e)
	default:
		return fmt.Errorf("kavenegar request failed: %w", err)
	}
}

func kavenegarAPIErrorHint(status int) string {
	switch status {
	case 403:
		return "invalid API key; set SMS_API_KEY in notifications-service/config.env"
	case 416:
		return "request IP is not allowed in Kavenegar panel; add your server/Docker egress IP to API access list or disable IP restriction"
	case 424:
		return "verification template not found or not approved; ensure template \"verify\" exists in Kavenegar console"
	default:
		return ""
	}
}
