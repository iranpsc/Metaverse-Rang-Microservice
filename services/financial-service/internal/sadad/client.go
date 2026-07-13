package sadad

import (
	"bytes"
	"crypto/des"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var tehranLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		return time.FixedZone("IRST", 3*60*60+30*60)
	}
	return loc
}()

const sadadHost = "https://sadad.shaparak.ir"

const (
	productionVerifyURL         = sadadHost + "/VPG/api/v0/Advice/Verify"
	productionGatewayURL        = sadadHost + "/VPG/Purchase"
	productionPaymentRequestURL = sadadHost + "/api/v0/Request/PaymentRequest"
)

const banktestSandboxHost = "https://sandbox.banktest.ir/melli/sadad.shaparak.ir"

const (
	sandboxPaymentRequestURL = banktestSandboxHost + "/VPG/api/v0/Request/PaymentRequest"
	sandboxVerifyURL         = banktestSandboxHost + "/VPG/api/v0/Advice/Verify"
	sandboxGatewayURL        = banktestSandboxHost + "/VPG/Purchase"
)

// Endpoints holds Sadad API URLs for a given environment.
type Endpoints struct {
	PaymentRequestURL string
	VerifyURL         string
	GatewayURL        string
	Multiplexed       bool // production sends MultiplexingData; BankTest sandbox omits it
}

// ProductionEndpoints are the live Sadad (Bank Melli) IPG URLs.
var ProductionEndpoints = Endpoints{
	PaymentRequestURL: productionPaymentRequestURL,
	VerifyURL:         productionVerifyURL,
	GatewayURL:        productionGatewayURL,
	Multiplexed:       true,
}

// SandboxEndpoints are the BankTest URLs (https://banktest.ir) for local/dev testing.
var SandboxEndpoints = Endpoints{
	PaymentRequestURL: sandboxPaymentRequestURL,
	VerifyURL:         sandboxVerifyURL,
	GatewayURL:        sandboxGatewayURL,
	Multiplexed:       false,
}

// Client handles Sadad payment gateway operations (Bank Melli).
type Client struct {
	httpClient *http.Client
	endpoints  Endpoints
}

// NewClient creates a Sadad client using production endpoints.
func NewClient() *Client {
	return NewClientWithSandbox(false)
}

// NewClientWithSandbox creates a Sadad client. When sandbox is true, requests are sent to
// BankTest (https://sandbox.banktest.ir/melli/...) instead of the live Sadad gateway.
func NewClientWithSandbox(sandbox bool) *Client {
	endpoints := ProductionEndpoints
	if sandbox {
		endpoints = SandboxEndpoints
	}
	return NewClientWithEndpoints(endpoints)
}

// NewClientWithEndpoints creates a client with custom API URLs (mainly for tests).
func NewClientWithEndpoints(endpoints Endpoints) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoints: endpoints,
	}
}

// MultiplexingRow routes payment amount to a target IBAN sub-account.
type MultiplexingRow struct {
	IbanNumber string `json:"IbanNumber"`
	Value      int64  `json:"Value"`
}

// MultiplexingData routes payment to one or more IBAN sub-accounts.
type MultiplexingData struct {
	Type             string            `json:"Type"`
	MultiplexingRows []MultiplexingRow `json:"MultiplexingRows"`
}

// MultiplexingDataForAmount builds amount-based multiplexing for a single IBAN.
func MultiplexingDataForAmount(ibanNumber string, amount int64) *MultiplexingData {
	return &MultiplexingData{
		Type: "Percentage",
		MultiplexingRows: []MultiplexingRow{
			{IbanNumber: ibanNumber, Value: 100},
		},
	}
}

// MarshalMultiplexingData serializes multiplexing data as JSON for Sadad IPG requests.
func MarshalMultiplexingData(data *MultiplexingData) (json.RawMessage, error) {
	if data == nil {
		return nil, fmt.Errorf("multiplexing data is nil")
	}
	return json.Marshal(data)
}

// RequestParams for Sadad payment token request.
type RequestParams struct {
	MerchantID       string
	TerminalID       string
	TransactionKey   string // base64-encoded TripleDES key
	OrderID          string
	Amount           int64 // Rials
	ReturnURL        string
	MultiplexingData json.RawMessage // JSON-encoded MultiplexingData object
}

// RequestResponse is the response from Sadad payment request.
type RequestResponse struct {
	ResCode     string
	Token       string
	Description string
	gatewayURL  string
}

// VerificationParams for payment verification.
type VerificationParams struct {
	TransactionKey string
	Token          string
}

// VerificationResponse is the response from Sadad verification.
type VerificationResponse struct {
	ResCode          string
	SystemTraceNo    string
	RetrivalRefNo    string
	CardNumberMasked string
	Description      string
}

type paymentRequestBody struct {
	TerminalID       string          `json:"TerminalId"`
	MerchantID       string          `json:"MerchantId"`
	Amount           int64           `json:"Amount"`
	OrderID          string          `json:"OrderId"`
	LocalDateTime    string          `json:"LocalDateTime"`
	ReturnURL        string          `json:"ReturnUrl"`
	SignData         string          `json:"SignData"`
	MultiplexingData json.RawMessage `json:"MultiplexingData,omitempty"`
}

type paymentRequestAPIResponse struct {
	ResCode     json.RawMessage `json:"ResCode"`
	Token       string          `json:"Token"`
	Description string          `json:"Description"`
}

type verifyRequestBody struct {
	Token    string `json:"Token"`
	SignData string `json:"SignData"`
}

type verifyAPIResponse struct {
	ResCode          json.RawMessage `json:"ResCode"`
	SystemTraceNo    string          `json:"SystemTraceNo"`
	RetrivalRefNo    string          `json:"RetrivalRefNo"`
	CardNumberMasked string          `json:"CardNumberMasked"`
	Description      string          `json:"Description"`
}

// RequestPayment initiates a payment request and returns a token.
// Production includes MultiplexingData; BankTest sandbox omits it.
func (c *Client) RequestPayment(params RequestParams) (*RequestResponse, error) {
	if c.endpoints.Multiplexed && len(params.MultiplexingData) == 0 {
		return nil, fmt.Errorf("multiplexing data is required for multiplexed payments")
	}

	signData, err := generateSignData(
		fmt.Sprintf("%s;%s;%d", params.TerminalID, params.OrderID, params.Amount),
		params.TransactionKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate sign data: %w", err)
	}

	localDateTime := sadadLocalDateTime()
	payload, err := json.Marshal(paymentRequestBody{
		TerminalID:       params.TerminalID,
		MerchantID:       params.MerchantID,
		Amount:           params.Amount,
		OrderID:          params.OrderID,
		LocalDateTime:    localDateTime,
		ReturnURL:        params.ReturnURL,
		SignData:         signData,
		MultiplexingData: params.MultiplexingData,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoints.PaymentRequestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Sadad rejects requests with a non-empty User-Agent.
	req.Header.Set("User-Agent", "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sadad returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if len(respBody) == 0 {
		return nil, fmt.Errorf("sadad returned empty response body")
	}

	var apiResp paymentRequestAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &RequestResponse{
		ResCode:     parseResCode(apiResp.ResCode),
		Token:       apiResp.Token,
		Description: apiResp.Description,
		gatewayURL:  c.endpoints.GatewayURL,
	}, nil
}

// VerifyPayment verifies a payment with Sadad.
func (c *Client) VerifyPayment(params VerificationParams) (*VerificationResponse, error) {
	signData, err := generateSignData(params.Token, params.TransactionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate sign data: %w", err)
	}

	body := verifyRequestBody{
		Token:    params.Token,
		SignData: signData,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoints.VerifyURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send verification request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read verification response: %w", err)
	}

	var apiResp verifyAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse verification response: %w", err)
	}

	return &VerificationResponse{
		ResCode:          parseResCode(apiResp.ResCode),
		SystemTraceNo:    apiResp.SystemTraceNo,
		RetrivalRefNo:    apiResp.RetrivalRefNo,
		CardNumberMasked: apiResp.CardNumberMasked,
		Description:      apiResp.Description,
	}, nil
}

// Success checks if the request response indicates success.
func (r *RequestResponse) Success() bool {
	return isSuccessResCode(r.ResCode) && r.Token != ""
}

// URL returns the payment gateway URL for the given token.
func (r *RequestResponse) URL() string {
	if !r.Success() {
		return ""
	}
	return fmt.Sprintf("%s?Token=%s", r.gatewayURL, r.Token)
}

// Error returns error information for the request.
func (r *RequestResponse) Error() *SadadError {
	return NewSadadError(r.ResCode)
}

// Success checks if the verification response indicates success.
func (v *VerificationResponse) Success() bool {
	return isSuccessResCode(v.ResCode) && v.RetrivalRefNo != ""
}

// Error returns error information for the verification.
func (v *VerificationResponse) Error() *SadadError {
	return NewSadadError(v.ResCode)
}

// generateSignData encrypts request fields using 3DES per Sadad/Shaparak gateway specification.
func generateSignData(data, base64Key string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return "", fmt.Errorf("invalid transaction key: %w", err)
	}

	// Sadad mandates Triple-DES for SignData; cannot substitute a different algorithm.
	block, err := des.NewTripleDESCipher(key) // lgtm[go/weak-cryptographic-algorithm]
	if err != nil {
		return "", fmt.Errorf("failed to create 3DES cipher: %w", err)
	}

	padded := pkcs7Pad([]byte(data), block.BlockSize())
	encrypted := make([]byte, len(padded))

	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(encrypted[i:i+block.BlockSize()], padded[i:i+block.BlockSize()]) // lgtm[go/weak-cryptographic-algorithm]
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func parseResCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var code string
	if err := json.Unmarshal(raw, &code); err == nil {
		return code
	}

	var codeInt int
	if err := json.Unmarshal(raw, &codeInt); err == nil {
		return fmt.Sprintf("%d", codeInt)
	}

	return string(raw)
}

func isSuccessResCode(code string) bool {
	return code == "0"
}

// sadadLocalDateTime returns the timestamp Sadad expects (Iran local time).
// Matches shetabit/multipay Sadad driver format: m/d/Y g:i:s a
func sadadLocalDateTime() string {
	return time.Now().In(tehranLocation).Format("1/2/2006 3:04:05 pm")
}
