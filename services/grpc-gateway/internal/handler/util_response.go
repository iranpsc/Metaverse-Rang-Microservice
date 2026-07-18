// Package handler provides HTTP handlers for the gRPC gateway service.
package handler

import (
	"encoding/json"
	"net/http"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/shared/pkg/helpers"
)

// writeJSON writes a JSON response, wrapping it in a "data" field unless:
// 1. The data is already wrapped (has a "data" key at top level)
// 2. The data is an error response (has "error" key)
// 3. skipWrap is true (for special cases like /auth/redirect)
func writeJSON(w http.ResponseWriter, status int, data interface{}, skipWrap ...bool) {
	shouldSkipWrap := len(skipWrap) > 0 && skipWrap[0]

	// Handle nil data
	if data == nil {
		data = map[string]interface{}{}
	}

	// Check if we should wrap the response
	if !shouldSkipWrap {
		// Use reflection to check the type
		dataType := reflect.TypeOf(data)
		dataKind := dataType.Kind()

		// Check if data is a map (objects)
		if dataKind == reflect.Map {
			if dataMap, ok := data.(map[string]interface{}); ok {
				// Already has "data" key - don't wrap again
				if _, hasData := dataMap["data"]; hasData {
					shouldSkipWrap = true
				}
				// Has "error" key - don't wrap error responses
				if _, hasError := dataMap["error"]; hasError {
					shouldSkipWrap = true
				}
				// Has "message" and "errors" keys - validation error, don't wrap
				if _, hasMessage := dataMap["message"]; hasMessage {
					if _, hasErrors := dataMap["errors"]; hasErrors {
						shouldSkipWrap = true
					}
				}
			} else if dataMap, ok := data.(map[string]string); ok {
				// Check for error or special responses in map[string]string
				if _, hasError := dataMap["error"]; hasError {
					shouldSkipWrap = true
				}
				// Laravel-style bare responses: {"url": "..."} or {"link": "..."}
				if _, hasURL := dataMap["url"]; hasURL {
					shouldSkipWrap = true
				}
				if _, hasLink := dataMap["link"]; hasLink {
					shouldSkipWrap = true
				}
			}
		}
		// Arrays, slices, and other non-map types (including []map[string]interface{})
		// will be wrapped in the "data" field below

		// Wrap in data field if not skipping
		if !shouldSkipWrap {
			data = map[string]interface{}{
				"data": data,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeGRPCError(w http.ResponseWriter, err error) {
	writeGRPCErrorWithLocale(w, err, "en")
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
		// Try to decode structured validation errors from service
		errorMsg := st.Message()
		if fields, decoded := helpers.DecodeValidationError(errorMsg); decoded {
			// Use decoded field errors
			helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
		} else {
			// Fallback: try to map error message to fields
			if fields, mapped := helpers.DecodeValidationError(errorMsg); mapped {
				helpers.WriteValidationErrorResponseFromMap(w, fields, locale)
			} else {
				// Last resort: return as generic validation error
				helpers.WriteValidationErrorResponseFromString(w, errorMsg, locale)
			}
		}
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, st.Message())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, st.Message())
	case codes.FailedPrecondition:
		writeError(w, http.StatusPreconditionFailed, st.Message())
	case codes.Unavailable:
		// Service unavailable - likely connection issue
		writeError(w, http.StatusServiceUnavailable, "service temporarily unavailable: "+st.Message())
	default:
		writeError(w, http.StatusInternalServerError, st.Message())
	}
}
