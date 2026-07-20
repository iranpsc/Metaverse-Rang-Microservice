package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/shared/pkg/helpers"
	"metarang/shared/pkg/jalali"
)

var (
	ErrKYCNotFound            = errors.New("kyc not found")
	ErrKYCNotOwned            = errors.New("kyc does not belong to user")
	ErrKYCNotRejected         = errors.New("kyc must be rejected to update")
	ErrInvalidFname           = errors.New("fname must be between 2 and 255 characters")
	ErrInvalidLname           = errors.New("lname must be between 2 and 255 characters")
	ErrInvalidMelliCode       = errors.New("invalid Iranian national code")
	ErrMelliCodeNotUnique     = errors.New("melli code already exists")
	ErrInvalidBirthdate       = errors.New("invalid birthdate format")
	ErrInvalidProvince        = errors.New("province must be at most 255 characters")
	ErrProvinceRequired       = errors.New("province is required")
	ErrInvalidGender          = errors.New("gender must be one of: male, female, other")
	ErrGenderRequired         = errors.New("gender is required")
	ErrVerifyTextIDRequired   = errors.New("verify_text_id is required")
	ErrVerifyTextIDNotFound   = errors.New("verify_text_id does not exist")
	ErrVideoRequired          = errors.New("video is required")
	ErrVideoFileNotFound      = errors.New("video file not found")
	ErrMelliCardRequired      = errors.New("melli_card is required")
	ErrBankAccountNotFound    = errors.New("bank account not found")
	ErrBankAccountNotOwned    = errors.New("bank account does not belong to user")
	ErrBankAccountNotRejected = errors.New("bank account must be rejected to update")
	ErrUserNotVerified        = errors.New("user must be verified to create bank account")
	ErrInvalidBankName        = errors.New("bank name must be at least 2 characters")
	ErrInvalidShabaNum        = errors.New("invalid Iranian sheba number")
	ErrInvalidCardNum         = errors.New("invalid Iranian bank card number")
	ErrShabaNumNotUnique      = errors.New("sheba number already exists")
	ErrCardNumNotUnique       = errors.New("card number already exists")
)

type KYCService interface {
	GetKYC(ctx context.Context, userID uint64) (*models.KYC, error)
	UpdateKYC(ctx context.Context, userID uint64, fname, lname, melliCode, birthdate, province, melliCard, videoPath, videoName string, verifyTextID uint64, gender string) (*models.KYC, error)
	ListBankAccounts(ctx context.Context, userID uint64) ([]*models.BankAccount, error)
	CreateBankAccount(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error)
	GetBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error)
	UpdateBankAccount(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error)
	DeleteBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) error
}

type kycService struct {
	kycRepo       repository.KYCRepository
	userRepo      repository.UserRepository
	storageClient KYCStorageClient
}

func NewKYCService(kycRepo repository.KYCRepository, userRepo repository.UserRepository, storageClient KYCStorageClient) KYCService {
	return &kycService{
		kycRepo:       kycRepo,
		userRepo:      userRepo,
		storageClient: storageClient,
	}
}

func (s *kycService) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	kyc, err := s.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kyc: %w", err)
	}
	if kyc == nil {
		return nil, nil
	}
	// KycPolicy::view — only the owner may view their record
	if kyc.UserID != userID {
		return nil, nil
	}
	return kyc, nil
}

func (s *kycService) UpdateKYC(ctx context.Context, userID uint64, fname, lname, melliCode, birthdate, province, melliCard, videoPath, videoName string, verifyTextID uint64, gender string) (*models.KYC, error) {
	// Validate required fields
	if melliCard == "" {
		return nil, ErrMelliCardRequired
	}
	videoPath = strings.TrimSpace(videoPath)
	videoName = strings.TrimSpace(videoName)
	if videoPath == "" || videoName == "" {
		return nil, ErrVideoRequired
	}
	if verifyTextID == 0 {
		return nil, ErrVerifyTextIDRequired
	}

	// Check if verify_text_id exists
	exists, err := s.kycRepo.CheckVerifyTextExists(ctx, verifyTextID)
	if err != nil {
		return nil, fmt.Errorf("failed to check verify_text_id: %w", err)
	}
	if !exists {
		return nil, ErrVerifyTextIDNotFound
	}

	// Validate input (this also trims the values internally)
	if err := s.validateKYCInput(fname, lname, melliCode, birthdate, province, gender); err != nil {
		return nil, err
	}

	// Trim values after validation (validation trims internally but doesn't modify originals)
	fname = strings.TrimSpace(fname)
	lname = strings.TrimSpace(lname)
	melliCode = strings.TrimSpace(melliCode)
	province = strings.TrimSpace(province)
	gender = strings.TrimSpace(gender)

	// Check if KYC already exists
	existing, err := s.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing kyc: %w", err)
	}

	// Policy check: if KYC exists and is not rejected, cannot update
	if existing != nil && !existing.Rejected() {
		return nil, ErrKYCNotRejected
	}

	// Parse Jalali birthdate to Gregorian
	parsedDate, err := jalali.JalaliToCarbon(birthdate)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBirthdate, err)
	}

	// Check melli_code uniqueness (exclude current user)
	unique, err := s.kycRepo.CheckUniqueMelliCode(ctx, melliCode, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check melli code uniqueness: %w", err)
	}
	if !unique {
		return nil, ErrMelliCodeNotUnique
	}

	videoURL, err := s.promoteKYCVideo(ctx, userID, videoPath, videoName)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing KYC - reset status to pending and clear errors
		existing.Fname = strings.TrimSpace(fname)
		existing.Lname = strings.TrimSpace(lname)
		existing.MelliCode = strings.TrimSpace(melliCode)
		existing.Birthdate = sql.NullTime{Time: parsedDate, Valid: true}
		existing.Province = strings.TrimSpace(province)
		existing.MelliCard = melliCard
		existing.Status = 0                // Reset to pending
		existing.Errors = sql.NullString{} // Clear errors
		// Video, verify_text_id, and gender are required, so always set them
		existing.Video = sql.NullString{String: videoURL, Valid: true}
		existing.VerifyTextID = sql.NullInt64{Int64: int64(verifyTextID), Valid: true}
		existing.Gender = sql.NullString{String: gender, Valid: true}

		if err := s.kycRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update kyc: %w", err)
		}
		return existing, nil
	}

	// Create new KYC
	kyc := &models.KYC{
		UserID:       userID,
		Fname:        strings.TrimSpace(fname),
		Lname:        strings.TrimSpace(lname),
		MelliCode:    strings.TrimSpace(melliCode),
		Province:     strings.TrimSpace(province),
		MelliCard:    melliCard,
		Status:       0, // Pending
		Birthdate:    sql.NullTime{Time: parsedDate, Valid: true},
		Errors:       sql.NullString{},
		Video:        sql.NullString{String: videoURL, Valid: true},
		VerifyTextID: sql.NullInt64{Int64: int64(verifyTextID), Valid: true},
		Gender:       sql.NullString{String: gender, Valid: true},
	}

	if err := s.kycRepo.Create(ctx, kyc); err != nil {
		return nil, fmt.Errorf("failed to create kyc: %w", err)
	}

	return kyc, nil
}

func (s *kycService) promoteKYCVideo(ctx context.Context, userID uint64, videoPath, videoName string) (string, error) {
	if s.storageClient == nil {
		return "", fmt.Errorf("storage service not available")
	}
	return s.storageClient.MoveKYCVideo(ctx, userID, videoPath, videoName)
}

// validateKYCInput validates all KYC input fields
func (s *kycService) validateKYCInput(fname, lname, melliCode, birthdate, province, gender string) error {
	fname = strings.TrimSpace(fname)
	if len(fname) < 2 || len(fname) > 255 {
		return ErrInvalidFname
	}

	lname = strings.TrimSpace(lname)
	if len(lname) < 2 || len(lname) > 255 {
		return ErrInvalidLname
	}

	melliCode = strings.TrimSpace(melliCode)
	cv := helpers.NewCustomValidator()
	if err := cv.Validate(struct {
		Code string `validate:"required,iranian_national_code"`
	}{Code: melliCode}); err != nil {
		return ErrInvalidMelliCode
	}

	// Validate birthdate format (Jalali: Y/m/d) — matches Laravel shamsi_date rule
	if birthdate == "" {
		return ErrInvalidBirthdate
	}
	if _, err := jalali.JalaliToCarbon(birthdate); err != nil {
		return ErrInvalidBirthdate
	}

	province = strings.TrimSpace(province)
	if province == "" {
		return ErrProvinceRequired
	}
	if len(province) > 255 {
		return ErrInvalidProvince
	}

	gender = strings.TrimSpace(gender)
	if gender == "" {
		return ErrGenderRequired
	}
	if gender != "male" && gender != "female" && gender != "other" {
		return ErrInvalidGender
	}

	return nil
}

// hasApprovedKYC matches Laravel User::verified() (kyc.status === 1).
func (s *kycService) hasApprovedKYC(ctx context.Context, userID uint64) (bool, error) {
	kyc, err := s.kycRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get kyc: %w", err)
	}
	if kyc == nil {
		return false, nil
	}
	return kyc.Status == 1, nil
}

// validateBankAccountInput validates bank account input fields
func (s *kycService) validateBankAccountInput(bankName, shabaNum, cardNum string) error {
	bankName = strings.TrimSpace(bankName)
	if len(bankName) < 2 {
		return ErrInvalidBankName
	}
	if len(bankName) > 255 {
		return ErrInvalidBankName
	}

	shabaNum = strings.TrimSpace(shabaNum)
	if !helpers.ValidateIranianSheba(shabaNum) {
		return ErrInvalidShabaNum
	}

	cardNum = strings.TrimSpace(cardNum)
	if !helpers.ValidateIranianBankCardNumber(cardNum) {
		return ErrInvalidCardNum
	}

	return nil
}

func (s *kycService) ListBankAccounts(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
	accounts, err := s.kycRepo.FindBankAccountsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list bank accounts: %w", err)
	}
	return accounts, nil
}

func (s *kycService) CreateBankAccount(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
	// BankAccountPolicy::create requires approved KYC (Laravel User::verified()).
	verified, err := s.hasApprovedKYC(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !verified {
		return nil, ErrUserNotVerified
	}

	// Validate input
	if err := s.validateBankAccountInput(bankName, shabaNum, cardNum); err != nil {
		return nil, err
	}

	// Normalize inputs
	bankName = strings.TrimSpace(bankName)
	shabaNum = strings.TrimSpace(strings.ToUpper(shabaNum))
	cardNum = strings.TrimSpace(cardNum)

	// Check uniqueness (for create, excludeID is 0)
	uniqueShaba, err := s.kycRepo.CheckUniqueShaba(ctx, shabaNum, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to check shaba uniqueness: %w", err)
	}
	if !uniqueShaba {
		return nil, ErrShabaNumNotUnique
	}

	uniqueCard, err := s.kycRepo.CheckUniqueCard(ctx, cardNum, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to check card uniqueness: %w", err)
	}
	if !uniqueCard {
		return nil, ErrCardNumNotUnique
	}

	// Create bank account with status 0 (pending)
	bankAccount := &models.BankAccount{
		BankableType: "App\\Models\\User",
		BankableID:   userID,
		BankName:     bankName,
		ShabaNum:     shabaNum,
		CardNum:      cardNum,
		Status:       0, // Pending verification
		Errors:       sql.NullString{},
	}

	if err := s.kycRepo.CreateBankAccount(ctx, bankAccount); err != nil {
		return nil, fmt.Errorf("failed to create bank account: %w", err)
	}

	return bankAccount, nil
}

func (s *kycService) GetBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
	bankAccount, err := s.kycRepo.FindBankAccountByID(ctx, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to find bank account: %w", err)
	}
	if bankAccount == nil {
		return nil, ErrBankAccountNotFound
	}

	// Check ownership
	if bankAccount.BankableType != "App\\Models\\User" || bankAccount.BankableID != userID {
		return nil, ErrBankAccountNotOwned
	}

	return bankAccount, nil
}

func (s *kycService) UpdateBankAccount(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
	// Get existing bank account
	bankAccount, err := s.kycRepo.FindBankAccountByID(ctx, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to find bank account: %w", err)
	}
	if bankAccount == nil {
		return nil, ErrBankAccountNotFound
	}

	// Check ownership
	if bankAccount.BankableType != "App\\Models\\User" || bankAccount.BankableID != userID {
		return nil, ErrBankAccountNotOwned
	}

	// Check if status is rejected (-1) - only rejected accounts can be updated
	if bankAccount.Status != -1 {
		return nil, ErrBankAccountNotRejected
	}

	// Validate input
	if err := s.validateBankAccountInput(bankName, shabaNum, cardNum); err != nil {
		return nil, err
	}

	// Normalize inputs
	bankName = strings.TrimSpace(bankName)
	shabaNum = strings.TrimSpace(strings.ToUpper(shabaNum))
	cardNum = strings.TrimSpace(cardNum)

	// Check uniqueness (exclude current record)
	uniqueShaba, err := s.kycRepo.CheckUniqueShaba(ctx, shabaNum, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to check shaba uniqueness: %w", err)
	}
	if !uniqueShaba {
		return nil, ErrShabaNumNotUnique
	}

	uniqueCard, err := s.kycRepo.CheckUniqueCard(ctx, cardNum, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to check card uniqueness: %w", err)
	}
	if !uniqueCard {
		return nil, ErrCardNumNotUnique
	}

	// Update bank account - reset to pending and clear errors
	bankAccount.BankName = bankName
	bankAccount.ShabaNum = shabaNum
	bankAccount.CardNum = cardNum
	bankAccount.Status = 0                // Reset to pending
	bankAccount.Errors = sql.NullString{} // Clear errors

	if err := s.kycRepo.UpdateBankAccount(ctx, bankAccount); err != nil {
		return nil, fmt.Errorf("failed to update bank account: %w", err)
	}

	return bankAccount, nil
}

func (s *kycService) DeleteBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) error {
	// Get existing bank account
	bankAccount, err := s.kycRepo.FindBankAccountByID(ctx, bankAccountID)
	if err != nil {
		return fmt.Errorf("failed to find bank account: %w", err)
	}
	if bankAccount == nil {
		return ErrBankAccountNotFound
	}

	// Check ownership
	if bankAccount.BankableType != "App\\Models\\User" || bankAccount.BankableID != userID {
		return ErrBankAccountNotOwned
	}

	// Delete bank account
	if err := s.kycRepo.DeleteBankAccount(ctx, bankAccountID); err != nil {
		return fmt.Errorf("failed to delete bank account: %w", err)
	}

	return nil
}
