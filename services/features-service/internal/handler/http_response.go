package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/shared/pkg/helpers"
)

// writeJSON preserves the grpc-gateway contract by wrapping bare values in data.
func writeJSON(w http.ResponseWriter, statusCode int, value interface{}, skipWrap ...bool) {
	skip := len(skipWrap) > 0 && skipWrap[0]
	if value == nil {
		value = map[string]interface{}{}
	}
	if !skip {
		kind := reflect.TypeOf(value).Kind()
		if kind == reflect.Map {
			switch data := value.(type) {
			case map[string]interface{}:
				_, hasData := data["data"]
				_, hasError := data["error"]
				_, hasMessage := data["message"]
				_, hasErrors := data["errors"]
				skip = hasData || hasError || (hasMessage && hasErrors)
			case map[string]string:
				_, hasError := data["error"]
				_, hasURL := data["url"]
				_, hasLink := data["link"]
				skip = hasError || hasURL || hasLink
			}
		}
		if !skip {
			value = map[string]interface{}{"data": value}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeValidationError(w http.ResponseWriter, message string) {
	writeValidationErrorWithLocale(w, message, GetProjectLocale())
}

func writeValidationErrorWithLocale(w http.ResponseWriter, message, locale string) {
	helpers.WriteValidationErrorResponseFromString(w, message, locale)
}

func writeGRPCError(w http.ResponseWriter, err error) {
	writeGRPCErrorWithLocale(w, err, GetProjectLocale())
}

func writeGRPCErrorWithLocale(w http.ResponseWriter, err error, locale string) {
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
		if fields, decoded := helpers.DecodeValidationError(st.Message()); decoded {
			helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
		} else {
			helpers.WriteValidationErrorResponseFromString(w, st.Message(), locale)
		}
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

func publicBaseURL(r *http.Request) string {
	if base := strings.TrimSuffix(os.Getenv("APP_URL"), "/"); base != "" {
		return base
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}

func requestPath(r *http.Request) string { return publicBaseURL(r) + r.URL.Path }

func buildSimplePaginationLinks(r *http.Request, page int32, hasMore bool) map[string]interface{} {
	base := requestPath(r)
	query := r.URL.Query()
	query.Set("page", "1")
	links := map[string]interface{}{"first": base + "?" + query.Encode(), "last": nil, "prev": nil, "next": nil}
	if page > 1 {
		query.Set("page", strconv.FormatInt(int64(page-1), 10))
		links["prev"] = base + "?" + query.Encode()
	}
	if hasMore {
		query.Set("page", strconv.FormatInt(int64(page+1), 10))
		links["next"] = base + "?" + query.Encode()
	}
	return links
}
