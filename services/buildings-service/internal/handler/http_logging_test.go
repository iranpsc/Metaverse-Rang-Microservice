package handler

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestRequestIDUsesGatewayHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{name: "client request id", header: "X-Request-Id", value: "client-id", want: "client-id"},
		{name: "kong request id", header: "Kong-Request-ID", value: "kong-id", want: "kong-id"},
		{name: "kong forwarded request id", header: "X-Kong-Request-ID", value: "forwarded-id", want: "forwarded-id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/features/1/build/1", nil)
			req.Header.Set(tc.header, "  "+tc.value+"  ")
			if got := requestID(req); got != tc.want {
				t.Fatalf("requestID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildFeatureErrorLogValueRedactsSatisfactionAmounts(t *testing.T) {
	err := errors.New("rpc error: code = FailedPrecondition desc = insufficient satisfaction: required 20.000000, available 1.000000")
	if got := buildFeatureErrorLogValue(err); got != "insufficient satisfaction" {
		t.Fatalf("buildFeatureErrorLogValue() = %v, want redacted message", got)
	}
}
