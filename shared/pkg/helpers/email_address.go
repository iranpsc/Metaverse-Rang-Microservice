package helpers

import (
	"errors"
	"net/mail"
	"strings"
)

// ErrInvalidEmailAddress is returned when an email address is malformed or unsafe for SMTP headers.
var ErrInvalidEmailAddress = errors.New("invalid email address")

// ParseEmailAddress validates a single RFC5322 mailbox and returns the normalized address.
func ParseEmailAddress(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidEmailAddress
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", ErrInvalidEmailAddress
	}

	parsed, err := mail.ParseAddress(raw)
	if err != nil {
		return "", ErrInvalidEmailAddress
	}

	address := strings.TrimSpace(parsed.Address)
	if address == "" || strings.ContainsAny(address, "\r\n") {
		return "", ErrInvalidEmailAddress
	}

	return address, nil
}

// ParseEmailAddresses validates a list of mailbox strings and returns normalized addresses.
func ParseEmailAddresses(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		address, err := ParseEmailAddress(item)
		if err != nil {
			return nil, err
		}
		out = append(out, address)
	}
	return out, nil
}
