package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	notificationspb "metarang/shared/pb/notifications"
)

type fakeUserRepository struct {
	users map[uint64]*models.User
}

func newFakeUserRepository(users map[uint64]*models.User) *fakeUserRepository {
	return &fakeUserRepository{users: users}
}

func (f *fakeUserRepository) Create(context.Context, *models.User) error {
	panic("unexpected call to Create")
}

func (f *fakeUserRepository) FindByEmail(context.Context, string) (*models.User, error) {
	panic("unexpected call to FindByEmail")
}

func (f *fakeUserRepository) FindByID(_ context.Context, id uint64) (*models.User, error) {
	if user, ok := f.users[id]; ok {
		return user, nil
	}
	return nil, nil
}

func (f *fakeUserRepository) Update(context.Context, *models.User) error {
	panic("unexpected call to Update")
}

func (f *fakeUserRepository) UpdateLastSeen(_ context.Context, userID uint64) error {
	if _, ok := f.users[userID]; ok {
		return nil
	}
	return nil
}

func (f *fakeUserRepository) FindByCode(_ context.Context, code string) (*models.User, error) {
	for _, user := range f.users {
		if user.Code == code {
			return user, nil
		}
	}
	return nil, nil
}

func (f *fakeUserRepository) GetSettings(context.Context, uint64) (*models.Settings, error) {
	panic("unexpected call to GetSettings")
}

func (f *fakeUserRepository) CreateSettings(context.Context, *models.Settings) error {
	panic("unexpected call to CreateSettings")
}

func (f *fakeUserRepository) GetKYC(context.Context, uint64) (*models.KYC, error) {
	panic("unexpected call to GetKYC")
}

func (f *fakeUserRepository) GetUnreadNotificationsCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetUnreadNotificationsCount")
}

func (f *fakeUserRepository) MarkEmailAsVerified(context.Context, uint64) error {
	panic("unexpected call to MarkEmailAsVerified")
}

func (f *fakeUserRepository) UpdatePhone(_ context.Context, userID uint64, phone string) error {
	if user, ok := f.users[userID]; ok {
		user.Phone = sql.NullString{String: phone, Valid: phone != ""}
		return nil
	}
	return fmt.Errorf("user %d not found", userID)
}

func (f *fakeUserRepository) MarkPhoneAsVerified(_ context.Context, userID uint64) error {
	if user, ok := f.users[userID]; ok {
		user.PhoneVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		return nil
	}
	return fmt.Errorf("user %d not found", userID)
}

func (f *fakeUserRepository) ExistsByWalletAddress(context.Context, string, uint64) (bool, error) {
	panic("unexpected call to ExistsByWalletAddress")
}

func (f *fakeUserRepository) LinkWalletAddress(context.Context, uint64, string) (repository.LinkWalletResult, error) {
	panic("unexpected call to LinkWalletAddress")
}

func (f *fakeUserRepository) IsPhoneTaken(_ context.Context, phone string, excludeUserID uint64) (bool, error) {
	for id, user := range f.users {
		if id == excludeUserID {
			continue
		}
		if user.Phone.Valid && user.Phone.String == phone {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUserRepository) ListUsers(context.Context, string, string, int32, int32) ([]*repository.UserWithRelations, int32, error) {
	panic("unexpected call to ListUsers")
}

func (f *fakeUserRepository) GetUsersLevelsForList(context.Context, []uint64) (map[uint64]*repository.UserListLevels, error) {
	panic("unexpected call to GetUsersLevelsForList")
}

func (f *fakeUserRepository) GetFollowersCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetFollowersCount")
}

func (f *fakeUserRepository) GetFollowingCount(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetFollowingCount")
}

func (f *fakeUserRepository) GetLatestProfilePhotoURL(context.Context, uint64) (string, error) {
	return "", nil
}

func (f *fakeUserRepository) GetAllProfilePhotoURLs(context.Context, uint64) ([]string, error) {
	panic("unexpected call to GetAllProfilePhotoURLs")
}

func (f *fakeUserRepository) GetUserLatestLevel(context.Context, uint64) (*repository.UserLevel, error) {
	panic("unexpected call to GetUserLatestLevel")
}

func (f *fakeUserRepository) GetLevelsBelowScore(context.Context, int32) ([]*repository.UserLevel, error) {
	panic("unexpected call to GetLevelsBelowScore")
}

func (f *fakeUserRepository) GetNextLevelScore(context.Context, int32) (int32, error) {
	panic("unexpected call to GetNextLevelScore")
}

func (f *fakeUserRepository) GetFeatureCounts(context.Context, uint64) (int32, int32, int32, error) {
	panic("unexpected call to GetFeatureCounts")
}

var _ repository.UserRepository = (*fakeUserRepository)(nil)

type fakeAccountSecurityRepository struct {
	nextID      uint64
	nextOtpID   uint64
	records     map[uint64]*models.AccountSecurity
	otps        map[uint64]*models.Otp
	createCount int
	updateCount int
}

func newFakeAccountSecurityRepository() *fakeAccountSecurityRepository {
	return &fakeAccountSecurityRepository{
		nextID:    100,
		nextOtpID: 200,
		records:   make(map[uint64]*models.AccountSecurity),
		otps:      make(map[uint64]*models.Otp),
	}
}

func (f *fakeAccountSecurityRepository) GetByUserID(_ context.Context, userID uint64) (*models.AccountSecurity, error) {
	if security, ok := f.records[userID]; ok {
		return security, nil
	}
	return nil, nil
}

func (f *fakeAccountSecurityRepository) Create(_ context.Context, security *models.AccountSecurity) error {
	f.createCount++
	if security.ID == 0 {
		security.ID = f.nextID
		f.nextID++
	}
	now := time.Now()
	security.CreatedAt = now
	security.UpdatedAt = now
	f.records[security.UserID] = security
	return nil
}

func (f *fakeAccountSecurityRepository) Update(_ context.Context, security *models.AccountSecurity) error {
	f.updateCount++
	security.UpdatedAt = time.Now()
	f.records[security.UserID] = security
	return nil
}

func (f *fakeAccountSecurityRepository) GetOtpByAccountSecurity(_ context.Context, accountSecurityID uint64) (*models.Otp, error) {
	if otp, ok := f.otps[accountSecurityID]; ok {
		return otp, nil
	}
	return nil, nil
}

func (f *fakeAccountSecurityRepository) UpsertOtp(_ context.Context, otp *models.Otp) error {
	if otp.ID == 0 {
		otp.ID = f.nextOtpID
		f.nextOtpID++
	}
	now := time.Now()
	otp.CreatedAt = now
	otp.UpdatedAt = now
	otp.VerifiableType = "App\\Models\\AccountSecurity"
	f.otps[otp.VerifiableID] = otp
	return nil
}

func (f *fakeAccountSecurityRepository) DeleteOtp(_ context.Context, otpID uint64) error {
	for key, otp := range f.otps {
		if otp.ID == otpID {
			delete(f.otps, key)
			return nil
		}
	}
	return nil
}

var _ repository.AccountSecurityRepository = (*fakeAccountSecurityRepository)(nil)

type fakeActivityRepository struct {
	events []*models.UserEvent
}

func newFakeActivityRepository() *fakeActivityRepository {
	return &fakeActivityRepository{}
}

func (f *fakeActivityRepository) CreateUserEvent(_ context.Context, event *models.UserEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeActivityRepository) CreateActivity(context.Context, *models.UserActivity) error {
	panic("unexpected call to CreateActivity")
}

func (f *fakeActivityRepository) GetLatestActivity(context.Context, uint64) (*models.UserActivity, error) {
	panic("unexpected call to GetLatestActivity")
}

func (f *fakeActivityRepository) UpdateActivity(context.Context, *models.UserActivity) error {
	panic("unexpected call to UpdateActivity")
}

func (f *fakeActivityRepository) GetTotalActivityMinutes(context.Context, uint64) (int32, error) {
	panic("unexpected call to GetTotalActivityMinutes")
}

func (f *fakeActivityRepository) GetUserLog(context.Context, uint64) (*models.UserLog, error) {
	panic("unexpected call to GetUserLog")
}

func (f *fakeActivityRepository) CreateUserLog(context.Context, *models.UserLog) error {
	panic("unexpected call to CreateUserLog")
}

func (f *fakeActivityRepository) UpdateUserLog(context.Context, *models.UserLog) error {
	panic("unexpected call to UpdateUserLog")
}

func (f *fakeActivityRepository) IncrementLogField(context.Context, uint64, string, float64) error {
	panic("unexpected call to IncrementLogField")
}

func (f *fakeActivityRepository) CloseUserEventReport(context.Context, uint64) error {
	panic("unexpected call to CloseUserEventReport")
}

func (f *fakeActivityRepository) CreateUserEventReport(context.Context, *models.UserEventReport) error {
	panic("unexpected call to CreateUserEventReport")
}

func (f *fakeActivityRepository) CreateUserEventReportResponse(context.Context, *models.UserEventReportResponse) error {
	panic("unexpected call to CreateUserEventReportResponse")
}

func (f *fakeActivityRepository) GetUserEventByID(context.Context, uint64, uint64) (*models.UserEvent, error) {
	panic("unexpected call to GetUserEventByID")
}

func (f *fakeActivityRepository) GetUserEventsByUserID(context.Context, uint64, int32) ([]*models.UserEvent, error) {
	panic("unexpected call to GetUserEventsByUserID")
}

func (f *fakeActivityRepository) GetUserEventReportByEventID(context.Context, uint64) (*models.UserEventReport, error) {
	return nil, nil
}

func (f *fakeActivityRepository) UpdateUserEventReportStatus(context.Context, uint64, int32) error {
	panic("unexpected call to UpdateUserEventReportStatus")
}

func (f *fakeActivityRepository) GetUserEventReportResponses(context.Context, uint64) ([]*models.UserEventReportResponse, error) {
	panic("unexpected call to GetUserEventReportResponses")
}

var _ repository.ActivityRepository = (*fakeActivityRepository)(nil)

type fakeCacheRepository struct {
	state                    map[string]bool
	redirectTo               map[string]string
	backURL                  map[string]string
	ttl                      map[string]time.Duration
	setTime                  map[string]time.Time
	verificationRequestSlots map[uint64]time.Time
}

func newFakeCacheRepository() *fakeCacheRepository {
	return &fakeCacheRepository{
		state:                    make(map[string]bool),
		redirectTo:               make(map[string]string),
		backURL:                  make(map[string]string),
		ttl:                      make(map[string]time.Duration),
		setTime:                  make(map[string]time.Time),
		verificationRequestSlots: make(map[uint64]time.Time),
	}
}

func (f *fakeCacheRepository) SetState(_ context.Context, state string, ttl time.Duration) error {
	f.state["oauth:state:"+state] = true
	f.ttl["oauth:state:"+state] = ttl
	f.setTime["oauth:state:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetState(_ context.Context, state string) (bool, error) {
	key := "oauth:state:" + state
	exists := f.state[key]
	if exists {
		delete(f.state, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return exists, nil
}

func (f *fakeCacheRepository) SetRedirectTo(_ context.Context, state, redirectTo string, ttl time.Duration) error {
	f.redirectTo["oauth:redirect_to:"+state] = redirectTo
	f.ttl["oauth:redirect_to:"+state] = ttl
	f.setTime["oauth:redirect_to:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetRedirectTo(_ context.Context, state string) (string, error) {
	key := "oauth:redirect_to:" + state
	val := f.redirectTo[key]
	if val != "" {
		delete(f.redirectTo, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return val, nil
}

func (f *fakeCacheRepository) SetBackURL(_ context.Context, state, backURL string, ttl time.Duration) error {
	f.backURL["oauth:back_url:"+state] = backURL
	f.ttl["oauth:back_url:"+state] = ttl
	f.setTime["oauth:back_url:"+state] = time.Now()
	return nil
}

func (f *fakeCacheRepository) GetBackURL(_ context.Context, state string) (string, error) {
	key := "oauth:back_url:" + state
	val := f.backURL[key]
	if val != "" {
		delete(f.backURL, key)
		delete(f.ttl, key)
		delete(f.setTime, key)
	}
	return val, nil
}

func (f *fakeCacheRepository) TryAcquireAccountSecurityVerificationSlot(_ context.Context, userID uint64, period time.Duration) (bool, error) {
	if until, exists := f.verificationRequestSlots[userID]; exists && time.Now().Before(until) {
		return false, nil
	}
	f.verificationRequestSlots[userID] = time.Now().Add(period)
	return true, nil
}

func (f *fakeCacheRepository) SetWeb3LinkNonce(context.Context, uint64, string, string, time.Duration) error {
	return nil
}

func (f *fakeCacheRepository) PullWeb3LinkNonce(context.Context, uint64, string) (string, error) {
	return "", nil
}

func (f *fakeCacheRepository) SetWeb3SecurityNonce(context.Context, uint64, string, string, time.Duration) error {
	return nil
}

func (f *fakeCacheRepository) PullWeb3SecurityNonce(context.Context, uint64, string) (string, error) {
	return "", nil
}

var _ repository.CacheRepository = (*fakeCacheRepository)(nil)

type fakeSMSServiceClient struct {
	lastRequest *notificationspb.SendOTPRequest
	err         error
}

func (f *fakeSMSServiceClient) SendSMS(context.Context, *notificationspb.SendSMSRequest, ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	panic("unexpected call to SendSMS")
}

func (f *fakeSMSServiceClient) SendOTP(_ context.Context, req *notificationspb.SendOTPRequest, _ ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	f.lastRequest = req
	if f.err != nil {
		return nil, f.err
	}
	return &notificationspb.SMSResponse{Sent: true}, nil
}

var _ notificationspb.SMSServiceClient = (*fakeSMSServiceClient)(nil)
