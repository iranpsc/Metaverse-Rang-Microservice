package authlocal

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ValidateRedirectURL ensures redirect targets belong to an allowlisted origin.
func ValidateRedirectURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid redirect URL")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid redirect scheme")
	}

	origin := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
	for _, allowed := range AllowedRedirectOrigins() {
		if origin == strings.ToLower(allowed) {
			return nil
		}
	}

	return fmt.Errorf("redirect URL not allowed")
}

// AllowedRedirectOrigins returns trusted frontend origins for OAuth redirects.
func AllowedRedirectOrigins() []string {
	seen := make(map[string]struct{})
	origins := make([]string, 0, 4)

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if _, ok := seen[origin]; ok {
			return
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	add(os.Getenv("FRONT_END_URL"))
	add(os.Getenv("APP_URL"))
	add(os.Getenv("API_GATEWAY_URL"))

	if extra := os.Getenv("ALLOWED_REDIRECT_ORIGINS"); extra != "" {
		for _, part := range strings.Split(extra, ",") {
			add(strings.TrimSpace(part))
		}
	}

	return origins
}
