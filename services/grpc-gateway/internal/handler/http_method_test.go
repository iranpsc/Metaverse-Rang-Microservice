package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEffectiveHTTPMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		query    string
		body     string
		ctype    string
		expected string
	}{
		{
			name:     "PUT passes through",
			method:   http.MethodPut,
			expected: http.MethodPut,
		},
		{
			name:     "POST without spoofing",
			method:   http.MethodPost,
			expected: http.MethodPost,
		},
		{
			name:     "POST with query _method put",
			method:   http.MethodPost,
			query:    "_method=put",
			expected: http.MethodPut,
		},
		{
			name:     "POST with form _method put",
			method:   http.MethodPost,
			body:     "_method=put&announcements_sms=1",
			ctype:    "application/x-www-form-urlencoded",
			expected: http.MethodPut,
		},
		{
			name:     "POST with multipart _method put",
			method:   http.MethodPost,
			body:     "--boundary\r\nContent-Disposition: form-data; name=\"_method\"\r\n\r\nput\r\n--boundary--\r\n",
			ctype:    "multipart/form-data; boundary=boundary",
			expected: http.MethodPut,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/api/general-settings/1"
			if tt.query != "" {
				target += "?" + tt.query
			}

			var body io.Reader = http.NoBody
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, target, body)
			if tt.ctype != "" {
				req.Header.Set("Content-Type", tt.ctype)
			}

			if got := EffectiveHTTPMethod(req); got != tt.expected {
				t.Fatalf("EffectiveHTTPMethod() = %q, want %q", got, tt.expected)
			}
		})
	}
}
