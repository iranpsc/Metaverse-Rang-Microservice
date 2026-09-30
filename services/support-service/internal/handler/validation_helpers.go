package handler

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/shared/pkg/helpers"
)

// returnValidationError returns a gRPC InvalidArgument error with encoded validation fields
func returnValidationError(fields map[string]string) error {
	encodedError := helpers.EncodeValidationError(fields)
	return status.Errorf(codes.InvalidArgument, "%s", encodedError)
}

// ValidateRequired validates that a field is not empty/zero.
func ValidateRequired(fieldName string, value interface{}, locale string) map[string]string {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	switch v := value.(type) {
	case string:
		if v == "" {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
	case uint64, uint32, int64, int32:
		if v == 0 {
			validationErrors[fieldName] = fmt.Sprintf(t.Required, fieldName)
		}
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

// MapServiceError maps domain / repository errors to gRPC status codes.
func MapServiceError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "unauthorized"):
		return status.Errorf(codes.PermissionDenied, "%s", msg)
	case strings.Contains(lower, "must not have more than"):
		return status.Errorf(codes.InvalidArgument, "%s", msg)
	case strings.Contains(lower, "not found"):
		return status.Errorf(codes.NotFound, "%s", msg)
	case strings.Contains(lower, "cannot respond"),
		strings.Contains(lower, "already closed"):
		return status.Errorf(codes.FailedPrecondition, "%s", msg)
	default:
		return status.Errorf(codes.Internal, "%s", msg)
	}
}

var validReportSubjects = map[string]struct{}{
	"displayError":  {},
	"spellingError": {},
	"codingError":   {},
	"FPSError":      {},
	"disrespect":    {},
}

// ValidateReportSubject validates report subject values.
func ValidateReportSubject(subject, locale string) map[string]string {
	if subject == "" {
		return ValidateRequired("subject", "", locale)
	}
	if _, ok := validReportSubjects[subject]; !ok {
		t := helpers.GetLocaleTranslations(locale)
		return map[string]string{"subject": fmt.Sprintf(t.Invalid, "subject")}
	}
	return nil
}

// ValidateMaxLen validates maximum string length.
func ValidateMaxLen(field, val string, max int, locale string) map[string]string {
	if len(val) > max {
		t := helpers.GetLocaleTranslations(locale)
		return map[string]string{field: fmt.Sprintf(t.Max, field, fmt.Sprintf("%d", max))}
	}
	return nil
}
