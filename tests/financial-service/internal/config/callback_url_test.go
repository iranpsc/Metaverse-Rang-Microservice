package config_test

import (
	"os"
	"testing"

	"metarang/financial-service/internal/config"
)

func TestResolveSadadCallbackURL(t *testing.T) {
	t.Setenv("SADAD_CALLBACK_URL", "")
	t.Setenv("PAYMENT_CALLBACK_URL", "")
	t.Setenv("SADAD_CALLBACK_PORT", "")
	t.Setenv("PROJECT_URL", "http://localhost:8000")

	if got := config.ResolveSadadCallbackURL(); got != "http://localhost:8000/api/order/callback" {
		t.Fatalf("expected default callback URL, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "${PROJECT_URL}/api/order/callback")
	if got := config.ResolveSadadCallbackURL(); got != "http://localhost:8000/api/order/callback" {
		t.Fatalf("expected expanded callback URL, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "${FRONTEND_URL}/payment/verify")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")
	if got := config.ResolveSadadCallbackURL(); got != "http://localhost:8000/api/order/callback" {
		t.Fatalf("expected fallback when callback URL points to frontend verify page, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "https://api.example.com/api/order/callback")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com/api/order/callback" {
		t.Fatalf("expected explicit callback URL, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "https://api.example.com/api/payment/callback")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com/api/order/callback" {
		t.Fatalf("expected legacy payment callback URL to normalize to order callback, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "${PROJECT_URL}/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "8080")
	if got := config.ResolveSadadCallbackURL(); got != "http://localhost:8080/api/order/callback" {
		t.Fatalf("expected callback URL with SADAD_CALLBACK_PORT applied for http, got %q", got)
	}

	// Production-safe: do not rewrite https ReturnUrls with SADAD_CALLBACK_PORT=8080.
	t.Setenv("SADAD_CALLBACK_URL", "https://api.example.com/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "8080")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com/api/order/callback" {
		t.Fatalf("expected https ReturnUrl to ignore SADAD_CALLBACK_PORT=8080, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "https://api.example.com:8443/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "8080")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com:8443/api/order/callback" {
		t.Fatalf("expected https URL to keep original port when SADAD_CALLBACK_PORT=8080 is ignored, got %q", got)
	}

	t.Setenv("SADAD_CALLBACK_URL", "http://localhost:8000/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "8065")
	if got := config.ResolveSadadCallbackURL(); got != "http://localhost:8065/api/order/callback" {
		t.Fatalf("expected local http callback port override, got %q", got)
	}
}

func TestNormalizePaymentCallbackURL(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{
			name:   "valid callback url",
			raw:    "http://localhost:8000/api/order/callback",
			want:   "http://localhost:8000/api/order/callback",
			wantOK: true,
		},
		{
			name:   "legacy payment callback url normalized",
			raw:    "http://localhost:8000/api/payment/callback",
			want:   "http://localhost:8000/api/order/callback",
			wantOK: true,
		},
		{
			name:   "base url without path",
			raw:    "http://localhost:8000",
			want:   "http://localhost:8000/api/order/callback",
			wantOK: true,
		},
		{
			name:   "frontend verify page rejected",
			raw:    "http://localhost:5173/payment/verify",
			wantOK: false,
		},
		{
			name:   "unrelated path rejected",
			raw:    "http://localhost:8000/api/order",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := config.NormalizePaymentCallbackURL(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if ok && got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveProjectURL(t *testing.T) {
	t.Setenv("PROJECT_URL", "https://project.example.com/")

	if got := config.ResolveProjectURL(); got != "https://project.example.com" {
		t.Fatalf("expected normalized project URL, got %q", got)
	}
}

func TestResolveFrontendURL(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://frontend.example.com/app/")

	if got := config.ResolveFrontendURL(); got != "https://frontend.example.com/app" {
		t.Fatalf("expected normalized frontend URL, got %q", got)
	}

	t.Setenv("FRONTEND_URL", "")
	if got := config.ResolveFrontendURL(); got != "" {
		t.Fatalf("expected empty frontend URL, got %q", got)
	}

	t.Setenv("FRONTEND_URL", "frontend.local")
	if got := config.ResolveFrontendURL(); got != "http://frontend.local" {
		t.Fatalf("expected scheme prefix, got %q", got)
	}
}

func TestResolveSadadCallbackURL_InvalidPortAndPaymentFallback(t *testing.T) {
	t.Setenv("SADAD_CALLBACK_URL", "")
	t.Setenv("PAYMENT_CALLBACK_URL", "https://api.example.com/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "not-a-port")
	t.Setenv("PROJECT_URL", "http://localhost:8000")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com/api/order/callback" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("PAYMENT_CALLBACK_URL", "")
	t.Setenv("SADAD_CALLBACK_URL", "https://api.example.com/api/order/callback")
	t.Setenv("SADAD_CALLBACK_PORT", "0")
	if got := config.ResolveSadadCallbackURL(); got != "https://api.example.com/api/order/callback" {
		t.Fatalf("invalid port should leave url unchanged, got %q", got)
	}
}

func TestResolveProjectURL_DefaultAndScheme(t *testing.T) {
	t.Setenv("PROJECT_URL", "")
	if got := config.ResolveProjectURL(); got != "http://localhost:8000" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("PROJECT_URL", "project.local")
	if got := config.ResolveProjectURL(); got != "http://project.local" {
		t.Fatalf("got %q", got)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
