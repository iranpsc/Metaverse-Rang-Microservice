package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/financial-service/internal/handler"
	financialpb "metarang/shared/pb/financial"
	authpkg "metarang/shared/pkg/auth"
)

type mockOrderAPI struct {
	CreateOrderFunc    func(context.Context, *financialpb.CreateOrderRequest) (*financialpb.CreateOrderResponse, error)
	HandleCallbackFunc func(context.Context, *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error)
}

func (m *mockOrderAPI) CreateOrder(ctx context.Context, req *financialpb.CreateOrderRequest) (*financialpb.CreateOrderResponse, error) {
	if m.CreateOrderFunc != nil {
		return m.CreateOrderFunc(ctx, req)
	}
	return &financialpb.CreateOrderResponse{Link: "https://pay.example.com"}, nil
}

func (m *mockOrderAPI) HandleCallback(ctx context.Context, req *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error) {
	if m.HandleCallbackFunc != nil {
		return m.HandleCallbackFunc(ctx, req)
	}
	return &financialpb.HandleCallbackResponse{RedirectUrl: "https://frontend/verify"}, nil
}

type mockStoreAPI struct {
	GetStorePackagesFunc func(context.Context, *financialpb.GetStorePackagesRequest) (*financialpb.GetStorePackagesResponse, error)
}

func (m *mockStoreAPI) GetStorePackages(ctx context.Context, req *financialpb.GetStorePackagesRequest) (*financialpb.GetStorePackagesResponse, error) {
	if m.GetStorePackagesFunc != nil {
		return m.GetStorePackagesFunc(ctx, req)
	}
	return &financialpb.GetStorePackagesResponse{}, nil
}

func withUser(r *http.Request, userID uint64) *http.Request {
	userCtx := &authpkg.UserContext{UserID: userID, Email: "u@example.com", Token: "tok"}
	return r.WithContext(context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx))
}

func TestHTTPCreateOrder_Success(t *testing.T) {
	order := &mockOrderAPI{}
	order.CreateOrderFunc = func(_ context.Context, req *financialpb.CreateOrderRequest) (*financialpb.CreateOrderResponse, error) {
		if req.UserId != 42 || req.Amount != 100 || req.Asset != "psc" {
			t.Fatalf("req=%+v", req)
		}
		return &financialpb.CreateOrderResponse{Link: "https://pay.example.com"}, nil
	}
	h := handler.NewHTTPFinancialHandler(order, &mockStoreAPI{})

	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":100,"asset":"psc"}`))
	req = withUser(req, 42)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["link"] != "https://pay.example.com" {
		t.Fatalf("body=%v", body)
	}
}

func TestHTTPCreateOrder_Unauthorized(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":1,"asset":"psc"}`))
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHTTPCreateOrder_Validation(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(`{"amount":0,"asset":"psc"}`))
	req = withUser(req, 1)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHTTPCreateOrder_MethodNotAllowed(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodGet, "/api/order", nil)
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHTTPHandleCallback_Redirect(t *testing.T) {
	order := &mockOrderAPI{}
	order.HandleCallbackFunc = func(_ context.Context, req *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error) {
		if req.OrderId != 99 || req.Token != "tok" || req.ResCode != "0" {
			t.Fatalf("req=%+v", req)
		}
		return &financialpb.HandleCallbackResponse{RedirectUrl: "https://frontend/ok"}, nil
	}
	h := handler.NewHTTPFinancialHandler(order, &mockStoreAPI{})

	form := url.Values{}
	form.Set("OrderId", "99")
	form.Set("Token", "tok")
	form.Set("ResCode", "0")
	req := httptest.NewRequest(http.MethodPost, "/api/order/callback?order_id=1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status=%d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://frontend/ok" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHTTPHandleCallback_RestoresPlusInToken(t *testing.T) {
	order := &mockOrderAPI{}
	order.HandleCallbackFunc = func(_ context.Context, req *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error) {
		if req.Token != "abc+def/ghi=" {
			t.Fatalf("token=%q", req.Token)
		}
		return &financialpb.HandleCallbackResponse{RedirectUrl: "https://frontend/ok"}, nil
	}
	h := handler.NewHTTPFinancialHandler(order, &mockStoreAPI{})

	req := httptest.NewRequest(http.MethodPost, "/api/order/callback?order_id=1", strings.NewReader("OrderId=99&Token=abc+def/ghi%3D&ResCode=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHTTPHandleCallback_MissingOrderID(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodGet, "/api/order/callback", nil)
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHTTPGetStorePackages_ArrayResponse(t *testing.T) {
	img := "img.png"
	store := &mockStoreAPI{}
	store.GetStorePackagesFunc = func(_ context.Context, req *financialpb.GetStorePackagesRequest) (*financialpb.GetStorePackagesResponse, error) {
		if len(req.Codes) != 2 {
			t.Fatalf("codes=%v", req.Codes)
		}
		return &financialpb.GetStorePackagesResponse{
			Packages: []*financialpb.Package{
				{Id: 1, Code: "aa", Asset: "psc", Amount: 10, UnitPrice: 1.5, Image: &img},
				{Id: 2, Code: "bb", Asset: "irr", Amount: 20, UnitPrice: 2.0},
			},
		}, nil
	}
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, store)

	req := httptest.NewRequest(http.MethodPost, "/api/store", strings.NewReader(`{"codes":["aa","bb"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.GetStorePackages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data, ok := body["data"].([]interface{})
	if !ok || len(data) != 2 {
		t.Fatalf("expected wrapped array, got %v", body)
	}
}

func TestHTTPGetStorePackages_Validation(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/store", strings.NewReader(`{"codes":["a"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.GetStorePackages(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHTTPHandlerError_NotFound(t *testing.T) {
	order := &mockOrderAPI{}
	order.HandleCallbackFunc = func(context.Context, *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error) {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	h := handler.NewHTTPFinancialHandler(order, &mockStoreAPI{})

	form := url.Values{}
	form.Set("OrderId", "1")
	req := httptest.NewRequest(http.MethodPost, "/api/order/callback?order_id=99", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.HandleCallback(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestRegisterHTTPRoutes_Health(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	mux := http.NewServeMux()
	passThrough := func(next http.Handler) http.Handler { return next }
	h.RegisterHTTPRoutes(mux, passThrough, passThrough)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestRegisterHTTPRoutes_CallbackTrailingSlash(t *testing.T) {
	h := handler.NewHTTPFinancialHandler(&mockOrderAPI{}, &mockStoreAPI{})
	mux := http.NewServeMux()
	passThrough := func(next http.Handler) http.Handler { return next }
	h.RegisterHTTPRoutes(mux, passThrough, passThrough)

	for _, path := range []string{"/api/order/callback/", "/api/payment/callback/"} {
		form := url.Values{}
		form.Set("OrderId", "1")
		req := httptest.NewRequest(http.MethodPost, path+"?order_id=99", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusFound {
			t.Fatalf("path=%s status=%d", path, w.Code)
		}
	}
}
