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

func TestRequestPaymentSendsMultiplexingDataAndLocalDateTime(t *testing.T) {
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

	multiplexingData, err := sadad.MultiplexingDataForAmount("123", 1000)
	if err != nil {
		t.Fatalf("MultiplexingDataForAmount failed: %v", err)
	}

	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL,
		VerifyURL:         server.URL,
		GatewayURL:        "https://example.com/purchase",
		Multiplexed:       true,
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID:       "merchant",
		TerminalID:       "terminal",
		TransactionKey:   "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:          42,
		Amount:           1000,
		ReturnURL:        "https://example.com/callback",
		MultiplexingData: multiplexingData,
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

	multiplexingPayload, ok := received["MultiplexingData"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected MultiplexingData in request, got %v", received["MultiplexingData"])
	}
	if multiplexingPayload["Type"] != "Amount" {
		t.Fatalf("expected Type Amount, got %v", multiplexingPayload["Type"])
	}
	rows, ok := multiplexingPayload["MultiplexingRows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one MultiplexingRows entry, got %v", multiplexingPayload["MultiplexingRows"])
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected row object, got %v", rows[0])
	}
	if row["IbanNumber"] != float64(123) {
		t.Fatalf("expected numeric IbanNumber 123, got %v", row["IbanNumber"])
	}
	if row["Value"] != float64(1000) {
		t.Fatalf("expected Value 1000, got %v", row["Value"])
	}
	if received["PaymentIdentity"] != nil {
		t.Fatalf("expected PaymentIdentity to be omitted, got %v", received["PaymentIdentity"])
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

func TestSandboxRequestPaymentOmitsMultiplexingData(t *testing.T) {
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
	if received["MultiplexingData"] != nil {
		t.Fatalf("expected MultiplexingData to be omitted in sandbox, got %v", received["MultiplexingData"])
	}
	wantURL := sadad.SandboxEndpoints.GatewayURL + "?Token=sandbox-token"
	if got := resp.URL(); got != wantURL {
		t.Fatalf("expected %q, got %q", wantURL, got)
	}
}

func TestMultiplexingDataForAmount(t *testing.T) {
	data, err := sadad.MultiplexingDataForAmount("42", 5000)
	if err != nil {
		t.Fatalf("MultiplexingDataForAmount failed: %v", err)
	}
	if data.Type != "Amount" {
		t.Fatalf("expected Type Amount, got %q", data.Type)
	}
	if len(data.MultiplexingRows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data.MultiplexingRows))
	}
	if data.MultiplexingRows[0].IbanNumber != 42 || data.MultiplexingRows[0].Value != 5000 {
		t.Fatalf("unexpected row: %+v", data.MultiplexingRows[0])
	}
}

func TestMultiplexingDataForAmountRejectsNonNumericRow(t *testing.T) {
	_, err := sadad.MultiplexingDataForAmount("IR123", 5000)
	if err == nil {
		t.Fatal("expected error for non-numeric account row")
	}
}
