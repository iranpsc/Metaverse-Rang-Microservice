package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "metargb/shared/pb/auth"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newBankAccountAuthHandler(t *testing.T, kyc *testutil.MockKYCService) *handler.AuthHandler {
	t.Helper()
	conn, cleanup := testutil.DialAuthConn(&testutil.AuthMocks{KYC: kyc})
	t.Cleanup(cleanup)
	return handler.NewAuthHandler(conn, nil, "en")
}

func TestListBankAccounts_ParsesErrorsArray(t *testing.T) {
	kyc := &testutil.MockKYCService{}
	kyc.ListBankAccountsFunc = func(_ context.Context, _ *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error) {
		return &pb.ListBankAccountsResponse{
			Data: []*pb.BankAccountResponse{
				{
					Id:       1,
					BankName: "Tejarat",
					ShabaNum: "IR820540102680020817909002",
					CardNum:  "6037997551234567",
					Status:   -1,
					Errors:   `["rejected reason"]`,
				},
			},
		}, nil
	}
	h := newBankAccountAuthHandler(t, kyc)

	req := httptest.NewRequest(http.MethodGet, "/api/bank-accounts", nil)
	req = testutil.RequestWithUser(req, 42)
	w := httptest.NewRecorder()
	h.ListBankAccounts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := body["data"].([]interface{})
	item := data[0].(map[string]interface{})
	if item["id"] != float64(1) {
		t.Fatalf("unexpected id: %#v", item["id"])
	}
	errors, ok := item["errors"].([]interface{})
	if !ok || len(errors) != 1 {
		t.Fatalf("expected parsed errors array, got %#v", item["errors"])
	}
	if _, ok := item["errors"].(string); ok {
		t.Fatal("errors should not be returned as a raw string")
	}
}

func TestListBankAccounts_EmptyErrors(t *testing.T) {
	kyc := &testutil.MockKYCService{}
	kyc.ListBankAccountsFunc = func(_ context.Context, _ *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error) {
		return &pb.ListBankAccountsResponse{
			Data: []*pb.BankAccountResponse{
				{Id: 2, BankName: "Melli", Status: 1, Errors: ""},
			},
		}, nil
	}
	h := newBankAccountAuthHandler(t, kyc)

	req := httptest.NewRequest(http.MethodGet, "/api/bank-accounts", nil)
	req = testutil.RequestWithUser(req, 42)
	w := httptest.NewRecorder()
	h.ListBankAccounts(w, req)

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	item := body["data"].([]interface{})[0].(map[string]interface{})
	if item["errors"] != nil {
		t.Fatalf("expected nil errors for empty string, got %#v", item["errors"])
	}
}

func TestGetBankAccount_PlainStringErrorsFallback(t *testing.T) {
	kyc := &testutil.MockKYCService{}
	kyc.GetBankAccountFunc = func(_ context.Context, _ *pb.GetBankAccountRequest) (*pb.BankAccountResponse, error) {
		return &pb.BankAccountResponse{
			Id:     3,
			Status: -1,
			Errors: "not-json",
		}, nil
	}
	h := newBankAccountAuthHandler(t, kyc)

	req := httptest.NewRequest(http.MethodGet, "/api/bank-accounts/3", nil)
	req = testutil.RequestWithUser(req, 42)
	w := httptest.NewRecorder()
	h.GetBankAccount(w, req)

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	item := body["data"].(map[string]interface{})
	if item["errors"] != "not-json" {
		t.Fatalf("expected raw string errors, got %#v", item["errors"])
	}
}
