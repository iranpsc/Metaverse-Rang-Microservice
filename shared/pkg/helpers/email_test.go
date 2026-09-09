package helpers

import (
	"errors"
	"testing"
)

func TestParseEmailAddress(t *testing.T) {
	t.Run("accepts valid addresses", func(t *testing.T) {
		cases := map[string]string{
			"user@example.com":                 "user@example.com",
			" User <user@example.com> ":       "user@example.com",
			"display@example.com":              "display@example.com",
			"first.last+tag@mail.example.co.uk": "first.last+tag@mail.example.co.uk",
		}
		for input, want := range cases {
			got, err := ParseEmailAddress(input)
			if err != nil {
				t.Fatalf("ParseEmailAddress(%q): %v", input, err)
			}
			if got != want {
				t.Fatalf("ParseEmailAddress(%q)=%q, want %q", input, got, want)
			}
		}
	})

	t.Run("rejects header injection and malformed values", func(t *testing.T) {
		cases := []string{
			"",
			"   ",
			"not-an-email",
			"user@example.com\nCc: attacker@evil.com",
			"user@example.com\r\nBcc: attacker@evil.com",
			"user@example.com\rCc: attacker@evil.com",
			"user@example.com,attacker@evil.com",
		}
		for _, input := range cases {
			_, err := ParseEmailAddress(input)
			if !errors.Is(err, ErrInvalidEmailAddress) {
				t.Fatalf("ParseEmailAddress(%q) err=%v, want ErrInvalidEmailAddress", input, err)
			}
		}
	})
}
