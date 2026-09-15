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
		"OwnerName":     "مالک",
		"DynastyName":   "خاندان نمونه",
		"RequesterName": "درخواست\u200cکننده",
		"RequesterCode": "HM-9",
	}, "مالک")
	if err != nil {
		t.Fatalf("RenderNotificationEmail: %v", err)
	}
	if !strings.Contains(html, "درخواست پیوستن جدید") {
		t.Fatalf("missing title content: %s", html)
	}
	if !strings.Contains(html, "خاندان نمونه") || !strings.Contains(html, "HM-9") {
		t.Fatalf("missing dynasty placeholders: %s", html)
	}
}
