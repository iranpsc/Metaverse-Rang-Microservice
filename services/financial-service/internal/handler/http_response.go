package handler

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/shared/pkg/helpers"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}, skipWrap ...bool) {
	if data == nil {
		data = map[string]interface{}{}
	}
	if !(len(skipWrap) > 0 && skipWrap[0]) && !payloadAlreadyShaped(data) {
		data = map[string]interface{}{"data": data}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func payloadAlreadyShaped(data interface{}) bool {
	switch payload := data.(type) {
	case map[string]interface{}:
		if _, ok := payload["data"]; ok {
			return true
		}
		if _, ok := payload["error"]; ok {
			return true
		}
		_, hasMessage := payload["message"]
		_, hasErrors := payload["errors"]
		return hasMessage && hasErrors
	case map[string]string:
		if _, ok := payload["error"]; ok {
			return true
		}
		if _, ok := payload["url"]; ok {
			return true
		}
		_, hasLink := payload["link"]
		return hasLink
	default:
		return false
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeHandlerError(w http.ResponseWriter, err error, locale string) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, st.Message())
	case codes.NotFound:
		writeError(w, http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		if fields, ok := helpers.DecodeValidationError(st.Message()); ok {
			helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
			return
		}
		helpers.WriteValidationErrorResponseFromString(w, st.Message(), locale)
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, st.Message())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, st.Message())
	case codes.FailedPrecondition:
		writeError(w, http.StatusPreconditionFailed, st.Message())
	case codes.Unavailable:
		writeError(w, http.StatusServiceUnavailable, "service temporarily unavailable: "+st.Message())
	default:
		writeError(w, http.StatusInternalServerError, st.Message())
	}
}
