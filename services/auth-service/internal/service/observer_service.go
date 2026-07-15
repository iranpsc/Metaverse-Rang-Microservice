package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/pubsub"
	"metarang/auth-service/internal/repository"
	notificationspb "metarang/shared/pb/notifications"
)

// ObserverService handles user events, activity tracking, and score calculation
// This mimics Laravel's UserObserver functionality
type ObserverService interface {
	// Login event handler
	OnUserLogin(ctx context.Context, user *models.User, ip, userAgent string) error

	// Logout event handler
	OnUserLogout(ctx context.Context, user *models.User, ip, userAgent string) error

	// User created event handler
	OnUserCreated(ctx context.Context, user *models.User) error

	// Hour reached event (called when activity is logged out)
	OnHourReached(ctx context.Context, user *models.User) error

	// Calculate and update user score
	CalculateScore(ctx context.Context, user *models.User) error
}

type observerService struct {
	userRepo           repository.UserRepository
	settingsRepo       repository.SettingsRepository
	activityRepo       repository.ActivityRepository
	publisher          pubsub.RedisPublisher
	notificationClient notificationspb.NotificationServiceClient
}

func NewObserverService(
	userRepo repository.UserRepository,
	activityRepo repository.ActivityRepository,
	publisher pubsub.RedisPublisher,
) ObserverService {
	return &observerService{
		userRepo:     userRepo,
		activityRepo: activityRepo,
		publisher:    publisher,
	}
}

func NewObserverServiceWithSettings(
	userRepo repository.UserRepository,
	settingsRepo repository.SettingsRepository,
	activityRepo repository.ActivityRepository,
	publisher pubsub.RedisPublisher,
	notificationClient notificationspb.NotificationServiceClient,
) ObserverService {
	return &observerService{
		userRepo:           userRepo,
		settingsRepo:       settingsRepo,
		activityRepo:       activityRepo,
		publisher:          publisher,
		notificationClient: notificationClient,
	}
}

// OnUserLogin handles the login event
// Implements the Laravel UserObserver::logedIn method
func (s *observerService) OnUserLogin(ctx context.Context, user *models.User, ip, userAgent string) error {
	// 1. Create user event
	event := &models.UserEvent{
		UserID: user.ID,
		Event:  "ورود به حساب کاربری", // "Login to user account" in Persian
		IP:     ip,
		Device: userAgent,
		Status: 1,
	}
	if err := s.activityRepo.CreateUserEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create login event: %w", err)
	}

	// 2. Update last_seen
	if err := s.userRepo.UpdateLastSeen(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	// 3. Send login notification (Laravel LogedInNotification)
	if err := s.sendLoggedInNotification(ctx, user, ip); err != nil {
		// Log error but don't fail the login
		fmt.Printf("failed to send login notification: %v\n", err)
	}

	// 4. Create activity tracking record
	activity := &models.UserActivity{
		UserID: user.ID,
		Start:  time.Now(),
		IP:     ip,
	}
	if err := s.activityRepo.CreateActivity(ctx, activity); err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}

	// 5. Broadcast WebSocket online status (Laravel UserStatusChanged)
	if s.publisher != nil {
		if err := s.publisher.PublishUserStatusChanged(ctx, user.ID, true); err != nil {
			// Log error but don't fail the login
			fmt.Printf("failed to publish user status: %v\n", err)
		}
	}

	return nil
}

// sendLoggedInNotification mirrors Laravel LogedInNotification channel selection.
func (s *observerService) sendLoggedInNotification(ctx context.Context, user *models.User, ip string) error {
	if s.notificationClient == nil {
		return nil
	}

	sendSMS, sendEmail := s.loginNotificationChannels(ctx, user)

	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.notificationClient.SendNotification(notifyCtx, &notificationspb.SendNotificationRequest{
		UserId:  user.ID,
		Type:    "login",
		Title:   "ورود به حساب کاربری",
		Message: "شما با موفقیت وارد حساب کاربری خود شدید.",
		Data: map[string]string{
			"ip":           ip,
			"related-to":   "events",
			"sender-name":  "متارنگ",
			"sender-image": "uploads/img/logo.png",
		},
		SendSms:   sendSMS,
		SendEmail: sendEmail,
	})
	if err != nil {
		return fmt.Errorf("notification service: %w", err)
	}
	return nil
}

func (s *observerService) loginNotificationChannels(ctx context.Context, user *models.User) (sendSMS, sendEmail bool) {
	settings, err := s.getUserSettings(ctx, user.ID)
	if err != nil || settings == nil || settings.Notifications == nil {
		return false, false
	}

	sendEmail = settings.Notifications["login_verification_email"]
	// Laravel only sends SMS when phone is verified
	hasVerifiedPhone := user.Phone.Valid && strings.TrimSpace(user.Phone.String) != "" && user.PhoneVerifiedAt.Valid
	sendSMS = hasVerifiedPhone && settings.Notifications["login_verification_sms"]
	return sendSMS, sendEmail
}

func (s *observerService) getUserSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	if s.settingsRepo != nil {
		settings, err := s.settingsRepo.FindByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if settings != nil {
			return settings, nil
		}
	}
	return s.userRepo.GetSettings(ctx, userID)
}

// OnUserLogout handles the logout event
// Implements the Laravel UserObserver::logedOut method
func (s *observerService) OnUserLogout(ctx context.Context, user *models.User, ip, userAgent string) error {
	// 1. Get latest activity
	latestActivity, err := s.activityRepo.GetLatestActivity(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest activity: %w", err)
	}

	if latestActivity != nil {
		// 2. Update activity end time and total minutes
		endTime := time.Now()
		totalMinutes := int32(endTime.Sub(latestActivity.Start).Minutes())

		latestActivity.End = sql.NullTime{Time: endTime, Valid: true}
		latestActivity.Total = totalMinutes
		latestActivity.IP = ip

		if err := s.activityRepo.UpdateActivity(ctx, latestActivity); err != nil {
			return fmt.Errorf("failed to update activity: %w", err)
		}
	}

	// 3. Trigger hour reached event for score calculation
	if err := s.OnHourReached(ctx, user); err != nil {
		// Log error but don't fail the logout
		fmt.Printf("failed to calculate score on logout: %v\n", err)
	}

	// 4. Set last_seen to 2 minutes ago (marks as offline)
	// We'll update the user directly since we need a special time
	// Note: This is a special case where we can't use UpdateLastSeen
	// TODO: Add UpdateLastSeenWithTime method to repository
	twoMinutesAgo := time.Now().Add(-2 * time.Minute)
	user.LastSeen = sql.NullTime{Time: twoMinutesAgo, Valid: true}

	// 5. Create logout event
	event := &models.UserEvent{
		UserID: user.ID,
		Event:  "خروج از حساب کاربری", // "Logout from user account" in Persian
		IP:     ip,
		Device: userAgent,
		Status: 1,
	}
	if err := s.activityRepo.CreateUserEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create logout event: %w", err)
	}

	// 6. Broadcast offline status
	if err := s.publisher.PublishUserStatusChanged(ctx, user.ID, false); err != nil {
		// Log error but don't fail the logout
		fmt.Printf("failed to publish user offline status: %v\n", err)
	}

	return nil
}

// OnUserCreated handles the user creation event
// Implements the Laravel UserObserver::created method
//
// NOTE: In microservices architecture, the following Laravel user creation tasks
// are distributed across services and must be coordinated:
//
// 1. Email verification (handled in Auth service - see below)
// 2. Wallet creation (must call Commercial service via gRPC)
// 3. Settings creation (handled in Auth service - see below)
// 4. User log creation (handled below)
// 5. User variables creation (must call Commercial service via gRPC)
// 6. Initial activity creation (handled below)
//
// The caller (user registration endpoint) must orchestrate these calls:
// - After creating user in Auth service
// - Call Commercial service to create wallet and variables
// - Call this OnUserCreated method
// - Return success to client
func (s *observerService) OnUserCreated(ctx context.Context, user *models.User) error {
	// 1. Mark email as verified (Laravel does this immediately on creation)
	if err := s.userRepo.MarkEmailAsVerified(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to mark email as verified: %w", err)
	}

	// 2. Create default settings (automatic_logout = 55 minutes by default)
	settings := &models.Settings{
		UserID:            user.ID,
		Status:            true,
		Level:             true,
		Details:           true,
		CheckoutDaysCount: 3,
		AutomaticLogout:   55,
		Privacy:           models.DefaultPrivacySettings(),
		Notifications:     models.DefaultNotificationSettings(),
	}

	// Use SettingsRepository if available, otherwise fall back to UserRepository
	if s.settingsRepo != nil {
		if err := s.settingsRepo.Create(ctx, settings); err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	} else {
		// Fallback for backward compatibility
		if err := s.userRepo.CreateSettings(ctx, settings); err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	}

	// 3. Create user log for score tracking
	log := &models.UserLog{
		UserID:            user.ID,
		TransactionsCount: 0,
		FollowersCount:    0,
		DepositAmount:     0,
		ActivityHours:     0,
		Score:             0,
	}
	if err := s.activityRepo.CreateUserLog(ctx, log); err != nil {
		return fmt.Errorf("failed to create user log: %w", err)
	}

	// 4. Create initial activity record
	activity := &models.UserActivity{
		UserID: user.ID,
		Start:  time.Now(),
		IP:     user.IP,
	}
	if err := s.activityRepo.CreateActivity(ctx, activity); err != nil {
		return fmt.Errorf("failed to create initial activity: %w", err)
	}

	// Wallet and user_variables are created by AuthService.Callback via HelperService
	// (Commercial service gRPC CreateWallet / CreateUserVariables).

	return nil
}

// OnHourReached calculates activity hours and updates user score
// Implements the Laravel UserObserver::hourReached method
func (s *observerService) OnHourReached(ctx context.Context, user *models.User) error {
	// 1. Get total active minutes
	totalActiveMinutes, err := s.activityRepo.GetTotalActivityMinutes(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get total activity minutes: %w", err)
	}

	// 2. Calculate activity hours score (ceil(minutes / 60) * 0.1)
	activityHours := math.Ceil(float64(totalActiveMinutes)/60) * 0.1

	// 3. Get user log
	log, err := s.activityRepo.GetUserLog(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get user log: %w", err)
	}

	if log != nil {
		// 4. Update activity hours
		log.ActivityHours = activityHours
		if err := s.activityRepo.UpdateUserLog(ctx, log); err != nil {
			return fmt.Errorf("failed to update user log: %w", err)
		}
	}

	// 5. Calculate and update total score
	return s.CalculateScore(ctx, user)
}

// CalculateScore calculates and updates user score based on all metrics
// Implements the Laravel UserObserver::calculateScore method
func (s *observerService) CalculateScore(ctx context.Context, user *models.User) error {
	// 1. Get user log
	log, err := s.activityRepo.GetUserLog(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get user log: %w", err)
	}

	if log == nil {
		return fmt.Errorf("user log not found for user %d", user.ID)
	}

	// 2. Calculate total score
	totalScore := log.TransactionsCount +
		log.FollowersCount +
		log.DepositAmount +
		log.ActivityHours

	// 3. Update log with new score
	log.Score = totalScore
	if err := s.activityRepo.UpdateUserLog(ctx, log); err != nil {
		return fmt.Errorf("failed to update user log score: %w", err)
	}

	// 4. Update user score
	user.Score = int32(totalScore)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user score: %w", err)
	}

	// 5. Check for level progression (TODO: integrate with Levels service)
	// This should call the Levels service via gRPC to:
	// - Check if user has reached new level
	// - Award prizes if eligible
	// - Attach level to user
	//
	// Example:
	// levelResp, err := s.levelsClient.CheckLevelProgression(ctx, &pb.CheckLevelProgressionRequest{
	//     UserId: user.ID,
	//     Score:  int32(totalScore),
	// })
	// if err == nil && levelResp.NewLevel != nil {
	//     // Level up achieved!
	//     // Prizes are awarded by the Levels service
	// }

	return nil
}
