package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	"metarang/financial-service/internal/constants"
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
	if !requestMethodAllowed(r, http.MethodPost) {
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
		writeRequestBodyError(w, err)
		return
	}

	if req.Amount < constants.MinOrderAmount {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"amount": fmt.Sprintf("The amount field must be at least %d", constants.MinOrderAmount),
		}, h.locale)
		return
	}

	if !allowedOrderAsset(req.Asset) {
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
	token := firstFormValue(r, "Token", "token")
	return strings.ReplaceAll(strings.TrimSpace(token), " ", "+")
}

var (
	errCallbackForm    = errors.New("failed to parse form data")
	errOrderIDRequired = errors.New("OrderId is required")
	errInvalidOrderID  = errors.New("invalid OrderId")
)

type sadadCallbackForm struct {
	orderID          uint64
	token            string
	resCode          string
	additionalParams map[string]string
}

func parseSadadCallback(r *http.Request) (sadadCallbackForm, error) {
	if err := r.ParseForm(); err != nil {
		return sadadCallbackForm{}, errCallbackForm
	}

	// OrderId comes only from the IPG POST body. Query-string order_id is ignored
	// so a tampered ReturnUrl cannot select a different order.
	orderIDStr := r.PostFormValue("OrderId")
	if orderIDStr == "" {
		return sadadCallbackForm{}, errOrderIDRequired
	}
	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		return sadadCallbackForm{}, errInvalidOrderID
	}

	return sadadCallbackForm{
		orderID:          orderID,
		token:            sadadCallbackToken(r),
		resCode:          firstFormValue(r, "ResCode", "resCode"),
		additionalParams: callbackForwardedParams(r),
	}, nil
}

func firstFormValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := r.FormValue(key); value != "" {
			return value
		}
	}
	return ""
}

var callbackFieldsNotForwarded = map[string]struct{}{
	"Token":    {},
	"token":    {},
	"ResCode":  {},
	"resCode":  {},
	"OrderId":  {},
	"order_id": {},
}

func callbackForwardedParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	for key, values := range r.Form {
		if _, skip := callbackFieldsNotForwarded[key]; skip || len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}
	return params
}

func requestMethodAllowed(r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return true
		}
	}
	return false
}

func writeRequestBodyError(w http.ResponseWriter, err error) {
	message := "invalid request body"
	if err == io.EOF {
		message = "request body is required"
	}
	writeError(w, http.StatusBadRequest, message)
}

func allowedOrderAsset(asset string) bool {
	for _, allowed := range constants.ValidOrderAssets {
		if asset == allowed {
			return true
		}
	}
	return false
}

func storePackageJSON(pkg *financialpb.Package) map[string]interface{} {
	var image interface{}
	if pkg.Image != nil && *pkg.Image != "" {
		image = *pkg.Image
	}
	return map[string]interface{}{
		"id":        pkg.Id,
		"code":      pkg.Code,
		"asset":     pkg.Asset,
		"amount":    pkg.Amount,
		"unitPrice": pkg.UnitPrice,
		"image":     image,
	}
}

// HandleCallback handles GET|POST /api/order/callback and /api/payment/callback
func (h *HTTPFinancialHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if !requestMethodAllowed(r, http.MethodGet, http.MethodPost) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	callback, err := parseSadadCallback(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	grpcReq := &financialpb.HandleCallbackRequest{
		OrderId:          callback.orderID,
		Token:            callback.token,
		ResCode:          callback.resCode,
		AdditionalParams: callback.additionalParams,
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
	if !requestMethodAllowed(r, http.MethodPost) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Codes []string `json:"codes"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		writeRequestBodyError(w, err)
		return
	}

	if len(req.Codes) < constants.MinStoreCodes {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"codes": fmt.Sprintf("The codes field must contain at least %d items", constants.MinStoreCodes),
		}, h.locale)
		return
	}

	for i, code := range req.Codes {
		if len(code) < constants.MinStoreCodeLength {
			helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
				"codes": fmt.Sprintf("The codes.%d field must be at least %d characters", i, constants.MinStoreCodeLength),
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
		packages = append(packages, storePackageJSON(pkg))
	}

	writeJSON(w, http.StatusOK, packages)
}
