package helpers

import (
	"encoding/json"
	"strings"
)

// ValidationFieldError represents a single field validation error
type ValidationFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorData holds structured validation error information
type ValidationErrorData struct {
	Fields map[string]string `json:"fields"`
}

// EncodeValidationError encodes field validation errors into a JSON string
// This can be embedded in gRPC error messages for structured error handling
func EncodeValidationError(fields map[string]string) string {
	if len(fields) == 0 {
		return ""
	}

	data := ValidationErrorData{
		Fields: fields,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		// Fallback: return first error message
		for _, msg := range fields {
			return msg
		}
		return "validation error"
	}

	return string(jsonData)
}

// DecodeValidationError decodes a JSON string into field validation errors
// Returns the fields map and a boolean indicating if decoding was successful
func DecodeValidationError(errorMsg string) (map[string]string, bool) {
	// Try to parse as JSON first
	var data ValidationErrorData
	if err := json.Unmarshal([]byte(errorMsg), &data); err == nil {
		if len(data.Fields) > 0 {
			return data.Fields, true
		}
	}

	// If not JSON, check if it's a simple error message that might map to a field
	// This handles cases where services return simple error messages
	errorMsgLower := strings.ToLower(errorMsg)

	// Common field mappings based on error message content
	fieldMappings := map[string][]string{
		"code":           {"code", "verification code", "otp", "verification"},
		"phone":          {"phone", "mobile", "telephone"},
		"time":           {"time", "duration", "minutes"},
		"email":          {"email", "e-mail"},
		"password":       {"password", "pass"},
		"name":           {"name", "username"},
		"fname":          {"fname", "first name", "firstname"},
		"lname":          {"lname", "last name", "lastname"},
		"melli_code":     {"melli code", "national code", "national_code"},
		"birthdate":      {"birthdate", "birth date", "date of birth"},
		"province":       {"province"},
		"gender":         {"gender"},
		"bank_name":      {"bank name", "bank_name"},
		"shaba_num":      {"shaba", "sheba", "iban"},
		"card_num":       {"card number", "card_num", "card"},
		"occupation":     {"occupation"},
		"education":      {"education"},
		"memory":         {"memory"},
		"loved_city":     {"loved city", "loved_city"},
		"loved_country":  {"loved country", "loved_country"},
		"loved_language": {"loved language", "loved_language"},
		"amount":         {"amount"},
		"asset":          {"asset"},
		"codes":          {"codes"},
	}

	// Try to find a matching field
	for field, keywords := range fieldMappings {
		for _, keyword := range keywords {
			if strings.Contains(errorMsgLower, keyword) {
				return map[string]string{field: errorMsg}, true
			}
		}
	}

	return nil, false
}

// FormatValidationErrorMessage creates a user-friendly error message from field errors
func FormatValidationErrorMessage(fields map[string]string, locale string) string {
	if len(fields) == 0 {
		return GetLocaleTranslations(locale).Invalid
	}

	// Return the first error message
	for _, msg := range fields {
		return msg
	}

	return GetLocaleTranslations(locale).Invalid
}

// CreateValidationError creates a validation error response from field errors
func CreateValidationError(field string, message string) map[string]string {
	return map[string]string{field: message}
}

// MergeValidationErrors merges multiple validation error maps
func MergeValidationErrors(errors ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, errs := range errors {
		for field, msg := range errs {
			result[field] = msg
		}
	}
	return result
}
