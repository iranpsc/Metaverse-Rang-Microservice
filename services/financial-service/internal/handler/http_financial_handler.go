package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	"metarang/financial-service/internal/middleware"
	financialpb "metarang/shared/pb/financial"
	"metarang/shared/pkg/helpers"
	"metarang/shared/pkg/sentry"
)

type orderAPI interface {
	CreateOrder(context.Context, *financialpb.CreateOrderRequest) (*financialpb.CreateOrderResponse, error)
	HandleCallback(context.Context, *financialpb.HandleCallbackRequest) (*financialpb.HandleCallbackResponse, error)
}

type storeAPI interface {
	GetStorePackages(context.Context, *financialpb.GetStorePackagesRequest) (*financialpb.GetStorePackagesResponse, error)
}

// HTTPFinancialHandler serves Kong-facing REST routes for financial-service.
type HTTPFinancialHandler struct {
	order  orderAPI
	store  storeAPI
	locale string
}

// NewHTTPFinancialHandler wraps local gRPC handlers for HTTP use.
func NewHTTPFinancialHandler(order orderAPI, store storeAPI) *HTTPFinancialHandler {
	locale := strings.ToLower(os.Getenv("PROJECT_LOCALE"))
	if locale == "" {
		locale = "en"
	}
	return &HTTPFinancialHandler{order: order, store: store, locale: locale}
}

// RegisterHTTPRoutes registers financial REST routes and /health.
func (h *HTTPFinancialHandler) RegisterHTTPRoutes(
	mux *http.ServeMux,
	authMiddleware func(http.Handler) http.Handler,
	optionalAuthMiddleware func(http.Handler) http.Handler,
) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	callbackHandler := http.HandlerFunc(h.HandleCallback)
	registerExactAndTrailingSlash(mux, callbackHandler,
		"/api/order/callback",
		"/api/payment/callback",
	)
	mux.Handle("/api/order", authMiddleware(http.HandlerFunc(h.CreateOrder)))
	mux.Handle("/api/store", optionalAuthMiddleware(http.HandlerFunc(h.GetStorePackages)))
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(
	httpHandler *HTTPFinancialHandler,
	port string,
	authMiddleware func(http.Handler) http.Handler,
	optionalAuthMiddleware func(http.Handler) http.Handler,
) error {
	mux := http.NewServeMux()
	httpHandler.RegisterHTTPRoutes(mux, authMiddleware, optionalAuthMiddleware)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}

func registerExactAndTrailingSlash(mux *http.ServeMux, handler http.Handler, paths ...string) {
	for _, path := range paths {
		mux.Handle(path, handler)
		if !strings.HasSuffix(path, "/") {
			mux.Handle(path+"/", handler)
		}
	}
}

func contextWithAcceptLanguage(r *http.Request) context.Context {
	al := r.Header.Get("Accept-Language")
	if al == "" {
		return r.Context()
	}
	md := metadata.Pairs("accept-language", al, "grpcgateway-accept-language", al)
	return metadata.NewIncomingContext(r.Context(), md)
}

// CreateOrder handles POST /api/order
func (h *HTTPFinancialHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Amount int32  `json:"amount"`
		Asset  string `json:"asset"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Amount < 1 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"amount": "The amount field must be at least 1",
		}, h.locale)
		return
	}

	validAssets := map[string]bool{"psc": true, "irr": true, "red": true, "blue": true, "yellow": true}
	if !validAssets[req.Asset] {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"asset": "The selected asset is invalid",
		}, h.locale)
		return
	}

	grpcReq := &financialpb.CreateOrderRequest{
		UserId: userCtx.UserID,
		Amount: req.Amount,
		Asset:  req.Asset,
	}

	resp, err := h.order.CreateOrder(contextWithAcceptLanguage(r), grpcReq)
	if err != nil {
		writeHandlerError(w, err, h.locale)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"link": resp.Link,
	}, true)
}

// sadadCallbackToken restores "+" that form decoding turned into spaces.
// Sadad tokens are Base64, so a space never belongs in the token sent to Verify.
func sadadCallbackToken(r *http.Request) string {
	token := r.FormValue("Token")
	if token == "" {
		token = r.FormValue("token")
	}
	return strings.ReplaceAll(strings.TrimSpace(token), " ", "+")
}

// HandleCallback handles GET|POST /api/order/callback and /api/payment/callback
func (h *HTTPFinancialHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form data")
		return
	}

	// OrderId comes only from the IPG POST body. Query-string order_id is ignored
	// so a tampered ReturnUrl cannot select a different order.
	orderIDStr := r.PostFormValue("OrderId")
	if orderIDStr == "" {
		writeError(w, http.StatusBadRequest, "OrderId is required")
		return
	}

	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid OrderId")
		return
	}

	token := sadadCallbackToken(r)

	resCode := r.FormValue("ResCode")
	if resCode == "" {
		resCode = r.FormValue("resCode")
	}

	additionalParams := make(map[string]string)
	for k, v := range r.Form {
		switch k {
		case "Token", "token", "ResCode", "resCode", "OrderId", "order_id":
			continue
		}
		if len(v) > 0 {
			additionalParams[k] = v[0]
		}
	}

	grpcReq := &financialpb.HandleCallbackRequest{
		OrderId:          orderID,
		Token:            token,
		ResCode:          resCode,
		AdditionalParams: additionalParams,
	}

	resp, err := h.order.HandleCallback(contextWithAcceptLanguage(r), grpcReq)
	if err != nil {
		writeHandlerError(w, err, h.locale)
		return
	}

	http.Redirect(w, r, resp.RedirectUrl, http.StatusFound)
}

// GetStorePackages handles POST /api/store
func (h *HTTPFinancialHandler) GetStorePackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Codes []string `json:"codes"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if len(req.Codes) < 2 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"codes": "The codes field must contain at least 2 items",
		}, h.locale)
		return
	}

	for i, code := range req.Codes {
		if len(code) < 2 {
			helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
				"codes": fmt.Sprintf("The codes.%d field must be at least 2 characters", i),
			}, h.locale)
			return
		}
	}

	grpcReq := &financialpb.GetStorePackagesRequest{
		Codes: req.Codes,
	}

	resp, err := h.store.GetStorePackages(contextWithAcceptLanguage(r), grpcReq)
	if err != nil {
		writeHandlerError(w, err, h.locale)
		return
	}

	packages := make([]map[string]interface{}, 0, len(resp.Packages))
	for _, pkg := range resp.Packages {
		pkgData := map[string]interface{}{
			"id":        pkg.Id,
			"code":      pkg.Code,
			"asset":     pkg.Asset,
			"amount":    pkg.Amount,
			"unitPrice": pkg.UnitPrice,
		}
		if pkg.Image != nil && *pkg.Image != "" {
			pkgData["image"] = *pkg.Image
		} else {
			pkgData["image"] = nil
		}
		packages = append(packages, pkgData)
	}

	writeJSON(w, http.StatusOK, packages)
}
