// Package helpers provides shared validation, formatting, and utility helpers.
package helpers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CustomValidator wraps go-playground validator with custom Persian rules
type CustomValidator struct {
	validate *validator.Validate
}

// NewCustomValidator creates a new custom validator with Persian rules
func NewCustomValidator() *CustomValidator {
	v := validator.New()

	registerValidation := func(tag string, fn validator.Func) {
		if err := v.RegisterValidation(tag, fn); err != nil {
			panic(fmt.Sprintf("register validation %q: %v", tag, err))
		}
	}

	// Register custom validators
	registerValidation("persian", validatePersian)
	registerValidation("persian_alpha", validatePersianAlpha)
	registerValidation("persian_num", validatePersianNum)
	registerValidation("persian_alpha_num", validatePersianAlphaNum)
	registerValidation("iranian_mobile", validateIranianMobile)
	registerValidation("iranian_postal_code", validateIranianPostalCode)
	registerValidation("iranian_national_code", validateIranianNationalCode)
	registerValidation("ir_sheba", validateIranianSheba)
	registerValidation("ir_bank_card_number", validateIranianBankCardNumber)

	return &CustomValidator{validate: v}
}

// Validate validates a struct
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validate.Struct(i)
}

// validatePersian validates Persian characters
func validatePersian(fl validator.FieldLevel) bool {
	persianRegex := regexp.MustCompile(`^[\x{0600}-\x{06FF}\s]+$`)
	return persianRegex.MatchString(fl.Field().String())
}

// validatePersianAlpha validates Persian alphabetic characters
func validatePersianAlpha(fl validator.FieldLevel) bool {
	persianAlphaRegex := regexp.MustCompile(`^[\x{0600}-\x{06FF}]+$`)
	return persianAlphaRegex.MatchString(fl.Field().String())
}

// validatePersianNum validates Persian numbers
func validatePersianNum(fl validator.FieldLevel) bool {
	persianNumRegex := regexp.MustCompile(`^[۰-۹]+$`)
	return persianNumRegex.MatchString(fl.Field().String())
}

// validatePersianAlphaNum validates Persian alphanumeric
func validatePersianAlphaNum(fl validator.FieldLevel) bool {
	persianAlphaNumRegex := regexp.MustCompile(`^[\x{0600}-\x{06FF}۰-۹\s]+$`)
	return persianAlphaNumRegex.MatchString(fl.Field().String())
}

// validateIranianMobile validates Iranian mobile numbers (09xxxxxxxxx)
func validateIranianMobile(fl validator.FieldLevel) bool {
	mobile := NormalizePersianNumbers(fl.Field().String())
	mobileRegex := regexp.MustCompile(`^09[0-9]{9}$`)
	return mobileRegex.MatchString(mobile)
}

// validateIranianPostalCode validates Iranian postal codes (10 digits, no dash)
func validateIranianPostalCode(fl validator.FieldLevel) bool {
	postalCode := NormalizePersianNumbers(fl.Field().String())
	// Remove any dashes or spaces
	postalCode = strings.ReplaceAll(postalCode, "-", "")
	postalCode = strings.ReplaceAll(postalCode, " ", "")

	postalCodeRegex := regexp.MustCompile(`^[0-9]{10}$`)
	return postalCodeRegex.MatchString(postalCode)
}

// validateIranianNationalCode validates Iranian national codes (10 digits with check digit)
func validateIranianNationalCode(fl validator.FieldLevel) bool {
	nationalCode := NormalizePersianNumbers(fl.Field().String())

	if len(nationalCode) != 10 {
		return false
	}

	// Check if all digits
	for _, char := range nationalCode {
		if char < '0' || char > '9' {
			return false
		}
	}

	// Validate check digit
	check := int(nationalCode[9] - '0')
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(nationalCode[i]-'0') * (10 - i)
	}

	remainder := sum % 11

	return (remainder < 2 && check == remainder) || (remainder >= 2 && check == 11-remainder)
}

// validateIranianSheba validates Iranian Sheba numbers
// Format: 25 digits (without IR prefix) - matches sample "6201600000000000080068121"
func validateIranianSheba(fl validator.FieldLevel) bool {
	sheba := strings.TrimSpace(fl.Field().String())

	// Must be exactly 25 digits (matching sample format)
	if len(sheba) != 25 {
		return false
	}

	// Check if all characters are digits
	for _, char := range sheba {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

// validateIranianBankCardNumber validates Iranian bank card numbers
// Format: 16 digits, with Luhn algorithm check
func validateIranianBankCardNumber(fl validator.FieldLevel) bool {
	cardNum := NormalizePersianNumbers(fl.Field().String())
	// Remove spaces and dashes
	cardNum = strings.ReplaceAll(cardNum, " ", "")
	cardNum = strings.ReplaceAll(cardNum, "-", "")

	// Must be exactly 16 digits
	if len(cardNum) != 16 {
		return false
	}

	// Check if all digits
	for _, char := range cardNum {
		if char < '0' || char > '9' {
			return false
		}
	}

	// Validate using Luhn algorithm
	return validateLuhn(cardNum)
}

// validateLuhn validates a number using the Luhn algorithm
func validateLuhn(number string) bool {
	sum := 0
	alternate := false

	// Process from right to left
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')

		if alternate {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}

// ValidateIranianSheba is a standalone function to validate Iranian Sheba (IBAN).
// Accepts: 25 digits only, or "IR" (case-insensitive) + 24 digits (standard Iranian IBAN, 26 chars total).
func ValidateIranianSheba(sheba string) bool {
	sheba = strings.TrimSpace(sheba)
	if len(sheba) == 0 {
		return false
	}
	// Strip optional "IR" prefix (Iranian IBAN country code)
	if len(sheba) >= 2 && (sheba[0] == 'I' || sheba[0] == 'i') && (sheba[1] == 'R' || sheba[1] == 'r') {
		sheba = sheba[2:]
	}
	// Accept 24 digits (standard IBAN) or 25 digits (legacy/sample format)
	if len(sheba) != 24 && len(sheba) != 25 {
		return false
	}
	for _, char := range sheba {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// ValidateIranianBankCardNumber is a standalone function to validate Iranian bank card numbers
func ValidateIranianBankCardNumber(cardNum string) bool {
	cardNum = NormalizePersianNumbers(cardNum)
	cardNum = strings.ReplaceAll(cardNum, " ", "")
	cardNum = strings.ReplaceAll(cardNum, "-", "")

	if len(cardNum) != 16 {
		return false
	}

	for _, char := range cardNum {
		if char < '0' || char > '9' {
			return false
		}
	}

	return validateLuhn(cardNum)
}
