package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"metarang/auth-service/internal/middleware"
	pb "metarang/shared/pb/auth"
	"metarang/shared/pkg/helpers"
)

// SendMobileChangeCode handles POST /api/mobile/send
func (h *HTTPAuthHandler) SendMobileChangeCode(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Mobile string `json:"mobile" form:"mobile"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	mobile := strings.TrimSpace(helpers.NormalizePersianNumbers(req.Mobile))
	if mobile == "" {
		t := helpers.GetLocaleTranslations(h.locale)
		helpers.WriteValidationErrorResponseFromMap(w, map[string]string{
			"mobile": fmt.Sprintf(t.Required, "mobile"),
		}, h.locale)
		return
	}

	_, err = h.authClient.SendMobileChangeCode(r.Context(), &pb.SendMobileChangeCodeRequest{
		UserId: userCtx.UserID,
		Mobile: mobile,
	})
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "OTP sent successfully",
	})
}

// VerifyMobileChange handles POST /api/mobile/verify
func (h *HTTPAuthHandler) VerifyMobileChange(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Code flexibleString `json:"code" form:"code"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	code := strings.TrimSpace(helpers.NormalizePersianNumbers(req.Code.String()))
	_, err = h.authClient.VerifyMobileChange(r.Context(), &pb.VerifyMobileChangeRequest{
		UserId:    userCtx.UserID,
		Code:      code,
		Ip:        getClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.writeGRPCErrorLocale(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "mobile number updated successfully",
	})
}
