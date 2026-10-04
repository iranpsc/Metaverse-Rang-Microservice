package service

import (
	"context"
	"log"
	"time"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
)

type noopSMSChannel struct{}

// SMSChannelConfig holds SMS provider configuration (read from config.env: SMS_PROVIDER, SMS_API_KEY, SMS_SENDER).
type SMSChannelConfig struct {
	Provider string // e.g. "kavenegar"
	APIKey   string
	Sender   string
	Timeout  time.Duration
}

// NewSMSChannel creates an SMS channel from the given config (e.g. from main after loading config.env).
// Supported providers: "kavenegar" (defaults to noop if not configured or provider not supported).
func NewSMSChannel(cfg SMSChannelConfig) SMSChannel {
	provider := cfg.Provider
	apiKey := cfg.APIKey
	sender := cfg.Sender

	log.Printf("SMS Channel initialization: provider=%s, apiKey set=%v, sender=%s", provider, apiKey != "", sender)

	switch provider {
	case "kavenegar":
		if apiKey == "" {
			log.Println("Warning: SMS_PROVIDER is 'kavenegar' but SMS_API_KEY is not set, using noop channel")
			return &noopSMSChannel{}
		}
		if sender == "" {
			sender = "10008663"
		}
		log.Printf("Initializing Kavenegar SMS channel with sender: %s", sender)
		return newKavenegarSMSChannel(apiKey, sender, cfg.Timeout)
	default:
		if provider == "" {
			log.Println("Warning: SMS_PROVIDER is not set, using noop channel")
		} else {
			log.Printf("Warning: Unknown SMS_PROVIDER '%s', using noop channel", provider)
		}
		return &noopSMSChannel{}
	}
}

func (c *noopSMSChannel) SendSMS(ctx context.Context, payload models.SMSPayload) (string, error) {
	return "", errs.ErrNotImplemented
}

func (c *noopSMSChannel) SendOTP(ctx context.Context, payload models.OTPPayload) (string, error) {
	return "", errs.ErrNotImplemented
}
