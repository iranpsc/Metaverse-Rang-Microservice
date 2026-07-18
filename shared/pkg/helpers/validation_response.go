package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidationErrorResponse represents the validation error response format
type ValidationErrorResponse struct {
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// LocaleTranslations holds error message translations for different locales
type LocaleTranslations struct {
	Required            string
	Email               string
	Min                 string
	Max                 string
	Len                 string
	OneOf               string
	Unique              string
	Persian             string
	PersianAlpha        string
	PersianNum          string
	PersianAlphaNum     string
	IranianMobile       string
	IranianPostalCode   string
	IranianNationalCode string
	IranianSheba        string
	IranianBankCard     string
	Invalid             string
}

// translations holds locale-specific translations
var translations = map[string]LocaleTranslations{
	"en": {
		Required:            "The %s field is required",
		Email:               "The %s field must be a valid email address",
		Min:                 "The %s field must be at least %s characters",
		Max:                 "The %s field must not exceed %s characters",
		Len:                 "The %s field must be exactly %s characters",
		OneOf:               "The %s field must be one of: %s",
		Unique:              "The %s field must be unique",
		Persian:             "The %s field must contain only Persian characters",
		PersianAlpha:        "The %s field must contain only Persian alphabetic characters",
		PersianNum:          "The %s field must contain only Persian numbers",
		PersianAlphaNum:     "The %s field must contain only Persian alphanumeric characters",
		IranianMobile:       "The %s field must be a valid Iranian mobile number",
		IranianPostalCode:   "The %s field must be a valid Iranian postal code",
		IranianNationalCode: "The %s field must be a valid Iranian national code",
		IranianSheba:        "The %s field must be a valid Iranian Sheba (IBAN) number",
		IranianBankCard:     "The %s field must be a valid Iranian bank card number",
		Invalid:             "The %s field is invalid",
	},
	"fa": {
		Required:            "فیلد %s الزامی است",
		Email:               "فیلد %s باید یک آدرس ایمیل معتبر باشد",
		Min:                 "فیلد %s باید حداقل %s کاراکتر باشد",
		Max:                 "فیلد %s نباید بیشتر از %s کاراکتر باشد",
		Len:                 "فیلد %s باید دقیقاً %s کاراکتر باشد",
		OneOf:               "فیلد %s باید یکی از موارد زیر باشد: %s",
		Unique:              "فیلد %s باید یکتا باشد",
		Persian:             "فیلد %s باید فقط شامل کاراکترهای فارسی باشد",
		PersianAlpha:        "فیلد %s باید فقط شامل حروف فارسی باشد",
		PersianNum:          "فیلد %s باید فقط شامل اعداد فارسی باشد",
		PersianAlphaNum:     "فیلد %s باید فقط شامل حروف و اعداد فارسی باشد",
		IranianMobile:       "فیلد %s باید یک شماره موبایل ایرانی معتبر باشد",
		IranianPostalCode:   "فیلد %s باید یک کد پستی ایرانی معتبر باشد",
		IranianNationalCode: "فیلد %s باید یک کد ملی ایرانی معتبر باشد",
		IranianSheba:        "فیلد %s باید یک شماره شبا (IBAN) ایرانی معتبر باشد",
		IranianBankCard:     "فیلد %s باید یک شماره کارت بانکی ایرانی معتبر باشد",
		Invalid:             "فیلد %s نامعتبر است",
	},
}

// GetDefaultLocale returns the default locale
func GetDefaultLocale() string {
	return "en"
}

// GetLocaleTranslations returns translations for a given locale, or default locale if not found
func GetLocaleTranslations(locale string) LocaleTranslations {
	if t, ok := translations[locale]; ok {
		return t
	}
	return translations[GetDefaultLocale()]
}

// FormatValidationError formats a validator.FieldError into a localized error message
func FormatValidationError(fe validator.FieldError, locale string) string {
	t := GetLocaleTranslations(locale)
	fieldName := getFieldName(fe)

	switch fe.Tag() {
	case "required":
		return fmt.Sprintf(t.Required, fieldName)
	case "email":
		return fmt.Sprintf(t.Email, fieldName)
	case "min":
		return fmt.Sprintf(t.Min, fieldName, fe.Param())
	case "max":
		return fmt.Sprintf(t.Max, fieldName, fe.Param())
	case "len":
		return fmt.Sprintf(t.Len, fieldName, fe.Param())
	case "oneof":
		return fmt.Sprintf(t.OneOf, fieldName, fe.Param())
	case "unique":
		return fmt.Sprintf(t.Unique, fieldName)
	case "persian":
		return fmt.Sprintf(t.Persian, fieldName)
	case "persian_alpha":
		return fmt.Sprintf(t.PersianAlpha, fieldName)
	case "persian_num":
		return fmt.Sprintf(t.PersianNum, fieldName)
	case "persian_alpha_num":
		return fmt.Sprintf(t.PersianAlphaNum, fieldName)
	case "iranian_mobile":
		return fmt.Sprintf(t.IranianMobile, fieldName)
	case "iranian_postal_code":
		return fmt.Sprintf(t.IranianPostalCode, fieldName)
	case "iranian_national_code":
		return fmt.Sprintf(t.IranianNationalCode, fieldName)
	case "ir_sheba":
		return fmt.Sprintf(t.IranianSheba, fieldName)
	case "ir_bank_card_number":
		return fmt.Sprintf(t.IranianBankCard, fieldName)
	default:
		return fmt.Sprintf(t.Invalid, fieldName)
	}
}

// getFieldName extracts a human-readable field name from the FieldError
func getFieldName(fe validator.FieldError) string {
	fieldName := fe.Field()

	// Convert camelCase to space-separated words
	fieldName = strings.ToLower(fieldName)

	// Replace common field name patterns
	fieldName = strings.ReplaceAll(fieldName, "_", " ")

	return fieldName
}

// WriteValidationErrorResponse writes a validation error response in the specified format
// It accepts validator.ValidationErrors and formats them according to the locale
func WriteValidationErrorResponse(w http.ResponseWriter, validationErrors validator.ValidationErrors, locale string) {
	errors := make(map[string]string)
	var firstMessage string

	for i, err := range validationErrors {
		fieldName := err.Field()
		errorMessage := FormatValidationError(err, locale)

		errors[fieldName] = errorMessage

		// First error message becomes the main message
		if i == 0 {
			firstMessage = errorMessage
		}
	}

	response := ValidationErrorResponse{
		Message: firstMessage,
		Errors:  errors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(response)
}

// WriteValidationErrorResponseFromMap writes a validation error response from a map of field errors
// This is useful when you have custom validation errors not from go-playground validator
func WriteValidationErrorResponseFromMap(w http.ResponseWriter, fieldErrors map[string]string, locale string) {
	if len(fieldErrors) == 0 {
		// Fallback if no errors provided
		t := GetLocaleTranslations(locale)
		response := ValidationErrorResponse{
			Message: t.Invalid,
			Errors:  fieldErrors,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Get first error message
	var firstMessage string
	for _, msg := range fieldErrors {
		firstMessage = msg
		break
	}

	response := ValidationErrorResponse{
		Message: firstMessage,
		Errors:  fieldErrors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(response)
}

// WriteValidationErrorResponseFromString writes a validation error response from a single error message
// This creates a generic error response when you don't have field-specific errors
func WriteValidationErrorResponseFromString(w http.ResponseWriter, message string, locale string) {
	t := GetLocaleTranslations(locale)

	// If message is empty, use default invalid message
	if message == "" {
		message = t.Invalid
	}

	response := ValidationErrorResponse{
		Message: message,
		Errors:  make(map[string]string),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(response)
}
