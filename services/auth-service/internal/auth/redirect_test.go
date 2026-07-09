package authlocal

import (
	"testing"
)

func TestValidateRedirectURL(t *testing.T) {
	t.Setenv("FRONT_END_URL", "http://localhost:3000")
	t.Setenv("APP_URL", "http://localhost:8000")

	if err := ValidateRedirectURL("http://localhost:3000/callback"); err != nil {
		t.Fatalf("expected allowed origin: %v", err)
	}
	if err := ValidateRedirectURL("https://evil.example.com"); err == nil {
		t.Fatal("expected disallowed origin to be rejected")
	}
	if err := ValidateRedirectURL("javascript:alert(1)"); err == nil {
		t.Fatal("expected invalid scheme to be rejected")
	}
}

func TestAllowedRedirectOriginsIncludesExtra(t *testing.T) {
	t.Setenv("FRONT_END_URL", "")
	t.Setenv("APP_URL", "")
	t.Setenv("ALLOWED_REDIRECT_ORIGINS", "https://app.example.com,https://staging.example.com")

	origins := AllowedRedirectOrigins()
	if len(origins) != 2 {
		t.Fatalf("expected 2 origins, got %v", origins)
	}
}
