package helpers

import (
	"errors"
	"testing"
)

func TestParseEmailAddress(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "simple", raw: "user@example.com", want: "user@example.com"},
		{name: "display name", raw: "User Name <user@example.com>", want: "user@example.com"},
		{name: "trimmed", raw: "  user@example.com  ", want: "user@example.com"},
		{name: "empty", raw: "", wantErr: true},
		{name: "crlf injection", raw: "user@example.com\r\nBcc: evil@example.com", wantErr: true},
		{name: "lf injection", raw: "user@example.com\nBcc: evil@example.com", wantErr: true},
		{name: "invalid", raw: "not-an-email", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEmailAddress(tc.raw)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidEmailAddress) {
					t.Fatalf("expected ErrInvalidEmailAddress, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestParseEmailAddresses(t *testing.T) {
	addrs, err := ParseEmailAddresses([]string{" cc@example.com ", "", "bcc@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 || addrs[0] != "cc@example.com" || addrs[1] != "bcc@example.com" {
		t.Fatalf("got %v", addrs)
	}

	if _, err := ParseEmailAddresses([]string{"good@example.com", "bad\r\nCc: x@y.com"}); !errors.Is(err, ErrInvalidEmailAddress) {
		t.Fatalf("expected invalid address error, got %v", err)
	}
}
