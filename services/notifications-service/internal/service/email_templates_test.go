package service

import (
	"strings"
	"testing"
)

func TestResolveEmailTemplate(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		data map[string]string
		want string
	}{
		{"feature purchase", "BuyFeatureNotification", nil, "email/feature_purchase"},
		{"buy request sent", "BuyRequestNotification", map[string]string{"type": "buyer"}, "email/buy_request_sent"},
		{"buy request received", "BuyRequestNotification", map[string]string{"type": "seller"}, "email/buy_request_received"},
		{"sell feature", "sellFeature", nil, "email/sell_feature"},
		{"sell request", "SellRequestNotification", nil, "email/sell_request"},
		{"dynasty sent", "dynasty_join_request", map[string]string{"side": "sent"}, "email/dynasty/join_request_sent"},
		{"dynasty received", "dynasty_join_request", map[string]string{"side": "received"}, "email/dynasty/join_request_received"},
		{"dynasty accept", "dynasty_join_request_accept", nil, "email/dynasty/join_request_accepted"},
		{"dynasty reject", "dynasty_join_request_reject", nil, "email/dynasty/join_request_rejected"},
		{"login alert", "login", nil, "email/login_alert"},
		{"unknown", "FeatureHourlyProfitDeposit", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveEmailTemplate(tt.typ, tt.data); got != tt.want {
				t.Fatalf("ResolveEmailTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderNotificationEmail_FeaturePurchase(t *testing.T) {
	html, err := RenderNotificationEmail("BuyFeatureNotification", "خریداری ملک", map[string]string{
		"RecipientName":      "علی",
		"OwnerCode":          "HM-1",
		"FeatureID":          "VOD-100",
		"FeatureArea":        "120.50",
		"FeatureApplication": "مسکونی",
		"FeatureDensity":     "3",
		"SellerCode":         "HM-2",
		"PriceIRR":           "1000000",
		"PricePSC":           "10",
		"PriceIRRLabel":      "IRR",
		"PricePSCLabel":      "PSC",
	}, "علی")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	if html == "" {
		t.Fatal("expected rendered HTML")
	}
	for _, want := range []string{"ثبت موفق فضای VOD", "علی", "HM-1", "VOD-100", "مسکونی", "1000000", "10", "IRR", "PSC"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q", want)
		}
	}
}

func TestRenderNotificationEmail_FeaturePurchaseRGBColorAsset(t *testing.T) {
	html, err := RenderNotificationEmail("BuyFeatureNotification", "خریداری ملک", map[string]string{
		"RecipientName":   "علی",
		"OwnerCode":       "HM-1",
		"FeatureID":       "VOD-200",
		"PaidAssetLabel":  "رنگ قرمز",
		"PaidAssetAmount": "1000",
		"purchase_type":   "rgb",
	}, "علی")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	for _, want := range []string{"رنگ قرمز", "1000", "VOD-200"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q", want)
		}
	}
	if strings.Contains(html, "مبلغ پرداختی (PSC)") || strings.Contains(html, "مبلغ پرداختی (IRR)") {
		t.Fatalf("RGB purchase email must not show PSC/IRR placeholders: %s", html)
	}
}

func TestRenderNotificationEmail_DynastyJoinReceived(t *testing.T) {
	html, err := RenderNotificationEmail("dynasty_join_request", "درخواست پیوستن جدید", map[string]string{
		"side":          "received",
		"RecipientName": "گیرنده",
		"OwnerName":     "گیرنده",
		"DynastyName":   "خاندان نمونه",
		"RequesterName": "درخواست\u200cکننده",
		"RequesterCode": "HM-9",
		"Role":          "برادر",
	}, "گیرنده")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	if !strings.Contains(html, "درخواست پیوستن جدید") {
		t.Fatalf("missing title content: %s", html)
	}
	for _, want := range []string{"گیرنده", "خاندان نمونه", "HM-9", "درخواست\u200cکننده", "برادر"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "سلام درخواست") {
		t.Fatalf("received email must greet the recipient, not the requester: %s", html)
	}
}

func TestRenderNotificationEmail_DynastyJoinSent(t *testing.T) {
	html, err := RenderNotificationEmail("dynasty_join_request", "درخواست پیوستن ارسال شد", map[string]string{
		"side":          "sent",
		"RecipientName": "فرستنده",
		"RequesterName": "فرستنده",
		"ReceiverName":  "گیرنده",
		"ReceiverCode":  "R-20",
		"DynastyName":   "خاندان فرستنده",
		"Role":          "خواهر",
	}, "فرستنده")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	for _, want := range []string{"فرستنده", "گیرنده", "R-20", "خاندان فرستنده", "خواهر"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q: %s", want, html)
		}
	}
}

func TestRenderNotificationEmail_DynastyJoinAcceptedSides(t *testing.T) {
	requesterHTML, err := RenderNotificationEmail("dynasty_join_request_accept", "پیوستن به خاندان تأیید شد", map[string]string{
		"side":          "requester",
		"RecipientName": "درخواست\u200cکننده",
		"DynastyName":   "خاندان A",
		"AcceptedBy":    "پذیرنده",
	}, "درخواست\u200cکننده")
	if err != nil {
		t.Fatalf("requester render: %v", err)
	}
	if !strings.Contains(requesterHTML, "پذیرنده") || !strings.Contains(requesterHTML, "تأیید توسط") {
		t.Fatalf("requester accept email missing AcceptedBy label: %s", requesterHTML)
	}

	receiverHTML, err := RenderNotificationEmail("dynasty_join_request_accept", "پیوستن به خاندان تأیید شد", map[string]string{
		"side":          "receiver",
		"RecipientName": "پذیرنده",
		"DynastyName":   "خاندان A",
		"AcceptedBy":    "درخواست\u200cکننده",
	}, "پذیرنده")
	if err != nil {
		t.Fatalf("receiver render: %v", err)
	}
	if !strings.Contains(receiverHTML, "پیوستن تو به خاندان") {
		t.Fatalf("receiver accept email should use receiver wording: %s", receiverHTML)
	}
	if !strings.Contains(receiverHTML, "سرپرست خاندان") {
		t.Fatalf("receiver accept email missing dynasty owner label: %s", receiverHTML)
	}
}

func TestRenderNotificationEmail_LoginAlert(t *testing.T) {
	html, err := RenderNotificationEmail("login", "ورود به حساب کاربری", map[string]string{
		"user_code":  "HM-77",
		"login_date": "1404/06/26",
		"login_time": "12:30:00",
		"ip":         "203.0.113.10",
	}, "علی")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	if html == "" {
		t.Fatal("expected rendered HTML")
	}
	for _, want := range []string{"ورود موفق به متارنگ", "علی", "HM-77", "1404/06/26", "12:30:00", "203.0.113.10"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q", want)
		}
	}
}
