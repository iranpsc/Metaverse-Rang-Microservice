package handler

import (
	"net/http"

	"metargb/shared/pkg/helpers"
)

// writeValidationErrorWithLocale writes a validation error response with locale support
func writeValidationErrorWithLocale(w http.ResponseWriter, message string, locale string) {
	helpers.WriteValidationErrorResponseFromString(w, message, locale)
}

// writeValidationError writes a validation error response using default locale (en)
// This is kept for backward compatibility
func writeValidationError(w http.ResponseWriter, message string) {
	writeValidationErrorWithLocale(w, message, "en")
}
