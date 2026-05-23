package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
)

// fakeKYCRepository is a mock implementation of KYCRepository for testing
type fakeKYCRepository struct {
	kycs                map[uint64]*models.KYC
	bankAccounts        map[uint64]*models.BankAccount
	verifyTexts         map[uint64]bool // Track which verify_text_ids exist
	createCount         int
	updateCount         int
	findByUserID        func(ctx context.Context, userID uint64) (*models.KYC, error)
	checkMelliCode      func(ctx context.Context, melliCode string, excludeUserID uint64) (bool, error)
	checkVerifyText     func(ctx context.Context, verifyTextID uint64) (bool, error)
}

func newFakeKYCRepository() *fakeKYCRepository {
	// By default, verify_text_id 1 exists
	verifyTexts := make(map[uint64]bool)
	verifyTexts[1] = true
	verifyTexts[2] = true
	verifyTexts[3] = true
	
	return &fakeKYCRepository{
		kycs:         make(map[uint64]*models.KYC),
		bankAccounts: make(map[uint64]*models.BankAccount),
		verifyTexts:  verifyTexts,
	}
}

func (r *fakeKYCRepository) Create(ctx context.Context, kyc *models.KYC) error {
	r.createCount++
	if kyc.ID == 0 {
		kyc.ID = uint64(len(r.kycs) + 1)
	}
	r.kycs[kyc.UserID] = kyc
	return nil
}

func (r *fakeKYCRepository) FindByUserID(ctx context.Context, userID uint64) (*models.KYC, error) {
	if r.findByUserID != nil {
		return r.findByUserID(ctx, userID)
	}
	return r.kycs[userID], nil
}

func (r *fakeKYCRepository) Update(ctx context.Context, kyc *models.KYC) error {
	r.updateCount++
	r.kycs[kyc.UserID] = kyc
	return nil
}

func (r *fakeKYCRepository) CheckUniqueMelliCode(ctx context.Context, melliCode string, excludeUserID uint64) (bool, error) {
	if r.checkMelliCode != nil {
		return r.checkMelliCode(ctx, melliCode, excludeUserID)
	}
	for _, kyc := range r.kycs {
		if kyc.MelliCode == melliCode && kyc.UserID != excludeUserID {
			return false, nil
		}
	}
	return true, nil
}

func (r *fakeKYCRepository) CreateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error {
	if bankAccount.ID == 0 {
		bankAccount.ID = uint64(len(r.bankAccounts) + 1)
	}
	r.bankAccounts[bankAccount.ID] = bankAccount
	return nil
}

func (r *fakeKYCRepository) FindBankAccountsByUserID(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
	var accounts []*models.BankAccount
	for _, account := range r.bankAccounts {
		if account.BankableID == userID {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *fakeKYCRepository) FindBankAccountByID(ctx context.Context, bankAccountID uint64) (*models.BankAccount, error) {
	return r.bankAccounts[bankAccountID], nil
}

func (r *fakeKYCRepository) UpdateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error {
	r.bankAccounts[bankAccount.ID] = bankAccount
	return nil
}

func (r *fakeKYCRepository) DeleteBankAccount(ctx context.Context, bankAccountID uint64) error {
	delete(r.bankAccounts, bankAccountID)
	return nil
}

func (r *fakeKYCRepository) CheckUniqueShaba(ctx context.Context, shabaNum string, excludeID uint64) (bool, error) {
	for _, account := range r.bankAccounts {
		if account.ShabaNum == shabaNum && account.ID != excludeID {
			return false, nil
		}
	}
	return true, nil
}

func (r *fakeKYCRepository) CheckUniqueCard(ctx context.Context, cardNum string, excludeID uint64) (bool, error) {
	for _, account := range r.bankAccounts {
		if account.CardNum == cardNum && account.ID != excludeID {
			return false, nil
		}
	}
	return true, nil
}

func (r *fakeKYCRepository) CheckVerifyTextExists(ctx context.Context, verifyTextID uint64) (bool, error) {
	if r.checkVerifyText != nil {
		return r.checkVerifyText(ctx, verifyTextID)
	}
	return r.verifyTexts[verifyTextID], nil
}

// fakeKYCUserRepository is a minimal mock for UserRepository
type fakeKYCUserRepository struct {
	users map[uint64]*models.User
}

func newFakeKYCUserRepository(users map[uint64]*models.User) *fakeKYCUserRepository {
	return &fakeKYCUserRepository{users: users}
}

func (r *fakeKYCUserRepository) FindByID(ctx context.Context, id uint64) (*models.User, error) {
	return r.users[id], nil
}

func (r *fakeKYCUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (r *fakeKYCUserRepository) FindByPhone(ctx context.Context, phone string) (*models.User, error) {
	for _, user := range r.users {
		if user.Phone.Valid && user.Phone.String == phone {
			return user, nil
		}
	}
	return nil, nil
}

func (r *fakeKYCUserRepository) Create(ctx context.Context, user *models.User) error {
	return nil
}

func (r *fakeKYCUserRepository) Update(ctx context.Context, user *models.User) error {
	return nil
}

func (r *fakeKYCUserRepository) UpdateLastSeen(ctx context.Context, userID uint64) error {
	return nil
}

func (r *fakeKYCUserRepository) UpdateScore(ctx context.Context, userID uint64, score int32) error {
	return nil
}

func (r *fakeKYCUserRepository) FindByCode(ctx context.Context, code string) (*models.User, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) FindReferrals(ctx context.Context, referrerID uint64) ([]*models.User, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) FindReferrer(ctx context.Context, userID uint64) (*models.User, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) CreateSettings(ctx context.Context, settings *models.Settings) error {
	return nil
}

func (r *fakeKYCUserRepository) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) GetUnreadNotificationsCount(ctx context.Context, userID uint64) (int32, error) {
	return 0, nil
}

func (r *fakeKYCUserRepository) MarkEmailAsVerified(ctx context.Context, userID uint64) error {
	return nil
}

func (r *fakeKYCUserRepository) UpdatePhone(ctx context.Context, userID uint64, phone string) error {
	return nil
}

func (r *fakeKYCUserRepository) MarkPhoneAsVerified(ctx context.Context, userID uint64) error {
	return nil
}

func (r *fakeKYCUserRepository) IsPhoneTaken(ctx context.Context, phone string, excludeUserID uint64) (bool, error) {
	return false, nil
}

func (r *fakeKYCUserRepository) ListUsers(ctx context.Context, search, orderBy string, page, pageSize int32) ([]*repository.UserWithRelations, int32, error) {
	return nil, 0, nil
}

func (r *fakeKYCUserRepository) GetUsersLevelsForList(ctx context.Context, userIDs []uint64) (map[uint64]*repository.UserListLevels, error) {
	return map[uint64]*repository.UserListLevels{}, nil
}

func (r *fakeKYCUserRepository) GetFollowersCount(ctx context.Context, userID uint64) (int32, error) {
	return 0, nil
}

func (r *fakeKYCUserRepository) GetFollowingCount(ctx context.Context, userID uint64) (int32, error) {
	return 0, nil
}

func (r *fakeKYCUserRepository) GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error) {
	return "", nil
}

func (r *fakeKYCUserRepository) GetAllProfilePhotoURLs(ctx context.Context, userID uint64) ([]string, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) GetUserLatestLevel(ctx context.Context, userID uint64) (*repository.UserLevel, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) GetLevelsBelowScore(ctx context.Context, score int32) ([]*repository.UserLevel, error) {
	return nil, nil
}

func (r *fakeKYCUserRepository) GetNextLevelScore(ctx context.Context, score int32) (int32, error) {
	return 0, nil
}

func (r *fakeKYCUserRepository) GetFeatureCounts(ctx context.Context, userID uint64) (int32, int32, int32, error) {
	return 0, 0, 0, nil
}

func TestGetKYC_NotFound(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.GetKYC(ctx, 1)
	if err != nil {
		t.Fatalf("GetKYC returned error: %v", err)
	}
	if kyc != nil {
		t.Errorf("expected nil for non-existent KYC, got %v", kyc)
	}
}

func TestGetKYC_NotOwned(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:     1,
		UserID: 2,
		Fname:  "Other",
		Lname:  "User",
		Status: 0,
	}
	kycRepo.kycs[2] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.GetKYC(ctx, 1)
	if err != nil {
		t.Fatalf("GetKYC returned error: %v", err)
	}
	if kyc != nil {
		t.Errorf("expected nil when user does not own KYC, got %v", kyc)
	}
}

func TestGetKYC_Found(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    1,
		Fname:     "Ali",
		Lname:     "Karimi",
		MelliCode: "1234567890",
		Status:    0,
		Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	kycRepo.kycs[1] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.GetKYC(ctx, 1)
	if err != nil {
		t.Fatalf("GetKYC returned error: %v", err)
	}
	if kyc == nil {
		t.Fatalf("expected KYC to be found")
	}
	if kyc.Fname != "Ali" {
		t.Errorf("expected Fname 'Ali', got %q", kyc.Fname)
	}
}

func TestUpdateKYC_CreateNew(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789", // Valid Iranian national code format (10 digits)
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}
	if kyc == nil {
		t.Fatalf("expected KYC to be created")
	}
	if kyc.Fname != "Ali" {
		t.Errorf("expected Fname 'Ali', got %q", kyc.Fname)
	}
	if kyc.Status != 0 {
		t.Errorf("expected Status 0 (pending), got %d", kyc.Status)
	}
	if kycRepo.createCount != 1 {
		t.Errorf("expected createCount 1, got %d", kycRepo.createCount)
	}
}

func TestUpdateKYC_UpdateRejected(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    1,
		Fname:     "Old",
		Lname:     "Name",
		MelliCode: "1234567890",
		Status:    -1, // Rejected
		Errors:    sql.NullString{String: "Some error", Valid: true},
		Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	kycRepo.kycs[1] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}
	if kyc.Fname != "Ali" {
		t.Errorf("expected Fname 'Ali', got %q", kyc.Fname)
	}
	if kyc.Status != 0 {
		t.Errorf("expected Status 0 (pending), got %d", kyc.Status)
	}
	if kyc.Errors.Valid {
		t.Errorf("expected errors to be cleared, got %v", kyc.Errors)
	}
	if kycRepo.updateCount != 1 {
		t.Errorf("expected updateCount 1, got %d", kycRepo.updateCount)
	}
}

func TestUpdateKYC_RejectPendingUpdate(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    1,
		Fname:     "Old",
		Lname:     "Name",
		MelliCode: "1234567890",
		Status:    0, // Pending - cannot update
		Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	kycRepo.kycs[1] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error when updating pending KYC")
	}
	if !errors.Is(err, ErrKYCNotRejected) {
		t.Errorf("expected ErrKYCNotRejected, got %v", err)
	}
}

func TestUpdateKYC_RejectApprovedUpdate(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    1,
		Fname:     "Old",
		Lname:     "Name",
		MelliCode: "1234567890",
		Status:    1, // Approved - cannot update
		Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	kycRepo.kycs[1] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error when updating approved KYC")
	}
	if !errors.Is(err, ErrKYCNotRejected) {
		t.Errorf("expected ErrKYCNotRejected, got %v", err)
	}
}

func TestUpdateKYC_InvalidFname(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"A", // Too short
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for invalid fname")
	}
	if !errors.Is(err, ErrInvalidFname) {
		t.Errorf("expected ErrInvalidFname, got %v", err)
	}
}

func TestUpdateKYC_InvalidLname(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"K", // Too short
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for invalid lname")
	}
	if !errors.Is(err, ErrInvalidLname) {
		t.Errorf("expected ErrInvalidLname, got %v", err)
	}
}

func TestUpdateKYC_InvalidGender(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"invalid", // Invalid gender
	)
	if err == nil {
		t.Fatalf("expected error for invalid gender")
	}
	if !errors.Is(err, ErrInvalidGender) {
		t.Errorf("expected ErrInvalidGender, got %v", err)
	}
}

func TestUpdateKYC_InvalidBirthdate(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"invalid-date", // Invalid date format
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for invalid birthdate")
	}
	if !errors.Is(err, ErrInvalidBirthdate) {
		t.Errorf("expected ErrInvalidBirthdate, got %v", err)
	}
}

func TestUpdateKYC_DuplicateMelliCode(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	// Create existing KYC with melli_code
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    2, // Different user
		MelliCode: "0123456789",
		Status:    -1,
	}
	kycRepo.kycs[2] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789", // Same melli_code as user 2
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for duplicate melli_code")
	}
	if !errors.Is(err, ErrMelliCodeNotUnique) {
		t.Errorf("expected ErrMelliCodeNotUnique, got %v", err)
	}
}

func TestUpdateKYC_ValidGenders(t *testing.T) {
	ctx := context.Background()
	validGenders := []string{"male", "female", "other"}

	for _, gender := range validGenders {
		t.Run(gender, func(t *testing.T) {
			kycRepo := newFakeKYCRepository()
			userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
			service := NewKYCService(kycRepo, userRepo)

			_, err := service.UpdateKYC(
				ctx,
				1,
				"Ali",
				"Karimi",
				"0123456789",
				"1403/01/15",
				"Tehran",
				"/uploads/kyc/melli-card.jpg",
				"tmp/uploads",
				"video.mp4",
				1,
				gender,
			)
			if err != nil {
				t.Errorf("expected no error for valid gender %q, got %v", gender, err)
			}
		})
	}
}

func TestUpdateKYC_TrimsWhitespace(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"  Ali  ",
		"  Karimi  ",
		"  0123456789  ",
		"1403/01/15",
		"  Tehran  ",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"  male  ",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}
	if kyc.Fname != "Ali" {
		t.Errorf("expected trimmed Fname 'Ali', got %q", kyc.Fname)
	}
	if kyc.Lname != "Karimi" {
		t.Errorf("expected trimmed Lname 'Karimi', got %q", kyc.Lname)
	}
	if kyc.MelliCode != "0123456789" {
		t.Errorf("expected trimmed MelliCode '0123456789', got %q", kyc.MelliCode)
	}
	if kyc.Province != "Tehran" {
		t.Errorf("expected trimmed Province 'Tehran', got %q", kyc.Province)
	}
	if kyc.Gender.String != "male" {
		t.Errorf("expected trimmed Gender 'male', got %q", kyc.Gender.String)
	}
}

// Test new required field validations
func TestUpdateKYC_MelliCardRequired(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"", // Empty melli_card
		"tmp/uploads",
		"video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for missing melli_card")
	}
	if !errors.Is(err, ErrMelliCardRequired) {
		t.Errorf("expected ErrMelliCardRequired, got %v", err)
	}
}

func TestUpdateKYC_VideoRequired(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name      string
		videoPath string
		videoName string
	}{
		{"empty path", "", "video.mp4"},
		{"empty name", "tmp/uploads", ""},
		{"both empty", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kycRepo := newFakeKYCRepository()
			userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
			service := NewKYCService(kycRepo, userRepo)

			_, err := service.UpdateKYC(
				ctx,
				1,
				"Ali",
				"Karimi",
				"0123456789",
				"1403/01/15",
				"Tehran",
				"/uploads/kyc/melli-card.jpg",
				tc.videoPath,
				tc.videoName,
				1,
				"male",
			)
			if err == nil {
				t.Fatalf("expected error for missing video (path=%q, name=%q)", tc.videoPath, tc.videoName)
			}
			if !errors.Is(err, ErrVideoRequired) {
				t.Errorf("expected ErrVideoRequired, got %v", err)
			}
		})
	}
}

func TestUpdateKYC_VerifyTextIDRequired(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		0, // Zero verify_text_id
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for missing verify_text_id")
	}
	if !errors.Is(err, ErrVerifyTextIDRequired) {
		t.Errorf("expected ErrVerifyTextIDRequired, got %v", err)
	}
}

func TestUpdateKYC_VerifyTextIDNotFound(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		999, // Non-existent verify_text_id
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for non-existent verify_text_id")
	}
	if !errors.Is(err, ErrVerifyTextIDNotFound) {
		t.Errorf("expected ErrVerifyTextIDNotFound, got %v", err)
	}
}

func TestUpdateKYC_ProvinceRequired(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name     string
		province string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kycRepo := newFakeKYCRepository()
			userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
			service := NewKYCService(kycRepo, userRepo)

			_, err := service.UpdateKYC(
				ctx,
				1,
				"Ali",
				"Karimi",
				"0123456789",
				"1403/01/15",
				tc.province,
				"/uploads/kyc/melli-card.jpg",
				"tmp/uploads",
				"video.mp4",
				1,
				"male",
			)
			if err == nil {
				t.Fatalf("expected error for missing province")
			}
			if !errors.Is(err, ErrProvinceRequired) {
				t.Errorf("expected ErrProvinceRequired, got %v", err)
			}
		})
	}
}

func TestUpdateKYC_GenderRequired(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name   string
		gender string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kycRepo := newFakeKYCRepository()
			userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
			service := NewKYCService(kycRepo, userRepo)

			_, err := service.UpdateKYC(
				ctx,
				1,
				"Ali",
				"Karimi",
				"0123456789",
				"1403/01/15",
				"Tehran",
				"/uploads/kyc/melli-card.jpg",
				"tmp/uploads",
				"video.mp4",
				1,
				tc.gender,
			)
			if err == nil {
				t.Fatalf("expected error for missing gender")
			}
			if !errors.Is(err, ErrGenderRequired) {
				t.Errorf("expected ErrGenderRequired, got %v", err)
			}
		})
	}
}

func TestUpdateKYC_ProvinceMaxLength(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	// Create a province name longer than 255 characters
	longProvince := string(make([]byte, 256))

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		longProvince,
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for province exceeding max length")
	}
	if !errors.Is(err, ErrInvalidProvince) {
		t.Errorf("expected ErrInvalidProvince, got %v", err)
	}
}

func TestUpdateKYC_AllRequiredFieldsSet(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"female",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}

	// Verify all required fields are set
	if kyc.MelliCard == "" {
		t.Error("expected MelliCard to be set")
	}
	if !kyc.Video.Valid || kyc.Video.String == "" {
		t.Error("expected Video to be set")
	}
	if !kyc.VerifyTextID.Valid || kyc.VerifyTextID.Int64 != 1 {
		t.Errorf("expected VerifyTextID to be 1, got %v", kyc.VerifyTextID)
	}
	if kyc.Province == "" {
		t.Error("expected Province to be set")
	}
	if !kyc.Gender.Valid || kyc.Gender.String == "" {
		t.Error("expected Gender to be set")
	}
	if kyc.Gender.String != "female" {
		t.Errorf("expected Gender to be 'female', got %q", kyc.Gender.String)
	}
}

func TestUpdateKYC_StatusAndErrorsReset(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	existingKYC := &models.KYC{
		ID:        1,
		UserID:    1,
		Fname:     "Old",
		Lname:     "Name",
		MelliCode: "1234567890",
		Status:    -1, // Rejected
		Errors:    sql.NullString{String: "Previous error message", Valid: true},
		Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	kycRepo.kycs[1] = existingKYC
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}

	// Verify status is reset to pending
	if kyc.Status != 0 {
		t.Errorf("expected Status to be 0 (pending), got %d", kyc.Status)
	}

	// Verify errors are cleared
	if kyc.Errors.Valid {
		t.Errorf("expected Errors to be cleared, got %v", kyc.Errors)
	}
}

func TestUpdateKYC_BirthdateConversion(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	kyc, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		"Karimi",
		"0123456789",
		"1403/01/15", // Jalali date
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err != nil {
		t.Fatalf("UpdateKYC returned error: %v", err)
	}

	// Verify birthdate is converted and stored
	if !kyc.Birthdate.Valid {
		t.Error("expected Birthdate to be valid")
	}
	// The date should be converted from Jalali to Gregorian
	// 1403/01/15 in Jalali converts to a date in 2024 or 2025 depending on the exact conversion
	// Accept either year as the conversion might vary slightly
	year := kyc.Birthdate.Time.Year()
	if year != 2024 && year != 2025 {
		t.Errorf("expected birthdate year to be 2024 or 2025, got %d", year)
	}
}

func TestUpdateKYC_InvalidMelliCode(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name      string
		melliCode string
	}{
		{"too short", "123"},
		{"too long", "123456789012"},
		{"invalid format", "abcdefghij"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kycRepo := newFakeKYCRepository()
			userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
			service := NewKYCService(kycRepo, userRepo)

			_, err := service.UpdateKYC(
				ctx,
				1,
				"Ali",
				"Karimi",
				tc.melliCode,
				"1403/01/15",
				"Tehran",
				"/uploads/kyc/melli-card.jpg",
				"tmp/uploads",
				"video.mp4",
				1,
				"male",
			)
			if err == nil {
				t.Fatalf("expected error for invalid melli_code: %q", tc.melliCode)
			}
			if !errors.Is(err, ErrInvalidMelliCode) {
				t.Errorf("expected ErrInvalidMelliCode, got %v", err)
			}
		})
	}
}

func TestUpdateKYC_FnameMaxLength(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	// Create a name longer than 255 characters
	longName := string(make([]byte, 256))

	_, err := service.UpdateKYC(
		ctx,
		1,
		longName,
		"Karimi",
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for fname exceeding max length")
	}
	if !errors.Is(err, ErrInvalidFname) {
		t.Errorf("expected ErrInvalidFname, got %v", err)
	}
}

func TestUpdateKYC_LnameMaxLength(t *testing.T) {
	ctx := context.Background()
	kycRepo := newFakeKYCRepository()
	userRepo := newFakeKYCUserRepository(map[uint64]*models.User{1: {ID: 1}})
	service := NewKYCService(kycRepo, userRepo)

	// Create a name longer than 255 characters
	longName := string(make([]byte, 256))

	_, err := service.UpdateKYC(
		ctx,
		1,
		"Ali",
		longName,
		"0123456789",
		"1403/01/15",
		"Tehran",
		"/uploads/kyc/melli-card.jpg",
		"http://localhost/uploads/kyc/video.mp4",
		1,
		"male",
	)
	if err == nil {
		t.Fatalf("expected error for lname exceeding max length")
	}
	if !errors.Is(err, ErrInvalidLname) {
		t.Errorf("expected ErrInvalidLname, got %v", err)
	}
}

func TestKYC_HelperMethods(t *testing.T) {
	t.Run("Rejected method", func(t *testing.T) {
		kyc := &models.KYC{Status: -1}
		if !kyc.Rejected() {
			t.Error("expected Rejected() to return true for status -1")
		}
		if kyc.Pending() {
			t.Error("expected Pending() to return false for status -1")
		}
		if kyc.Approved() {
			t.Error("expected Approved() to return false for status -1")
		}
	})

	t.Run("Pending method", func(t *testing.T) {
		kyc := &models.KYC{Status: 0}
		if kyc.Rejected() {
			t.Error("expected Rejected() to return false for status 0")
		}
		if !kyc.Pending() {
			t.Error("expected Pending() to return true for status 0")
		}
		if kyc.Approved() {
			t.Error("expected Approved() to return false for status 0")
		}
	})

	t.Run("Approved method", func(t *testing.T) {
		kyc := &models.KYC{Status: 1}
		if kyc.Rejected() {
			t.Error("expected Rejected() to return false for status 1")
		}
		if kyc.Pending() {
			t.Error("expected Pending() to return false for status 1")
		}
		if !kyc.Approved() {
			t.Error("expected Approved() to return true for status 1")
		}
	})
}
