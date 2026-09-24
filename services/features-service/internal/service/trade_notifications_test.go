package service

import "testing"

func TestNotificationJSONFlag(t *testing.T) {
	raw := `{"trades_sms":true,"trades_email":0}`
	if !notificationJSONFlag(raw, "trades_sms", false) {
		t.Fatal("bool true")
	}
	if notificationJSONFlag(raw, "trades_email", true) {
		t.Fatal("numeric 0 should be false")
	}
	if !notificationJSONFlag(raw, "missing", true) {
		t.Fatal("missing uses default")
	}
	if notificationJSONFlag("{", "trades_sms", false) {
		t.Fatal("invalid json uses default")
	}
	if !notificationJSONFlag(`{"trades_sms":"true"}`, "trades_sms", false) {
		t.Fatal("string true")
	}
	if !notificationJSONFlag(`{"trades_sms":1}`, "trades_sms", false) {
		t.Fatal("numeric 1")
	}
	if notificationJSONFlag(`{"trades_sms":"no"}`, "trades_sms", true) {
		t.Fatal("unknown string uses parsed false")
	}
}
