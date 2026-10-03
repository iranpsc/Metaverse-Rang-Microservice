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
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("expected Accept application/json, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", got)
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
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID: "merchant",
		TerminalID: "terminal",
		SignData:   "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:    42,
		Amount:     1000,
		ReturnURL:  "https://example.com/callback",
		MultiplexingData: &sadad.MultiplexingData{
			Type: "Amount",
			MultiplexingRows: []sadad.MultiplexingRow{
				{IbanNumber: "IRRIAL", Value: 1000},
			},
		},
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
	if received["PaymentIdentity"] != nil {
		t.Fatalf("expected PaymentIdentity to be omitted, got %v", received["PaymentIdentity"])
	}

	muxData, ok := received["MultiplexingData"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected MultiplexingData object, got %T", received["MultiplexingData"])
	}
	if muxData["Type"] != "Amount" {
		t.Fatalf("expected Type Amount, got %v", muxData["Type"])
	}
	rows, ok := muxData["MultiplexingRows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected 1 MultiplexingRow, got %v", muxData["MultiplexingRows"])
	}
	row0, _ := rows[0].(map[string]interface{})
	if row0["IbanNumber"] != "IRRIAL" || row0["Value"] != float64(1000) {
		t.Fatalf("unexpected multiplexing row: %+v", row0)
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
	// Zero-padded month/day per PHP m/d/Y (e.g. 09/16/2026).
	parts := strings.SplitN(localDateTime, " ", 2)
	if len(parts) == 0 || len(parts[0]) != 10 {
		t.Fatalf("expected zero-padded m/d/Y LocalDateTime date, got %q", localDateTime)
	}
}

func TestRequestPaymentMatchesSadadSignAndNumericIban(t *testing.T) {
	const paymentSign = "Tci2sESpUbJTIXxCNUVbqNRbbME4Pt8Kqyxz7+IR7eGTNcwvx4XCaw=="

	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ResCode":     0,
			"Token":       "abc+def/ghi=",
			"Description": "تراکنش موفق",
		})
	}))
	defer server.Close()

	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL,
		VerifyURL:         server.URL,
		GatewayURL:        "https://sadad.shaparak.ir/Purchase",
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID:    "000000140339999",
		TerminalID:    "24095674",
		SignData:      "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:       1234567890123456,
		Amount:        150000,
		ReturnURL:     "https://example.com/callback",
		LocalDateTime: "10/02/2026 3:30:00 pm",
		MultiplexingData: &sadad.MultiplexingData{
			Type: "Amount",
			MultiplexingRows: []sadad.MultiplexingRow{
				{IbanNumber: "1", Value: 150000},
			},
		},
	})
	if err != nil {
		t.Fatalf("RequestPayment failed: %v", err)
	}

	if received["SignData"] != paymentSign {
		t.Fatalf("SignData = %v", received["SignData"])
	}
	if received["LocalDateTime"] != "10/02/2026 3:30:00 pm" {
		t.Fatalf("LocalDateTime = %v", received["LocalDateTime"])
	}
	if received["OrderId"] != float64(1234567890123456) || received["Amount"] != float64(150000) {
		t.Fatalf("unexpected amount or order id: %+v", received)
	}
	muxData, _ := received["MultiplexingData"].(map[string]interface{})
	rows, _ := muxData["MultiplexingRows"].([]interface{})
	row0, _ := rows[0].(map[string]interface{})
	if row0["IbanNumber"] != float64(1) || row0["Value"] != float64(150000) {
		t.Fatalf("expected numeric iban row, got %+v", row0)
	}

	wantURL := "https://sadad.shaparak.ir/Purchase?Token=abc%2Bdef%2Fghi%3D"
	if got := resp.URL(); got != wantURL {
		t.Fatalf("purchase url %q", got)
	}
}

func TestRequestPaymentRejectsNonNumericAccountIndex(t *testing.T) {
	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: "http://127.0.0.1:1",
	})
	_, err := client.RequestPayment(sadad.RequestParams{
		TerminalID: "24095674",
		SignData:   "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:    1,
		Amount:     10,
		MultiplexingData: &sadad.MultiplexingData{
			Type:             "Amount",
			MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "not-an-index", Value: 10}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid iban number")
	}
}
