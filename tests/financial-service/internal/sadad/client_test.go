package sadad_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"metarang/financial-service/internal/sadad"
)

func TestRequestPaymentSendsPaymentIdentityAndLocalDateTime(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != "" {
			t.Fatalf("expected empty User-Agent, got %q", ua)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ResCode": 0,
			"Token":   "test-token",
		})
	}))
	defer server.Close()

	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL,
		VerifyURL:         server.URL,
		GatewayURL:        "https://example.com/purchase",
		Multiplexed:       true,
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID:      "merchant",
		TerminalID:      "terminal",
		TransactionKey:  "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:         42,
		Amount:          1000,
		ReturnURL:       "https://example.com/callback",
		PaymentIdentity: "377012345678",
	})
	if err != nil {
		t.Fatalf("RequestPayment failed: %v", err)
	}
	if !resp.Success() {
		t.Fatalf("expected success response, got ResCode=%q", resp.ResCode)
	}

	if received["OrderId"] != float64(42) {
		t.Fatalf("expected numeric OrderId 42, got %v", received["OrderId"])
	}
	if received["PaymentIdentity"] != "377012345678" {
		t.Fatalf("expected PaymentIdentity 377012345678, got %v", received["PaymentIdentity"])
	}
	if received["MultiplexingData"] != nil {
		t.Fatalf("expected MultiplexingData to be omitted, got %v", received["MultiplexingData"])
	}

	localDateTime, _ := received["LocalDateTime"].(string)
	if localDateTime == "" {
		t.Fatal("expected LocalDateTime in request")
	}
	tehran, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Fatalf("failed to load Tehran location: %v", err)
	}
	if !strings.Contains(localDateTime, time.Now().In(tehran).Format("2006")) {
		t.Fatalf("expected current year in LocalDateTime, got %q", localDateTime)
	}
	if strings.Contains(localDateTime, "-") {
		t.Fatalf("expected Sadad date format without dashes, got %q", localDateTime)
	}
}

func TestSandboxRequestPaymentOmitsPaymentIdentity(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ResCode": 0,
			"Token":   "sandbox-token",
		})
	}))
	defer server.Close()

	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL,
		VerifyURL:         server.URL,
		GatewayURL:        sadad.SandboxEndpoints.GatewayURL,
		Multiplexed:       false,
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID:     "46645",
		TerminalID:     "GBHDTY98",
		TransactionKey: "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:        1,
		Amount:         10000,
		ReturnURL:      "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("RequestPayment failed: %v", err)
	}
	if !resp.Success() {
		t.Fatalf("expected success response, got ResCode=%q", resp.ResCode)
	}
	if received["MerchantId"] != "46645" || received["TerminalId"] != "GBHDTY98" {
		t.Fatalf("unexpected request body: %+v", received)
	}
	if received["PaymentIdentity"] != nil {
		t.Fatalf("expected PaymentIdentity to be omitted in sandbox, got %v", received["PaymentIdentity"])
	}
	if received["MultiplexingData"] != nil {
		t.Fatalf("expected MultiplexingData to be omitted in sandbox, got %v", received["MultiplexingData"])
	}
	wantURL := sadad.SandboxEndpoints.GatewayURL + "?Token=sandbox-token"
	if got := resp.URL(); got != wantURL {
		t.Fatalf("expected %q, got %q", wantURL, got)
	}
}
