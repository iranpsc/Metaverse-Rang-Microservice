package service

import (
	"testing"

	"metarang/notifications-service/internal/models"
)

func TestEnrichEmailData(t *testing.T) {
	contact := &models.UserContact{Code: "CIT-99", Name: "علی", Email: "a@x.com"}

	t.Run("fills user_code from contact when missing", func(t *testing.T) {
		got := enrichEmailData(map[string]string{"ip": "1.1.1.1"}, contact)
		if got["user_code"] != "CIT-99" {
			t.Fatalf("user_code=%q", got["user_code"])
		}
		if got["ip"] != "1.1.1.1" {
			t.Fatalf("ip=%q", got["ip"])
		}
	})

	t.Run("treats whitespace user_code as missing", func(t *testing.T) {
		got := enrichEmailData(map[string]string{"user_code": "  "}, contact)
		if got["user_code"] != "CIT-99" {
			t.Fatalf("user_code=%q", got["user_code"])
		}
	})

	t.Run("does not overwrite existing user_code", func(t *testing.T) {
		got := enrichEmailData(map[string]string{"user_code": "EXPLICIT"}, contact)
		if got["user_code"] != "EXPLICIT" {
			t.Fatalf("user_code=%q", got["user_code"])
		}
	})

	t.Run("does not overwrite existing UserCode", func(t *testing.T) {
		got := enrichEmailData(map[string]string{"UserCode": "Pascal"}, contact)
		if got["UserCode"] != "Pascal" {
			t.Fatalf("UserCode=%q", got["UserCode"])
		}
		if got["user_code"] != "" {
			t.Fatalf("unexpected user_code=%q", got["user_code"])
		}
	})

	t.Run("nil contact returns a copy", func(t *testing.T) {
		in := map[string]string{"ip": "9.9.9.9"}
		got := enrichEmailData(in, nil)
		if got["ip"] != "9.9.9.9" || got["user_code"] != "" {
			t.Fatalf("got=%v", got)
		}
		got["ip"] = "changed"
		if in["ip"] != "9.9.9.9" {
			t.Fatal("expected input map to stay unchanged")
		}
	})

	t.Run("empty contact code leaves user_code empty", func(t *testing.T) {
		got := enrichEmailData(nil, &models.UserContact{Code: "  "})
		if got["user_code"] != "" {
			t.Fatalf("user_code=%q", got["user_code"])
		}
	})
}
