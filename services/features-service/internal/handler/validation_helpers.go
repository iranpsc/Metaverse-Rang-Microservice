package handler

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/shared/pkg/helpers"
)

// returnValidationError returns a gRPC InvalidArgument error with encoded validation fields
func returnValidationError(fields map[string]string) error {
	encodedError := helpers.EncodeValidationError(fields)
	return status.Errorf(codes.InvalidArgument, "%s", encodedError)
}

// validateRequired validates that a field is not empty/zero
func validateRequired(fieldName string, value interface{}, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	switch v := value.(type) {
	case string:
		if v == "" {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	// Separate cases: with multiple types in one case, v is still interface{} per Go spec,
	// so numeric zero checks would not run correctly.
	case uint64:
		if v == 0 {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	case uint32:
		if v == 0 {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	case int64:
		if v == 0 {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	case int32:
		if v == 0 {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	}

	return validationErrors
}

// validateOneOf validates that a value is one of the allowed values
func validateOneOf(fieldName string, value string, allowed []string, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	valid := false
	for _, allowedValue := range allowed {
		if value == allowedValue {
			valid = true
			break
		}
	}

	if !valid {
		allowedStr := ""
		for i, v := range allowed {
			if i > 0 {
				allowedStr += ", "
			}
			allowedStr += v
		}
		validationErrors[fieldName] = fmt.Sprintf(t.OneOf, fieldName, allowedStr)
	}

	return validationErrors
}

// validateMin validates that a numeric value is at least the minimum
func validateMin(fieldName string, value int64, min int64, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	if value < min {
		validationErrors[fieldName] = fmt.Sprintf(t.Min, fieldName, fmt.Sprintf("%d", min))
	}

	return validationErrors
}

// validateMinLength validates that a string has at least the minimum length
func validateMinLength(fieldName string, value string, minLength int, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	if len(value) < minLength {
		validationErrors[fieldName] = fmt.Sprintf(t.Min, fieldName, fmt.Sprintf("%d", minLength))
	}

	return validationErrors
}

// mergeValidationErrors merges multiple validation error maps
func mergeValidationErrors(errors ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, errs := range errors {
		for field, msg := range errs {
			result[field] = msg
		}
	}
	return result
}
