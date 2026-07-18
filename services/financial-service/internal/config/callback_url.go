// Package config provides configuration utilities for the financial service.
package config

import (
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	OrderCallbackPath         = "/api/order/callback"
	legacyPaymentCallbackPath = "/api/payment/callback"
	defaultProjectURL         = "http://localhost:8000"
)

// ResolveSadadCallbackURL returns the Sadad ReturnUrl base (without order_id query param).
// The gateway must redirect users to the API callback endpoint, never the frontend verify page.
// Supports ${PROJECT_URL} expansion in config.env (e.g. SADAD_CALLBACK_URL=${PROJECT_URL}/api/order/callback).
// When SADAD_CALLBACK_PORT is set, its value replaces the port on the resolved callback URL host.
func ResolveSadadCallbackURL() string {
	for _, key := range []string{"SADAD_CALLBACK_URL", "PAYMENT_CALLBACK_URL"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			if expanded := strings.TrimSpace(os.ExpandEnv(raw)); expanded != "" {
				if normalized, ok := NormalizePaymentCallbackURL(expanded); ok {
					return applySadadCallbackPort(normalized)
				}
				log.Printf("Warning: %s=%q is not a valid API callback URL; falling back to PROJECT_URL", key, expanded)
			}
		}
	}

	projectURL := strings.TrimSpace(os.ExpandEnv(getEnv("PROJECT_URL", defaultProjectURL)))
	return applySadadCallbackPort(strings.TrimSuffix(projectURL, "/") + OrderCallbackPath)
}

func applySadadCallbackPort(rawURL string) string {
	portStr := strings.TrimSpace(os.Getenv("SADAD_CALLBACK_PORT"))
	if portStr == "" {
		return rawURL
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		log.Printf("Warning: invalid SADAD_CALLBACK_PORT=%q; callback URL port unchanged", portStr)
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return rawURL
	}

	parsed.Host = net.JoinHostPort(hostname, strconv.Itoa(port))
	return strings.TrimSuffix(parsed.String(), "/")
}

// NormalizePaymentCallbackURL validates and normalizes Sadad callback URLs.
func NormalizePaymentCallbackURL(raw string) (string, bool) {
	if strings.Contains(raw, "/payment/verify") {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	path := strings.TrimSuffix(parsed.Path, "/")
	switch path {
	case OrderCallbackPath:
		return strings.TrimSuffix(raw, "/"), true
	case legacyPaymentCallbackPath:
		parsed.Path = OrderCallbackPath
		return strings.TrimSuffix(parsed.String(), "/"), true
	}

	if path == "" || path == "/" {
		parsed.Path = OrderCallbackPath
		return strings.TrimSuffix(parsed.String(), "/"), true
	}

	return "", false
}

// ResolveProjectURL returns the normalized project base URL from PROJECT_URL.
func ResolveProjectURL() string {
	raw := strings.TrimSpace(getEnv("PROJECT_URL", defaultProjectURL))
	expanded := strings.TrimSpace(os.ExpandEnv(raw))
	return strings.TrimSuffix(normalizeURLScheme(expanded), "/")
}

// ResolveFrontendURL returns the normalized frontend base URL from FRONTEND_URL.
func ResolveFrontendURL() string {
	raw := strings.TrimSpace(os.Getenv("FRONTEND_URL"))
	if raw == "" {
		return ""
	}
	expanded := strings.TrimSpace(os.ExpandEnv(raw))
	return strings.TrimSuffix(normalizeURLScheme(expanded), "/")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func normalizeURLScheme(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	if strings.Contains(rawURL, "://") {
		return rawURL
	}
	return "http://" + rawURL
}
