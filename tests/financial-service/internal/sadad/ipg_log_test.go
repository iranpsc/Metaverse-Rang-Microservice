package sadad_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"metarang/financial-service/internal/sadad"
)

func TestSadadIPGLogsRequestBeforeSendAndResponseAfterReceive(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	const responseBody = `{"ResCode":"0","Token":"stage-token","Description":"ok"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		written := logs.String()
		if !strings.Contains(written, "Sadad IPG request") || !strings.Contains(written, `"OrderId":77`) {
			t.Fatalf("request body was not logged before send: %s", written)
		}
		if strings.Contains(written, "Sadad IPG response") {
			t.Fatal("response was logged before the gateway answered")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()

	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL,
		VerifyURL:         server.URL + "/verify",
		GatewayURL:        "https://example.com/purchase",
	})
	resp, err := client.RequestPayment(sadad.RequestParams{
		MerchantID:    "merchant",
		TerminalID:    "terminal",
		SignData:      "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
		OrderID:       77,
		Amount:        2500,
		ReturnURL:     "https://example.com/callback",
		LocalDateTime: "01/02/2026 3:04:05 pm",
		MultiplexingData: &sadad.MultiplexingData{
			Type:             "Amount",
			MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "IR820540102680020817909002", Value: 2500}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token != "stage-token" {
		t.Fatalf("token=%q", resp.Token)
	}

	written := logs.String()
	if !strings.Contains(written, "Sadad IPG response") || !strings.Contains(written, responseBody) || !strings.Contains(written, "status=200") {
		t.Fatalf("response was not logged after receive: %s", written)
	}
	if !strings.Contains(written, server.URL) {
		t.Fatalf("request url missing from logs: %s", written)
	}

	logs.Reset()
	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(logs.String(), `"Token":"pay-token"`) {
			t.Fatalf("verify request was not logged before send: %s", logs.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ResCode":       0,
			"RetrivalRefNo": "9988",
			"Description":   "verified",
		})
	}))
	defer verifyServer.Close()

	verifyClient := sadad.NewClientWithEndpoints(sadad.Endpoints{VerifyURL: verifyServer.URL})
	verified, err := verifyClient.VerifyPayment(sadad.VerificationParams{
		Token:    "pay-token",
		SignData: "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.RetrivalRefNo != "9988" {
		t.Fatalf("rrn=%q", verified.RetrivalRefNo)
	}
	if !strings.Contains(logs.String(), "Sadad IPG response") || !strings.Contains(logs.String(), "9988") {
		t.Fatalf("verify response was not logged: %s", logs.String())
	}
}
