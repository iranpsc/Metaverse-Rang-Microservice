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
	users                           map[uint64]*models.User
	updateFunc                      func(context.Context, *models.User) error
	listUsersFunc                   func(context.Context, string, string, int32, int32) ([]*repository.UserWithRelations, int32, error)
	getUsersLevelsForListFunc       func(context.Context, []uint64) (map[uint64]*repository.UserListLevels, error)
	getUserLatestLevelFunc          func(context.Context, uint64) (*repository.UserLevel, error)
	getLevelsBelowScoreFunc         func(context.Context, int32) ([]*repository.UserLevel, error)
	getNextLevelScoreFunc           func(context.Context, int32) (int32, error)
	getFollowersCountFunc           func(context.Context, uint64) (int32, error)
	getFollowingCountFunc           func(context.Context, uint64) (int32, error)
	getAllProfilePhotoURLsFunc      func(context.Context, uint64) ([]string, error)
	getFeatureCountsFunc            func(context.Context, uint64) (int32, int32, int32, error)
	getSettingsFunc                 func(context.Context, uint64) (*models.Settings, error)
	getKYCFunc                      func(context.Context, uint64) (*models.KYC, error)
	getUnreadNotificationsCountFunc func(context.Context, uint64) (int32, error)
	getLatestProfilePhotoURLFunc    func(context.Context, uint64) (string, error)
	isPhoneTakenFunc                func(context.Context, string, uint64) (bool, error)
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

func (f *fakeUserRepository) Update(ctx context.Context, user *models.User) error {
	if f.updateFunc != nil {
		return f.updateFunc(ctx, user)
	}
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

func (f *fakeUserRepository) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	if f.getSettingsFunc != nil {
		return f.getSettingsFunc(ctx, userID)
	}
	panic("unexpected call to GetSettings")
}

func (f *fakeUserRepository) CreateSettings(_ context.Context, settings *models.Settings) error {
	if f.getSettingsFunc != nil {
		// allow Create then Get via getSettingsFunc store pattern
	}
	_ = settings
	return nil
}

func (f *fakeUserRepository) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	if f.getKYCFunc != nil {
		return f.getKYCFunc(ctx, userID)
	}
	panic("unexpected call to GetKYC")
}

func (f *fakeUserRepository) GetUnreadNotificationsCount(ctx context.Context, userID uint64) (int32, error) {
	if f.getUnreadNotificationsCountFunc != nil {
		return f.getUnreadNotificationsCountFunc(ctx, userID)
	}
	return 0, nil
}

func (f *fakeUserRepository) MarkEmailAsVerified(_ context.Context, userID uint64) error {
	if user, ok := f.users[userID]; ok {
		user.EmailVerifiedAt = sql.NullTime{Time: time.Now(), Valid: true}
		return nil
	}
	return nil
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

func (f *fakeUserRepository) FindByWalletAddress(context.Context, string) (*models.User, error) {
	panic("unexpected call to FindByWalletAddress")
}

func (f *fakeUserRepository) LinkWalletAddress(context.Context, uint64, string) (repository.LinkWalletResult, error) {
	panic("unexpected call to LinkWalletAddress")
}

func (f *fakeUserRepository) IsPhoneTaken(ctx context.Context, phone string, excludeUserID uint64) (bool, error) {
	if f.isPhoneTakenFunc != nil {
		return f.isPhoneTakenFunc(ctx, phone, excludeUserID)
	}
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

func (f *fakeUserRepository) ListUsers(ctx context.Context, search, orderBy string, page, limit int32) ([]*repository.UserWithRelations, int32, error) {
	if f.listUsersFunc != nil {
		return f.listUsersFunc(ctx, search, orderBy, page, limit)
	}
	panic("unexpected call to ListUsers")
}

func (f *fakeUserRepository) GetUsersLevelsForList(ctx context.Context, userIDs []uint64) (map[uint64]*repository.UserListLevels, error) {
	if f.getUsersLevelsForListFunc != nil {
		return f.getUsersLevelsForListFunc(ctx, userIDs)
	}
	panic("unexpected call to GetUsersLevelsForList")
}

func (f *fakeUserRepository) GetFollowersCount(ctx context.Context, userID uint64) (int32, error) {
	if f.getFollowersCountFunc != nil {
		return f.getFollowersCountFunc(ctx, userID)
	}
	panic("unexpected call to GetFollowersCount")
}

func (f *fakeUserRepository) GetFollowingCount(ctx context.Context, userID uint64) (int32, error) {
	if f.getFollowingCountFunc != nil {
		return f.getFollowingCountFunc(ctx, userID)
	}
	panic("unexpected call to GetFollowingCount")
}

func (f *fakeUserRepository) GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error) {
	if f.getLatestProfilePhotoURLFunc != nil {
		return f.getLatestProfilePhotoURLFunc(ctx, userID)
	}
	return "", nil
}

func (f *fakeUserRepository) GetAllProfilePhotoURLs(ctx context.Context, userID uint64) ([]string, error) {
	if f.getAllProfilePhotoURLsFunc != nil {
		return f.getAllProfilePhotoURLsFunc(ctx, userID)
	}
	panic("unexpected call to GetAllProfilePhotoURLs")
}

func (f *fakeUserRepository) GetUserLatestLevel(ctx context.Context, userID uint64) (*repository.UserLevel, error) {
	if f.getUserLatestLevelFunc != nil {
		return f.getUserLatestLevelFunc(ctx, userID)
	}
	panic("unexpected call to GetUserLatestLevel")
}

func (f *fakeUserRepository) GetLevelsBelowScore(ctx context.Context, score int32) ([]*repository.UserLevel, error) {
	if f.getLevelsBelowScoreFunc != nil {
		return f.getLevelsBelowScoreFunc(ctx, score)
	}
	panic("unexpected call to GetLevelsBelowScore")
}

func (f *fakeUserRepository) GetNextLevelScore(ctx context.Context, currentScore int32) (int32, error) {
	if f.getNextLevelScoreFunc != nil {
		return f.getNextLevelScoreFunc(ctx, currentScore)
	}
	panic("unexpected call to GetNextLevelScore")
}

func (f *fakeUserRepository) GetFeatureCounts(ctx context.Context, userID uint64) (int32, int32, int32, error) {
	if f.getFeatureCountsFunc != nil {
		return f.getFeatureCountsFunc(ctx, userID)
	}
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
	events                          []*models.UserEvent
	reports                         map[uint64]*models.UserEventReport
	responses                       map[uint64][]*models.UserEventReportResponse
	nextReportID                    uint64
	nextResponseID                  uint64
	getUserEventsByUserIDFunc       func(context.Context, uint64, int32) ([]*models.UserEvent, error)
	getUserEventByIDFunc            func(context.Context, uint64, uint64) (*models.UserEvent, error)
	getUserEventReportByEventIDFunc func(context.Context, uint64) (*models.UserEventReport, error)
	getUserEventReportResponsesFunc func(context.Context, uint64) ([]*models.UserEventReportResponse, error)
	activities                      []*models.UserActivity
	latestActivity                  map[uint64]*models.UserActivity
	userLogs                        map[uint64]*models.UserLog
}

func newFakeActivityRepository() *fakeActivityRepository {
	return &fakeActivityRepository{
		reports:        make(map[uint64]*models.UserEventReport),
		responses:      make(map[uint64][]*models.UserEventReportResponse),
		nextReportID:   1,
		nextResponseID: 1,
		latestActivity: make(map[uint64]*models.UserActivity),
		userLogs:       make(map[uint64]*models.UserLog),
	}
}

func (f *fakeActivityRepository) CreateUserEvent(_ context.Context, event *models.UserEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeActivityRepository) CreateActivity(_ context.Context, activity *models.UserActivity) error {
	f.activities = append(f.activities, activity)
	f.latestActivity[activity.UserID] = activity
	return nil
}

func (f *fakeActivityRepository) GetLatestActivity(_ context.Context, userID uint64) (*models.UserActivity, error) {
	return f.latestActivity[userID], nil
}

func (f *fakeActivityRepository) UpdateActivity(_ context.Context, activity *models.UserActivity) error {
	f.latestActivity[activity.UserID] = activity
	return nil
}

func (f *fakeActivityRepository) GetTotalActivityMinutes(context.Context, uint64) (int32, error) {
	return 120, nil
}

func (f *fakeActivityRepository) GetUserLog(_ context.Context, userID uint64) (*models.UserLog, error) {
	return f.userLogs[userID], nil
}

func (f *fakeActivityRepository) CreateUserLog(_ context.Context, log *models.UserLog) error {
	f.userLogs[log.UserID] = log
	return nil
}

func (f *fakeActivityRepository) UpdateUserLog(_ context.Context, log *models.UserLog) error {
	f.userLogs[log.UserID] = log
	return nil
}

func (f *fakeActivityRepository) IncrementLogField(_ context.Context, userID uint64, _ string, amount float64) error {
	log := f.userLogs[userID]
	if log == nil {
		log = &models.UserLog{UserID: userID}
		f.userLogs[userID] = log
	}
	log.Score += amount
	return nil
}

func (f *fakeActivityRepository) CloseUserEventReport(_ context.Context, reportID uint64) error {
	for _, report := range f.reports {
		if report.ID == reportID {
			report.Closed = true
			return nil
		}
	}
	return nil
}

func (f *fakeActivityRepository) CreateUserEventReport(_ context.Context, report *models.UserEventReport) error {
	if report.ID == 0 {
		report.ID = f.nextReportID
		f.nextReportID++
	}
	f.reports[report.UserEventID] = report
	return nil
}

func (f *fakeActivityRepository) CreateUserEventReportResponse(_ context.Context, response *models.UserEventReportResponse) error {
	if response.ID == 0 {
		response.ID = f.nextResponseID
		f.nextResponseID++
	}
	f.responses[response.UserEventReportID] = append(f.responses[response.UserEventReportID], response)
	return nil
}

func (f *fakeActivityRepository) GetUserEventByID(ctx context.Context, userID, eventID uint64) (*models.UserEvent, error) {
	if f.getUserEventByIDFunc != nil {
		return f.getUserEventByIDFunc(ctx, userID, eventID)
	}
	for _, event := range f.events {
		if event.ID == eventID && event.UserID == userID {
			return event, nil
		}
	}
	return nil, nil
}

func (f *fakeActivityRepository) GetUserEventsByUserID(ctx context.Context, userID uint64, page int32) ([]*models.UserEvent, error) {
	if f.getUserEventsByUserIDFunc != nil {
		return f.getUserEventsByUserIDFunc(ctx, userID, page)
	}
	var out []*models.UserEvent
	for _, event := range f.events {
		if event.UserID == userID {
			out = append(out, event)
		}
	}
	return out, nil
}

func (f *fakeActivityRepository) GetUserEventReportByEventID(ctx context.Context, eventID uint64) (*models.UserEventReport, error) {
	if f.getUserEventReportByEventIDFunc != nil {
		return f.getUserEventReportByEventIDFunc(ctx, eventID)
	}
	return f.reports[eventID], nil
}

func (f *fakeActivityRepository) UpdateUserEventReportStatus(_ context.Context, reportID uint64, status int32) error {
	for _, report := range f.reports {
		if report.ID == reportID {
			report.Status = status
			return nil
		}
	}
	return nil
}

func (f *fakeActivityRepository) GetUserEventReportResponses(ctx context.Context, reportID uint64) ([]*models.UserEventReportResponse, error) {
	if f.getUserEventReportResponsesFunc != nil {
		return f.getUserEventReportResponsesFunc(ctx, reportID)
	}
	return f.responses[reportID], nil
}

var _ repository.ActivityRepository = (*fakeActivityRepository)(nil)

type fakeCacheRepository struct {
	state                    map[string]bool
	redirectTo               map[string]string
	backURL                  map[string]string
	ttl                      map[string]time.Duration
	setTime                  map[string]time.Time
	verificationRequestSlots map[uint64]time.Time
	mobileChangeSendSlots    map[uint64]time.Time
	mobileChangeChallenges   map[uint64]*repository.MobileChangeChallenge
}

func newFakeCacheRepository() *fakeCacheRepository {
	return &fakeCacheRepository{
		state:                    make(map[string]bool),
		redirectTo:               make(map[string]string),
		backURL:                  make(map[string]string),
		ttl:                      make(map[string]time.Duration),
		setTime:                  make(map[string]time.Time),
		verificationRequestSlots: make(map[uint64]time.Time),
		mobileChangeSendSlots:    make(map[uint64]time.Time),
		mobileChangeChallenges:   make(map[uint64]*repository.MobileChangeChallenge),
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

func (f *fakeCacheRepository) TryAcquireMobileChangeSendSlot(_ context.Context, userID uint64, period time.Duration) (bool, error) {
	if until, exists := f.mobileChangeSendSlots[userID]; exists && time.Now().Before(until) {
		return false, nil
	}
	f.mobileChangeSendSlots[userID] = time.Now().Add(period)
	return true, nil
}

func (f *fakeCacheRepository) ReleaseMobileChangeSendSlot(_ context.Context, userID uint64) error {
	delete(f.mobileChangeSendSlots, userID)
	return nil
}

func (f *fakeCacheRepository) SaveMobileChangeChallenge(_ context.Context, userID uint64, challenge *repository.MobileChangeChallenge, _ time.Duration) error {
	copied := *challenge
	f.mobileChangeChallenges[userID] = &copied
	return nil
}

func (f *fakeCacheRepository) GetMobileChangeChallenge(_ context.Context, userID uint64) (*repository.MobileChangeChallenge, error) {
	challenge, ok := f.mobileChangeChallenges[userID]
	if !ok || challenge == nil {
		return nil, nil
	}
	copied := *challenge
	return &copied, nil
}

func (f *fakeCacheRepository) DeleteMobileChangeChallenge(_ context.Context, userID uint64) error {
	delete(f.mobileChangeChallenges, userID)
	return nil
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
