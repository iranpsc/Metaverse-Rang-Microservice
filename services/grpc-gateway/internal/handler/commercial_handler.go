package handler

import (
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"metarang/grpc-gateway/internal/middleware"
	commercialpb "metarang/shared/pb/commercial"
	"metarang/shared/pkg/helpers"
)

type CommercialHandler struct {
	transactionClient commercialpb.TransactionServiceClient
	walletClient      commercialpb.WalletServiceClient
	locale            string
}

func NewCommercialHandler(commercialConn *grpc.ClientConn, locale string) *CommercialHandler {
	return &CommercialHandler{
		transactionClient: commercialpb.NewTransactionServiceClient(commercialConn),
		walletClient:      commercialpb.NewWalletServiceClient(commercialConn),
		locale:            locale,
	}
}

// GetCurrentUserWallet handles GET /api/user/wallet (Laravel WalletController@index)
func (h *CommercialHandler) GetCurrentUserWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resp, err := h.walletClient.GetWallet(r.Context(), &commercialpb.GetWalletRequest{UserId: userCtx.UserID})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Laravel returns wallet fields as strings.
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

// ListTransactions handles GET /api/user/transactions (Laravel TransactionController@index)
func (h *CommercialHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	query := r.URL.Query()

	page := int32(1)
	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	perPage := int32(15)
	if perPageStr := query.Get("per_page"); perPageStr != "" {
		if p, err := strconv.ParseInt(perPageStr, 10, 32); err == nil && p > 0 {
			perPage = int32(p)
		}
	}

	grpcReq := &commercialpb.ListTransactionsRequest{
		UserId:        userCtx.UserID,
		Page:          page,
		PerPage:       perPage,
		Search:        query.Get("search"),
		StartDateTime: query.Get("start_date_time"),
		EndDateTime:   query.Get("end_date_time"),
		Action:        query.Get("action"),
		Asset:         query.Get("asset"),
		Type:          query.Get("type"),
		Status:        parseStatusQueryParams(query),
	}

	resp, err := h.transactionClient.ListTransactions(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		transactions = append(transactions, map[string]interface{}{
			"id":     tx.Id,
			"type":   tx.Type,
			"asset":  tx.Asset,
			"amount": tx.Amount,
			"action": tx.Action,
			"status": tx.Status,
			"date":   tx.Date,
			"time":   tx.Time,
		})
	}

	currentPage := page
	if resp.CurrentPage > 0 {
		currentPage = resp.CurrentPage
	}

	response := map[string]interface{}{
		"data":  transactions,
		"links": buildSimplePaginationLinks(r, currentPage, resp.HasMorePages),
	}

	itemCount := len(transactions)
	var from interface{}
	var to interface{}
	if itemCount > 0 {
		fromVal := int((currentPage-1)*perPage) + 1
		from = fromVal
		to = fromVal + itemCount - 1
	}

	response["meta"] = map[string]interface{}{
		"current_page": currentPage,
		"from":         from,
		"path":         requestPath(r),
		"per_page":     perPage,
		"to":           to,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetLatestTransaction handles GET /api/user/transactions/latest (Laravel TransactionController@latestTransaction)
func (h *CommercialHandler) GetLatestTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resp, err := h.transactionClient.GetLatestTransaction(r.Context(), &commercialpb.GetLatestTransactionRequest{
		UserId: userCtx.UserID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	data := map[string]interface{}{}
	if resp.LatestTransaction != nil {
		data["id"] = resp.LatestTransaction.Id
		data["amount"] = resp.LatestTransaction.Amount
		data["status"] = resp.LatestTransaction.Status
		data["asset"] = resp.LatestTransaction.Asset

		date := ""
		timeValue := ""
		if resp.LatestTransaction.CreatedAt != nil {
			createdAt := resp.LatestTransaction.CreatedAt.AsTime()
			date = helpers.FormatJalaliDate(createdAt)
			timeValue = helpers.FormatJalaliTime(createdAt)
		}
		data["date"] = date
		data["time"] = timeValue
	}

	if resp.LatestPayment != nil {
		paymentInfo := map[string]interface{}{
			"ref_id": resp.LatestPayment.RefId,
		}
		if resp.LatestPayment.CreatedAt != nil {
			createdAt := resp.LatestPayment.CreatedAt.AsTime()
			paymentInfo["date"] = helpers.FormatJalaliDate(createdAt)
			paymentInfo["hour"] = helpers.FormatJalaliTime(createdAt)
		}
		data["payment_info"] = paymentInfo
	}

	if resp.LatestOrder != nil {
		data["product"] = resp.LatestOrder.Asset
		data["count"] = resp.LatestOrder.Amount
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

func parseStatusQueryParams(query map[string][]string) []int32 {
	statusValues := parseIndexedQueryArray(query, "status")
	if len(statusValues) == 0 {
		if values, ok := query["status"]; ok {
			statusValues = values
		} else if values, ok := query["status[]"]; ok {
			statusValues = values
		}
	}

	statuses := make([]int32, 0, len(statusValues))
	for _, value := range statusValues {
		if value == "" {
			continue
		}
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if status, err := strconv.ParseInt(part, 10, 32); err == nil {
				statuses = append(statuses, int32(status))
			}
		}
	}

	return statuses
}
