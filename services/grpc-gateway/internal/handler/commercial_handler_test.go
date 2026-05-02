package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCommercialHandler_GetCurrentUserWallet_Unauthorized(t *testing.T) {
	h := NewCommercialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/user/wallet", nil)
	rr := httptest.NewRecorder()
	h.GetCurrentUserWallet(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCommercialHandler_GetCurrentUserWallet_MethodNotAllowed(t *testing.T) {
	h := NewCommercialHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/user/wallet", nil)
	rr := httptest.NewRecorder()
	h.GetCurrentUserWallet(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestCommercialHandler_ListTransactions_Unauthorized(t *testing.T) {
	h := NewCommercialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/user/transactions", nil)
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCommercialHandler_GetLatestTransaction_Unauthorized(t *testing.T) {
	h := NewCommercialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/user/transactions/latest", nil)
	rr := httptest.NewRecorder()
	h.GetLatestTransaction(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
