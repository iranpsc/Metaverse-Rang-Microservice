package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"metargb/grpc-gateway/internal/middleware"
	pb "metargb/shared/pb/auth"
	financialpb "metargb/shared/pb/financial"
	"metargb/shared/pkg/helpers"
)

func appendAcceptLanguage(ctx context.Context, r *http.Request) context.Context {
	al := r.Header.Get("Accept-Language")
	if al == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "accept-language", al, "grpcgateway-accept-language", al)
}

type FinancialHandler struct {
	orderClient financialpb.OrderServiceClient
	storeClient financialpb.StoreServiceClient
	authClient  pb.AuthServiceClient
	locale      string
}

func NewFinancialHandler(financialConn, authConn *grpc.ClientConn, locale string) *FinancialHandler {
	return &FinancialHandler{
		orderClient: financialpb.NewOrderServiceClient(financialConn),
		storeClient: financialpb.NewStoreServiceClient(financialConn),
		authClient:  pb.NewAuthServiceClient(authConn),
		locale:      locale,
	}
}

// CreateOrder handles POST /api/order
func (h *FinancialHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	userID := userCtx.UserID

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

	// Validate amount
	if req.Amount < 1 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"amount": "The amount field must be at least 1",
		}, h.locale)
		return
	}

	// Validate asset
	validAssets := map[string]bool{"psc": true, "irr": true, "red": true, "blue": true, "yellow": true}
	if !validAssets[req.Asset] {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"asset": "The selected asset is invalid",
		}, h.locale)
		return
	}

	grpcReq := &financialpb.CreateOrderRequest{
		UserId: userID,
		Amount: req.Amount,
		Asset:  req.Asset,
	}

	resp, err := h.orderClient.CreateOrder(appendAcceptLanguage(r.Context(), r), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"link": resp.Link,
	})
}

// HandleCallback handles POST /api/parsian/callback
func (h *FinancialHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse form data (Parsian sends form-encoded data)
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form data")
		return
	}

	// Extract OrderId and status
	orderIDStr := r.FormValue("OrderId")
	if orderIDStr == "" {
		writeError(w, http.StatusBadRequest, "OrderId is required")
		return
	}

	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid OrderId")
		return
	}

	statusStr := r.FormValue("status")
	status, err := strconv.ParseInt(statusStr, 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	tokenStr := r.FormValue("Token")
	token, err := strconv.ParseInt(tokenStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid Token")
		return
	}

	// Collect all additional parameters
	additionalParams := make(map[string]string)
	for k, v := range r.Form {
		if k != "OrderId" && k != "status" && k != "Token" {
			if len(v) > 0 {
				additionalParams[k] = v[0]
			}
		}
	}

	grpcReq := &financialpb.HandleCallbackRequest{
		OrderId:          orderID,
		Status:           int32(status),
		Token:            token,
		AdditionalParams: additionalParams,
	}

	resp, err := h.orderClient.HandleCallback(appendAcceptLanguage(r.Context(), r), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Redirect to frontend URL
	http.Redirect(w, r, resp.RedirectUrl, http.StatusFound)
}

// GetStorePackages handles POST /api/store
func (h *FinancialHandler) GetStorePackages(w http.ResponseWriter, r *http.Request) {
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

	// Validation: at least 2 codes required
	if len(req.Codes) < 2 {
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"codes": "The codes field must contain at least 2 items",
		}, h.locale)
		return
	}

	// Validate each code
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

	resp, err := h.storeClient.GetStorePackages(appendAcceptLanguage(r.Context(), r), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Convert to JSON format matching Laravel PackageResource
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

// Helper function (deprecated - use helpers.WriteValidationErrorResponseFromMap instead)
// This function is kept for backward compatibility but should not be used in new code
func writeValidationErrorWithErrors(w http.ResponseWriter, message string, errors map[string][]string) {
	// Convert map[string][]string to map[string]string (take first error for each field)
	errorsMap := make(map[string]string)
	for field, errs := range errors {
		if len(errs) > 0 {
			errorsMap[field] = errs[0]
		}
	}
	helpers.WriteValidationErrorResponseFromMap(w, errorsMap, "en")
}
