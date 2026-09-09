package helpers

import (
	"errors"
	"net/mail"
	"strings"
)

// ErrInvalidEmailAddress is returned when an email address is malformed or unsafe for SMTP headers.
var ErrInvalidEmailAddress = errors.New("invalid email address")

// ParseEmailAddress validates a single RFC5322 address and returns the normalized addr-spec.
// CR/LF and other header-injection characters are rejected before parsing.
func ParseEmailAddress(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidEmailAddress
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return "", ErrInvalidEmailAddress
	}

	parsed, err := mail.ParseAddress(raw)
	if err != nil {
		return "", ErrInvalidEmailAddress
	}
	if strings.ContainsAny(parsed.Address, "\r\n\x00") {
		return "", ErrInvalidEmailAddress
	}
	return parsed.Address, nil
}
