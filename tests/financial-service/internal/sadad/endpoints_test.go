package sadad_test

import (
	"testing"

	"metarang/financial-service/internal/sadad"
)

func TestProductionEndpointsMatchOfficialVPGHelp(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "payment request",
			got:      sadad.ProductionEndpoints.PaymentRequestURL,
			expected: "https://sadad.shaparak.ir/api/v0/Request/PaymentRequest",
		},
		{
			name:     "verify",
			got:      sadad.ProductionEndpoints.VerifyURL,
			expected: "https://sadad.shaparak.ir/api/v0/Advice/Verify",
		},
		{
			name:     "purchase gateway",
			got:      sadad.ProductionEndpoints.GatewayURL,
			expected: "https://sadad.shaparak.ir/Purchase",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, tc.got)
			}
		})
	}
}
