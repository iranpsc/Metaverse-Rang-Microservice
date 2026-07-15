package sadad_test

import (
	"testing"

	"metarang/financial-service/internal/sadad"
)

func TestSandboxEndpointsMatchBankTestURLs(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "payment request",
			got:      sadad.SandboxEndpoints.PaymentRequestURL,
			expected: "https://sandbox.banktest.ir/melli/sadad.shaparak.ir/VPG/api/v0/Request/PaymentRequest",
		},
		{
			name:     "verify",
			got:      sadad.SandboxEndpoints.VerifyURL,
			expected: "https://sandbox.banktest.ir/melli/sadad.shaparak.ir/VPG/api/v0/Advice/Verify",
		},
		{
			name:     "purchase gateway",
			got:      sadad.SandboxEndpoints.GatewayURL,
			expected: "https://sandbox.banktest.ir/melli/sadad.shaparak.ir/VPG/Purchase",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, tc.got)
			}
		})
	}

	if sadad.SandboxEndpoints.Multiplexed {
		t.Fatal("sandbox endpoints must not send MultiplexingData")
	}
}

func TestProductionEndpointsUseMultiplexing(t *testing.T) {
	if sadad.ProductionEndpoints.PaymentRequestURL != "https://sadad.shaparak.ir/VPG/api/v0/Request/PaymentRequest" {
		t.Fatalf("unexpected production payment request URL: %q", sadad.ProductionEndpoints.PaymentRequestURL)
	}
	if !sadad.ProductionEndpoints.Multiplexed {
		t.Fatal("production endpoints must send MultiplexingData")
	}
}
