package sadad_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"metarang/financial-service/internal/sadad"
)

const testKey = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0"

func TestNewClientConstructors(t *testing.T) {
	if sadad.NewClient() == nil {
		t.Fatal("NewClient")
	}
	if sadad.NewClientWithSandbox(true) == nil {
		t.Fatal("sandbox client")
	}
	if sadad.NewClientWithSandbox(false) == nil {
		t.Fatal("production client")
	}
}

func TestRequestPaymentValidationAndErrors(t *testing.T) {
	client := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: "http://127.0.0.1:1",
		VerifyURL:         "http://127.0.0.1:1",
		GatewayURL:        "https://gw",
		Multiplexed:       true,
	})

	t.Run("nil multiplexing", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{SignData: testKey, Amount: 1})
		if err == nil {
			t.Fatal("expected multiplexing required")
		}
	})
	t.Run("empty type", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData:         testKey,
			MultiplexingData: &sadad.MultiplexingData{Type: "", MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "IR1"}}},
		})
		if err == nil {
			t.Fatal("expected type required")
		}
	})
	t.Run("empty rows", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData:         testKey,
			MultiplexingData: &sadad.MultiplexingData{Type: "Percentage"},
		})
		if err == nil {
			t.Fatal("expected rows required")
		}
	})
	t.Run("empty iban", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData: testKey,
			MultiplexingData: &sadad.MultiplexingData{
				Type:             "Amount",
				MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "", Value: 1000}},
			},
		})
		if err == nil {
			t.Fatal("expected iban required")
		}
	})
	t.Run("zero value row", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData: testKey,
			MultiplexingData: &sadad.MultiplexingData{
				Type: "Percentage",
				MultiplexingRows: []sadad.MultiplexingRow{
					{IbanNumber: "IR1", Value: 100},
					{IbanNumber: "IR2", Value: 0},
				},
			},
		})
		if err == nil {
			t.Fatal("expected zero value rejected")
		}
	})
	t.Run("percentage sum not 100", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData: testKey,
			MultiplexingData: &sadad.MultiplexingData{
				Type:             "Percentage",
				MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "IR1", Value: 50}},
			},
		})
		if err == nil {
			t.Fatal("expected percentage sum validation")
		}
	})
	t.Run("invalid type", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData: testKey,
			MultiplexingData: &sadad.MultiplexingData{
				Type:             "Foo",
				MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "IR1", Value: 100}},
			},
		})
		if err == nil {
			t.Fatal("expected invalid type")
		}
	})
	t.Run("invalid sign key", func(t *testing.T) {
		_, err := client.RequestPayment(sadad.RequestParams{
			SignData: "%%%",
			MultiplexingData: &sadad.MultiplexingData{
				Type:             "Amount",
				MultiplexingRows: []sadad.MultiplexingRow{{IbanNumber: "IR1", Value: 1000}},
			},
		})
		if err == nil {
			t.Fatal("expected sign error")
		}
	})
}

func TestRequestPaymentHTTPFailures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok-empty", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	})
	mux.HandleFunc("/bad-json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{not-json")
	})
	mux.HandleFunc("/codes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ResCode":     "0",
			"Token":       "tok",
			"Description": "ok",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	params := sadad.RequestParams{
		MerchantID: "m",
		TerminalID: "t",
		SignData:   testKey,
		OrderID:    1,
		Amount:     10,
		ReturnURL:  "http://cb",
		MultiplexingData: &sadad.MultiplexingData{
			Type: "Amount",
			MultiplexingRows: []sadad.MultiplexingRow{
				{IbanNumber: "IR1", Value: 10},
			},
		},
		LocalDateTime: "01/02/2006 3:04:05 pm",
	}

	for _, path := range []string{"/ok-empty", "/fail", "/bad-json"} {
		c := sadad.NewClientWithEndpoints(sadad.Endpoints{
			PaymentRequestURL: server.URL + path,
			VerifyURL:         server.URL + path,
			GatewayURL:        "https://gw",
			Multiplexed:       true,
		})
		if _, err := c.RequestPayment(params); err == nil {
			t.Fatalf("path %s expected error", path)
		}
	}

	c := sadad.NewClientWithEndpoints(sadad.Endpoints{
		PaymentRequestURL: server.URL + "/codes",
		VerifyURL:         server.URL + "/codes",
		GatewayURL:        "https://gw.example/purchase",
		Multiplexed:       true,
	})
	resp, err := c.RequestPayment(params)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success() {
		t.Fatal("expected success")
	}
	if resp.URL() == "" {
		t.Fatal("expected gateway url")
	}
	if resp.Error() == nil || !resp.Error().IsSuccess() {
		t.Fatal("expected success sadad error wrapper")
	}

	failResp := &sadad.RequestResponse{ResCode: "101", Token: ""}
	if failResp.Success() || failResp.URL() != "" {
		t.Fatal("failed request should have empty URL")
	}
	if failResp.Error().Message() == "" || failResp.Error().GetCode() != "101" {
		t.Fatalf("error mapping %+v", failResp.Error())
	}
}

func TestVerifyPayment(t *testing.T) {
	t.Run("invalid key", func(t *testing.T) {
		c := sadad.NewClientWithEndpoints(sadad.Endpoints{VerifyURL: "http://127.0.0.1:1"})
		if _, err := c.VerifyPayment(sadad.VerificationParams{Token: "t", SignData: "%%%"}); err == nil {
			t.Fatal("expected sign error")
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ResCode":          0,
			"SystemTraceNo":    "st",
			"RetrivalRefNo":    "rrn",
			"CardNumberMasked": "1234****",
			"Description":      "ok",
		})
	}))
	defer server.Close()

	c := sadad.NewClientWithEndpoints(sadad.Endpoints{VerifyURL: server.URL, GatewayURL: "https://gw"})
	resp, err := c.VerifyPayment(sadad.VerificationParams{Token: "tok", SignData: testKey})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success() {
		t.Fatalf("expected success %+v", resp)
	}
	if resp.Error() == nil || resp.Error().GetCode() != "0" {
		t.Fatal("expected error wrapper")
	}

	fail := &sadad.VerificationResponse{ResCode: "105"}
	if fail.Success() {
		t.Fatal("expected failure without retrival ref")
	}
	if fail.Error().Message() == "" {
		t.Fatal("expected mapped message")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-json")
	}))
	defer badJSON.Close()
	c = sadad.NewClientWithEndpoints(sadad.Endpoints{VerifyURL: badJSON.URL})
	if _, err := c.VerifyPayment(sadad.VerificationParams{Token: "tok", SignData: testKey}); err == nil {
		t.Fatal("expected parse error")
	}

	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := closed.URL
	closed.Close()
	c = sadad.NewClientWithEndpoints(sadad.Endpoints{VerifyURL: url})
	if _, err := c.VerifyPayment(sadad.VerificationParams{Token: "tok", SignData: testKey}); err == nil {
		t.Fatal("expected send error")
	}
}

func TestSadadErrorMessages(t *testing.T) {
	codes := []string{"0", "-1", "101", "102", "103", "104", "105", "106", "107", "1104", "999"}
	for _, code := range codes {
		e := sadad.NewSadadError(code)
		if e.GetCode() != code {
			t.Fatalf("code=%s", code)
		}
		if e.Message() == "" {
			t.Fatalf("empty message for %s", code)
		}
		if e.IsSuccess() != (code == "0") {
			t.Fatalf("IsSuccess for %s", code)
		}
	}
	if got := sadad.NewSadadError("1104").Message(); got != "اطلاعات تسهیم صحیح نیست" {
		t.Fatalf("expected multiplexing error message, got %q", got)
	}
}
