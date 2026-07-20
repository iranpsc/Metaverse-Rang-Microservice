package handler

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/service"
	"metarang/shared/pkg/helpers"
)

// mapServiceErrorToValidationFields maps service errors to field validation errors
func mapServiceErrorToValidationFields(err error, locale string) (map[string]string, bool) {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	switch {
	// KYC errors
	case errors.Is(err, service.ErrInvalidFname):
		validationErrors["fname"] = fmt.Sprintf(t.Invalid, "fname")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidLname):
		validationErrors["lname"] = fmt.Sprintf(t.Invalid, "lname")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidMelliCode):
		validationErrors["melli_code"] = fmt.Sprintf(t.IranianNationalCode, "melli_code")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidBirthdate):
		validationErrors["birthdate"] = fmt.Sprintf(t.Invalid, "birthdate")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidProvince):
		validationErrors["province"] = fmt.Sprintf(t.Invalid, "province")
		return validationErrors, true
	case errors.Is(err, service.ErrProvinceRequired):
		validationErrors["province"] = fmt.Sprintf(t.Required, "province")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidGender):
		validationErrors["gender"] = fmt.Sprintf(t.Invalid, "gender")
		return validationErrors, true
	case errors.Is(err, service.ErrGenderRequired):
		validationErrors["gender"] = fmt.Sprintf(t.Required, "gender")
		return validationErrors, true
	case errors.Is(err, service.ErrVerifyTextIDRequired):
		validationErrors["verify_text_id"] = fmt.Sprintf(t.Required, "verify_text_id")
		return validationErrors, true
	case errors.Is(err, service.ErrVerifyTextIDNotFound):
		validationErrors["verify_text_id"] = fmt.Sprintf(t.Invalid, "verify_text_id")
		return validationErrors, true
	case errors.Is(err, service.ErrVideoRequired):
		validationErrors["video"] = fmt.Sprintf(t.Required, "video")
		return validationErrors, true
	case errors.Is(err, service.ErrVideoFileNotFound):
		validationErrors["video"] = "video file not found"
		return validationErrors, true
	case errors.Is(err, service.ErrMelliCardRequired):
		validationErrors["melli_card"] = fmt.Sprintf(t.Required, "melli_card")
		return validationErrors, true
	case errors.Is(err, service.ErrMelliCodeNotUnique):
		validationErrors["melli_code"] = fmt.Sprintf(t.Unique, "melli_code")
		return validationErrors, true

	// Bank account errors
	case errors.Is(err, service.ErrInvalidBankName):
		validationErrors["bank_name"] = fmt.Sprintf(t.Invalid, "bank_name")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidShabaNum):
		validationErrors["shaba_num"] = fmt.Sprintf(t.IranianSheba, "shaba_num")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidCardNum):
		validationErrors["card_num"] = fmt.Sprintf(t.IranianBankCard, "card_num")
		return validationErrors, true
	case errors.Is(err, service.ErrShabaNumNotUnique):
		validationErrors["shaba_num"] = fmt.Sprintf(t.Unique, "shaba_num")
		return validationErrors, true
	case errors.Is(err, service.ErrCardNumNotUnique):
		validationErrors["card_num"] = fmt.Sprintf(t.Unique, "card_num")
		return validationErrors, true

	// Personal info errors
	case errors.Is(err, service.ErrInvalidOccupation):
		validationErrors["occupation"] = fmt.Sprintf(t.Invalid, "occupation")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidEducation):
		validationErrors["education"] = fmt.Sprintf(t.Invalid, "education")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidMemory):
		validationErrors["memory"] = fmt.Sprintf(t.Invalid, "memory")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidLovedCity):
		validationErrors["loved_city"] = fmt.Sprintf(t.Invalid, "loved_city")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidLovedCountry):
		validationErrors["loved_country"] = fmt.Sprintf(t.Invalid, "loved_country")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidLovedLanguage):
		validationErrors["loved_language"] = fmt.Sprintf(t.Invalid, "loved_language")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidProblemSolving):
		validationErrors["problem_solving"] = fmt.Sprintf(t.Invalid, "problem_solving")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidPrediction):
		validationErrors["prediction"] = fmt.Sprintf(t.Invalid, "prediction")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidAbout):
		validationErrors["about"] = fmt.Sprintf(t.Invalid, "about")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidPassionKey):
		validationErrors["passion_key"] = fmt.Sprintf(t.Invalid, "passion_key")
		return validationErrors, true

	// Settings errors
	case errors.Is(err, service.ErrInvalidCheckoutDays):
		validationErrors["checkout_days_count"] = fmt.Sprintf(t.Invalid, "checkout_days_count")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidAutomaticLogout):
		validationErrors["automatic_logout"] = fmt.Sprintf(t.Invalid, "automatic_logout")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidProfileSetting):
		validationErrors["setting"] = fmt.Sprintf(t.Invalid, "setting")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidPrivacyKey):
		validationErrors["privacy_key"] = fmt.Sprintf(t.Invalid, "privacy_key")
		return validationErrors, true
	case errors.Is(err, service.ErrInvalidPrivacyValue):
		validationErrors["privacy_value"] = fmt.Sprintf(t.Invalid, "privacy_value")
		return validationErrors, true

	// User events errors
	case errors.Is(err, service.ErrInvalidCitizenCode):
		validationErrors["citizen_code"] = fmt.Sprintf(t.Invalid, "citizen_code")
		return validationErrors, true

	// Profile photo errors
	case errors.Is(err, service.ErrInvalidImage):
		validationErrors["image"] = fmt.Sprintf(t.Invalid, "image")
		return validationErrors, true

	// Profile limitation errors
	case errors.Is(err, service.ErrInvalidOptions):
		validationErrors["options"] = fmt.Sprintf(t.Invalid, "options")
		return validationErrors, true
	}

	return nil, false
}

// returnValidationError returns a gRPC InvalidArgument error with encoded validation fields
func returnValidationError(fields map[string]string) error {
	encodedError := helpers.EncodeValidationError(fields)
	return status.Error(codes.InvalidArgument, encodedError)
}
