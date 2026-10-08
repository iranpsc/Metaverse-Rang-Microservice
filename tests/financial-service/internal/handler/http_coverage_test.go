package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"metarang/financial-service/internal/handler"
	financialpb "metarang/shared/pb/financial"
	"metarang/shared/pkg/helpers"
)

func TestHTTPCreateOrder_EmptyAndInvalidBody(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})

	req := httptest.NewRequest(http.MethodPost, "/api/order", http.NoBody)
	req = withUser(req, 1)
	req.ContentLength = 0
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty body status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader("{bad"))
	req = withUser(req, 1)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":1,"asset":"nope"}`))
	req = withUser(req, 1)
	w = httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid asset status=%d", w.Code)
	}
}

func TestHTTPCreateOrder_HandlerErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"plain", errors.New("boom"), http.StatusInternalServerError},
		{"unauthenticated", status.Error(codes.Unauthenticated, "nope"), http.StatusUnauthorized},
		{"permission", status.Error(codes.PermissionDenied, "denied"), http.StatusForbidden},
		{"exists", status.Error(codes.AlreadyExists, "dup"), http.StatusConflict},
		{"precondition", status.Error(codes.FailedPrecondition, "pre"), http.StatusPreconditionFailed},
		{"unavailable", status.Error(codes.Unavailable, "down"), http.StatusServiceUnavailable},
		{"internal", status.Error(codes.Internal, "x"), http.StatusInternalServerError},
		{"invalid plain", status.Error(codes.InvalidArgument, "amount invalid"), http.StatusUnprocessableEntity},
		{"invalid encoded", status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{"amount": "bad"})), http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := &mockOrderAPI{}
			order.CreateOrderFunc = func(context.Context, *financialpb.CreateOrderRequest) (*financialpb.CreateOrderResponse, error) {
				return nil, tc.err
			}
			h := handler.NewHTTPFinancialHandler(order, &mockStoreAPI{})
			req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":1,"asset":"psc"}`))
			req = withUser(req, 1)
			req.Header.Set("Accept-Language", "fa")
			w := httptest.NewRecorder()
			h.CreateOrder(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestHTTPHandleCallback_MoreInputs(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})

	req := httptest.NewRequest(http.MethodPut, "/api/order/callback", nil)
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/order/callback?order_id=12", nil)
	w = httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("query order_id status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/order/callback", strings.NewReader("OrderId=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid OrderId status=%d", w.Code)
	}

	form := strings.NewReader("OrderId=12&token=abc&resCode=0&PrimaryAccNo=pan")
	req = httptest.NewRequest(http.MethodPost, "/api/order/callback", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("form OrderId status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHTTPGetStorePackages_MoreBranches(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})

	req := httptest.NewRequest(http.MethodGet, "/api/store", nil)
	w := httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/store", http.NoBody)
	req.ContentLength = 0
	w = httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty body status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/store", strings.NewReader("{"))
	w = httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad json status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/store", strings.NewReader(`{"codes":["aa","b"]}`))
	w = httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short code status=%d", w.Code)
	}

	store := &mockStoreAPI{}
	store.GetStorePackagesFunc = func(ctx context.Context, req *financialpb.GetStorePackagesRequest) (*financialpb.GetStorePackagesResponse, error) {
		if md, ok := metadata.FromIncomingContext(ctx); !ok || len(md.Get("accept-language")) == 0 {
			t.Fatal("expected accept-language metadata")
		}
		return nil, status.Error(codes.Internal, "store down")
	}
	h = handler.NewHTTPFinancialHandler(&mockOrderAPI{}, store)
	req = httptest.NewRequest(http.MethodPost, "/api/store", strings.NewReader(`{"codes":["aa","bb"]}`))
	req.Header.Set("Accept-Language", "en")
	w = httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestStartHTTPServer_ListenError(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	pass := func(next http.Handler) http.Handler { return next }
	err := handler.StartHTTPServer(h, "not-a-port", pass, pass)
	if err == nil {
		t.Fatal("expected listen error")
	}
}

func TestNewHTTPFinancialHandler_DefaultLocale(t *testing.T) {
	t.Setenv("PROJECT_LOCALE", "")
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":0,"asset":"psc"}`))
	req = withUser(req, 1)
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", w.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body == nil {
		t.Fatal("expected json body")
	}
}
