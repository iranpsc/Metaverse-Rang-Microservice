package handler

import (
	"net/http"
	"strconv"

	"google.golang.org/grpc"

	"metargb/grpc-gateway/internal/middleware"
	commercialpb "metargb/shared/pb/commercial"
)

// CommercialHandler exposes Laravel-aligned wallet and transaction routes via commercial-service gRPC.
type CommercialHandler struct {
	walletClient commercialpb.WalletServiceClient
	txClient     commercialpb.TransactionServiceClient
}

// NewCommercialHandler builds a handler backed by commercial-service.
func NewCommercialHandler(conn *grpc.ClientConn) *CommercialHandler {
	return &CommercialHandler{
		walletClient: commercialpb.NewWalletServiceClient(conn),
		txClient:     commercialpb.NewTransactionServiceClient(conn),
	}
}

// GetCurrentUserWallet handles GET /api/user/wallet (auth user), aligned with Laravel WalletResource.
func (h *CommercialHandler) GetCurrentUserWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthenticated")
		return
	}

	ctx := appendAcceptLanguage(middleware.ContextWithAuthFromRequest(r), r)
	resp, err := h.walletClient.GetWallet(ctx, &commercialpb.GetWalletRequest{UserId: userCtx.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"psc":          resp.Psc,
		"irr":          resp.Irr,
		"red":          resp.Red,
		"blue":         resp.Blue,
		"yellow":       resp.Yellow,
		"satisfaction": resp.Satisfaction,
		"effect":       resp.Effect,
	})
}

// ListTransactions handles GET /api/user/transactions (simple list; filters forwarded to gRPC).
func (h *CommercialHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthenticated")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 {
		perPage = 15
	}

	req := &commercialpb.ListTransactionsRequest{
		UserId:        userCtx.UserID,
		Page:          int32(page),
		PerPage:       int32(perPage),
		Search:        q.Get("search"),
		StartDateTime: q.Get("start_date_time"),
		EndDateTime:   q.Get("end_date_time"),
		Action:        q.Get("action"),
		Asset:         q.Get("asset"),
		Type:          q.Get("type"),
	}

	for _, s := range q["status"] {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			continue
		}
		req.Status = append(req.Status, int32(v))
	}

	ctx := appendAcceptLanguage(middleware.ContextWithAuthFromRequest(r), r)
	resp, err := h.txClient.ListTransactions(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	items := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, t := range resp.Transactions {
		items = append(items, map[string]interface{}{
			"id":     t.Id,
			"type":   t.Type,
			"asset":  t.Asset,
			"amount": t.Amount,
			"action": t.Action,
			"status": t.Status,
			"date":   t.Date,
			"time":   t.Time,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": items,
		"meta": map[string]interface{}{
			"current_page": resp.CurrentPage,
			"has_more":     resp.HasMorePages,
		},
	})
}

// GetLatestTransaction handles GET /api/user/transactions/latest — mirrors Laravel LatestTransactionResource shape where possible.
func (h *CommercialHandler) GetLatestTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthenticated")
		return
	}

	ctx := appendAcceptLanguage(middleware.ContextWithAuthFromRequest(r), r)
	resp, err := h.txClient.GetLatestTransaction(ctx, &commercialpb.GetLatestTransactionRequest{UserId: userCtx.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	out := map[string]interface{}{}
	if resp.LatestTransaction != nil {
		t := resp.LatestTransaction
		out["id"] = t.Id
		out["amount"] = t.Amount
		out["status"] = t.Status
		out["product"] = t.Asset
		out["count"] = t.Amount
	}
	if resp.LatestPayment != nil {
		p := resp.LatestPayment
		out["payment_info"] = map[string]interface{}{
			"ref_id": p.RefId,
		}
	}
	if resp.LatestOrder != nil {
		o := resp.LatestOrder
		out["product"] = o.Asset
		out["count"] = o.Amount
	}

	writeJSON(w, http.StatusOK, out)
}
