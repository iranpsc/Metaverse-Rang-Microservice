// Package service implements business logic for the levels service.
package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"metarang/levels-service/internal/client"
	pb "metarang/shared/pb/levels"
)

type activityRepository interface {
	CreateActivity(ctx context.Context, req *pb.LogActivityRequest) (uint64, error)
	CreateUserEvent(ctx context.Context, userID uint64, event, ip, device string, status int8) error
	FindByUserID(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, error)
	GetLatestActivity(ctx context.Context, userID uint64) (*pb.UserActivity, error)
	UpdateActivity(ctx context.Context, activityID uint64, endTime time.Time, totalMinutes int32) error
	GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error)
	GetVariableRate(ctx context.Context, name string) (float64, error)
	GetSignificantTradeCount(ctx context.Context, userID uint64, minIrrAmount, minPscAmount float64) (int32, error)
}

type activityUserLogRepository interface {
	GetUserLog(ctx context.Context, userID uint64) (*pb.UserLog, error)
	CalculateScore(ctx context.Context, userID uint64) (int32, error)
	UpdateScore(ctx context.Context, userID uint64, score int32) error
	UpdateTransactionsCount(ctx context.Context, userID uint64, count string) error
	IncrementDeposit(ctx context.Context, userID uint64, amount string) error
	GetTotalFollowers(ctx context.Context, userID uint64) (int32, error)
	UpdateFollowersCount(ctx context.Context, userID uint64, totalFollowers int32) error
	UpdateActivityHours(ctx context.Context, userID uint64, totalMinutes int32) error
}

type activityLevelRepository interface {
	GetNextLevelForScore(ctx context.Context, userID uint64, score int32) (*pb.Level, error)
	AttachLevelToUser(ctx context.Context, userID, levelID uint64) error
	GetLevelPrize(ctx context.Context, levelID uint64) (*pb.LevelPrize, error)
	HasUserReceivedPrize(ctx context.Context, userID, prizeID uint64) (bool, error)
	RecordReceivedPrize(ctx context.Context, userID, prizeID uint64) error
}

type ActivityService struct {
	activityRepo     activityRepository
	userLogRepo      activityUserLogRepository
	levelRepo        activityLevelRepository
	commercialClient client.CommercialClient
	defaultPSCRate   float64
}

func NewActivityService(
	activityRepo activityRepository,
	userLogRepo activityUserLogRepository,
	levelRepo activityLevelRepository,
	commercialClient client.CommercialClient,
) *ActivityService {
	return &ActivityService{
		activityRepo:     activityRepo,
		userLogRepo:      userLogRepo,
		levelRepo:        levelRepo,
		commercialClient: commercialClient,
		defaultPSCRate:   30000,
	}
}

// LogActivity records user activity
// Implements Laravel: UserObserver@logedIn
func (s *ActivityService) LogActivity(ctx context.Context, req *pb.LogActivityRequest) (uint64, error) {
	activityID, err := s.activityRepo.CreateActivity(ctx, req)
	if err != nil {
		return 0, err
	}

	// Create user event
	status := int8(1)
	event := "ورود به حساب کاربری" // Login in Persian
	if req.EventType == "logout" {
		event = "خروج از حساب کاربری" // Logout in Persian
	}

	_ = s.activityRepo.CreateUserEvent(ctx, req.UserId, event, req.Ip, req.Device, status)

	return activityID, nil
}

// GetUserActivities retrieves user's activity history
func (s *ActivityService) GetUserActivities(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, *pb.UserLog, error) {
	activities, err := s.activityRepo.FindByUserID(ctx, userID, limit)
	if err != nil {
		return nil, nil, err
	}

	userLog, err := s.userLogRepo.GetUserLog(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return activities, userLog, nil
}

// UpdateActivityScore recalculates user score
// Implements Laravel: UserObserver@calculateScore
func (s *ActivityService) UpdateActivityScore(ctx context.Context, userID uint64) (int32, bool, uint64, error) {
	// Calculate the new score
	newScore, err := s.userLogRepo.CalculateScore(ctx, userID)
	if err != nil {
		return 0, false, 0, err
	}

	// Update score in user_logs and users tables
	if err := s.userLogRepo.UpdateScore(ctx, userID, newScore); err != nil {
		return 0, false, 0, err
	}

	// Check if user reached a new level
	// Implements Laravel: Level::where('score', '<=', $user->score)->whereNotIn('id', $user->levels->pluck('id'))->with('prize')->first()
	nextLevel, err := s.levelRepo.GetNextLevelForScore(ctx, userID, newScore)
	levelUp := false
	var newLevelID uint64

	if err == nil && nextLevel != nil {
		// User reached new level
		levelUp = true
		newLevelID = nextLevel.Id

		// Attach level to user
		if err := s.levelRepo.AttachLevelToUser(ctx, userID, newLevelID); err != nil {
			return newScore, false, 0, err
		}

		// Award prize automatically (matching Laravel behavior)
		// TODO: Implement this by calling commercial service to update wallet
		// For now, just record the prize as received
		prize, err := s.levelRepo.GetLevelPrize(ctx, newLevelID)
		if err == nil && prize != nil {
			// Check if user can receive prize (not already received)
			hasReceived, _ := s.levelRepo.HasUserReceivedPrize(ctx, userID, prize.Id)
			if !hasReceived {
				pscRate, rateErr := s.activityRepo.GetVariableRate(ctx, "psc")
				if rateErr != nil || pscRate <= 0 {
					pscRate = s.defaultPSCRate
				}
				if err := applyLevelPrizeBalances(ctx, s.commercialClient, userID, prize, pscRate); err != nil {
					return newScore, false, 0, fmt.Errorf("failed to apply level-up prize balances: %w", err)
				}

				// Record prize as received
				if err := s.levelRepo.RecordReceivedPrize(ctx, userID, prize.Id); err != nil {
					return newScore, false, 0, err
				}
			}
		}
	}

	return newScore, levelUp, newLevelID, nil
}

// RecordTrade records trade for score calculation
// Implements Laravel: UserObserver@traded
func (s *ActivityService) RecordTrade(ctx context.Context, userID uint64, irrAmount, pscAmount string) error {
	// Count significant trades (irr > 7000000 OR psc > equivalent)
	// Implements Laravel: UserObserver@getSignificantTradeCount

	// Parse amounts
	irr, _ := strconv.ParseFloat(irrAmount, 64)
	psc, _ := strconv.ParseFloat(pscAmount, 64)

	minIrrAmount := float64(7000000)
	pscRate, err := s.activityRepo.GetVariableRate(ctx, "psc")
	if err != nil || pscRate <= 0 {
		pscRate = s.defaultPSCRate
	}
	minPscAmount := minIrrAmount / pscRate

	// Check if this trade is significant
	if irr < minIrrAmount && psc < minPscAmount {
		// Trade is not significant, don't count it
		return nil
	}

	trades, err := s.activityRepo.GetSignificantTradeCount(ctx, userID, minIrrAmount, minPscAmount)
	if err != nil {
		return err
	}
	if err := s.userLogRepo.UpdateTransactionsCount(ctx, userID, fmt.Sprintf("%d", trades*2)); err != nil {
		return err
	}

	// After updating count, recalculate score
	return s.recalculateAndUpdateScore(ctx, userID)
}

// RecordDeposit records deposit for score calculation
// Implements Laravel: UserObserver@deposit
func (s *ActivityService) RecordDeposit(ctx context.Context, userID uint64, amount string) error {
	// Increment deposit_amount by amount * 0.0001
	// Laravel: $user->log->increment('deposit_amount', $amount * 0.0001)
	if err := s.userLogRepo.IncrementDeposit(ctx, userID, amount); err != nil {
		return err
	}

	// Recalculate score
	return s.recalculateAndUpdateScore(ctx, userID)
}

// RecordFollower records follower for score calculation
// Implements Laravel: UserObserver@followed
func (s *ActivityService) RecordFollower(ctx context.Context, userID uint64) error {
	// Count total followers
	// Laravel: $totalFollowers = $user->followers->count()
	totalFollowers, err := s.userLogRepo.GetTotalFollowers(ctx, userID)
	if err != nil {
		return err
	}

	// Update followers_count (count * 0.1)
	// Laravel: $user->log->update(['followers_count' => $totalFollowers * 0.1])
	if err := s.userLogRepo.UpdateFollowersCount(ctx, userID, totalFollowers); err != nil {
		return err
	}

	// Recalculate score
	return s.recalculateAndUpdateScore(ctx, userID)
}

// LogLogout records user logout and updates activity hours
// Implements Laravel: UserObserver@logedOut
func (s *ActivityService) LogLogout(ctx context.Context, userID uint64, ip string) error {
	// Get latest activity
	latestActivity, err := s.activityRepo.GetLatestActivity(ctx, userID)
	if err != nil {
		return err
	}

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, latestActivity.Start)
	if err != nil {
		return err
	}

	// Calculate total minutes
	endTime := time.Now()
	totalMinutes := int32(endTime.Sub(startTime).Minutes())

	// Update activity with end time and total
	// Laravel: $latestActivity->update(['end' => now(), 'total' => $latestActivity->start->diffInMinutes(now())])
	if err := s.activityRepo.UpdateActivity(ctx, latestActivity.Id, endTime, totalMinutes); err != nil {
		return err
	}

	// Call hourReached
	return s.HourReached(ctx, userID)
}

// HourReached recalculates activity hours score
// Implements Laravel: UserObserver@hourReached
func (s *ActivityService) HourReached(ctx context.Context, userID uint64) error {
	// Get total active minutes
	// Laravel: $totalActiveHours = $user->activities->sum('total')
	totalMinutes, err := s.activityRepo.GetTotalActivityMinutes(ctx, userID)
	if err != nil {
		return err
	}

	// Update activity_hours (ceil(minutes / 60) * 0.1)
	// Laravel: $user->log->update(['activity_hours' => ceil($totalActiveHours / 60) * 0.1])
	if err := s.userLogRepo.UpdateActivityHours(ctx, userID, totalMinutes); err != nil {
		return err
	}

	// Recalculate score
	return s.recalculateAndUpdateScore(ctx, userID)
}

// recalculateAndUpdateScore is a helper to recalculate and update user score
// Implements Laravel: $this->calculateScore($user)
func (s *ActivityService) recalculateAndUpdateScore(ctx context.Context, userID uint64) error {
	_, _, _, err := s.UpdateActivityScore(ctx, userID)
	return err
}
