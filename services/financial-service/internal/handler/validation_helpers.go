package handler

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"metargb/shared/pkg/helpers"
)

const defaultLocale = "en"

// GetLocaleFromContext reads Accept-Language from incoming gRPC metadata (set by grpc-gateway).
// Returns "fa" when the primary tag starts with fa, otherwise "en".
func GetLocaleFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return defaultLocale
	}
	vals := md.Get("grpcgateway-accept-language")
	if len(vals) == 0 {
		vals = md.Get("accept-language")
	}
	if len(vals) == 0 {
		vals = md.Get("Accept-Language")
	}
	if len(vals) == 0 {
		return defaultLocale
	}
	primary := strings.TrimSpace(strings.Split(vals[0], ",")[0])
	primary = strings.TrimSpace(strings.Split(primary, ";")[0])
	if strings.HasPrefix(strings.ToLower(primary), "fa") {
		return "fa"
	}
	return defaultLocale
}

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

// validateOneOf validates that a string value is one of the allowed values
func validateOneOf(fieldName string, value string, allowedValues []string, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	if value == "" {
		return validationErrors // Let validateRequired handle empty values
	}

	valid := false
	for _, allowed := range allowedValues {
		if value == allowed {
			valid = true
			break
		}
	}

	if !valid {
		validationErrors[fieldName] = fmt.Sprintf(t.Invalid, fieldName)
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
