package handler

import (
	"io"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/grpc-gateway/internal/middleware"
	pb "metargb/shared/pb/auth"
	"metargb/shared/pkg/helpers"
)

type WalletHandler struct {
	walletClient pb.WalletConnectionServiceClient
	locale       string
}

func NewWalletHandler(conn *grpc.ClientConn, locale string) *WalletHandler {
	return &WalletHandler{
		walletClient: pb.NewWalletConnectionServiceClient(conn),
		locale:       locale,
	}
}

// GetLinkNonce handles GET /api/wallet/link/nonce
func (h *WalletHandler) GetLinkNonce(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Address string `json:"address" query:"address"`
	}
	if err := decodeRequest(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	resp, err := h.walletClient.GetLinkNonce(r.Context(), &pb.GetWalletLinkNonceRequest{
		UserId:  userCtx.UserID,
		Address: req.Address,
	})
	if err != nil {
		h.writeWalletGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"nonce": resp.Nonce}, true)
}

// LinkWallet handles POST /api/wallet/link
func (h *WalletHandler) LinkWallet(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Address   string `json:"address"`
		Signature string `json:"signature"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	resp, err := h.walletClient.LinkWallet(r.Context(), &pb.LinkWalletRequest{
		UserId:    userCtx.UserID,
		Address:   req.Address,
		Signature: req.Signature,
		Ip:        getClientIP(r),
	})
	if err != nil {
		h.writeWalletGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message":        resp.Message,
		"wallet_address": resp.WalletAddress,
	}, true)
}

// GetSecurityNonce handles GET /api/wallet/security/nonce
func (h *WalletHandler) GetSecurityNonce(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Address string `json:"address" query:"address"`
	}
	if err := decodeRequest(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	resp, err := h.walletClient.GetSecurityNonce(r.Context(), &pb.GetWalletSecurityNonceRequest{
		UserId:  userCtx.UserID,
		Address: req.Address,
	})
	if err != nil {
		h.writeWalletGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"nonce": resp.Nonce}, true)
}

// VerifySecuritySignature handles POST /api/wallet/security/verify
func (h *WalletHandler) VerifySecuritySignature(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Address   string        `json:"address"`
		Signature string        `json:"signature"`
		Duration  flexibleInt32 `json:"duration"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	resp, err := h.walletClient.VerifySecuritySignature(r.Context(), &pb.VerifyWalletSecuritySignatureRequest{
		UserId:    userCtx.UserID,
		Address:   req.Address,
		Signature: req.Signature,
		Duration:  req.Duration.Int32(),
		Ip:        getClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.writeWalletGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": resp.Message,
		"until":   resp.Until,
	}, true)
}

func (h *WalletHandler) writeWalletGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.Unauthenticated:
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": st.Message()}, true)
	case codes.PermissionDenied:
		writeJSON(w, http.StatusForbidden, map[string]string{"message": st.Message()}, true)
	case codes.InvalidArgument:
		errorMsg := st.Message()
		if fields, decoded := helpers.DecodeValidationError(errorMsg); decoded {
			helpers.WriteValidationErrorResponseFromMap(w, fields, h.locale)
		} else if fields, mapped := helpers.DecodeValidationError(errorMsg); mapped {
			helpers.WriteValidationErrorResponseFromMap(w, fields, h.locale)
		} else {
			helpers.WriteValidationErrorResponseFromString(w, errorMsg, h.locale)
		}
	case codes.FailedPrecondition:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": st.Message()}, true)
	case codes.NotFound:
		writeJSON(w, http.StatusNotFound, map[string]string{"message": st.Message()}, true)
	default:
		writeError(w, http.StatusInternalServerError, st.Message())
	}
}
