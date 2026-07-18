// Package mocks provides test doubles for the levels service.
package mocks

import (
	"context"
	"time"

	pb "metarang/shared/pb/levels"
)

type MockCommercialClient struct {
	AddBalanceFunc func(ctx context.Context, userID uint64, asset string, amount float64) error
	CloseFunc      func() error
}

func (m *MockCommercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if m.AddBalanceFunc != nil {
		return m.AddBalanceFunc(ctx, userID, asset, amount)
	}
	return nil
}

func (m *MockCommercialClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

type MockLevelRepository struct {
	GetUserLatestLevelFunc   func(ctx context.Context, userID uint64) (*pb.Level, error)
	GetLevelsBelowScoreFunc  func(ctx context.Context, score int32) ([]*pb.Level, error)
	GetNextLevelFunc         func(ctx context.Context, currentScore int32) (*pb.Level, error)
	GetAllLevelsFunc         func(ctx context.Context) ([]*pb.Level, error)
	FindByIDFunc             func(ctx context.Context, id uint64) (*pb.Level, error)
	FindBySlugFunc           func(ctx context.Context, slug string) (*pb.Level, error)
	GetLevelGeneralInfoFunc  func(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error)
	GetLevelGemFunc          func(ctx context.Context, levelID uint64) (*pb.LevelGem, error)
	GetLevelGiftFunc         func(ctx context.Context, levelID uint64) (*pb.LevelGift, error)
	GetLevelLicensesFunc     func(ctx context.Context, levelID uint64) (*pb.LevelLicense, error)
	GetLevelPrizeFunc        func(ctx context.Context, levelID uint64) (*pb.LevelPrize, error)
	HasUserReceivedPrizeFunc func(ctx context.Context, userID, prizeID uint64) (bool, error)
	RecordReceivedPrizeFunc  func(ctx context.Context, userID, prizeID uint64) error
	GetVariableRateFunc      func(ctx context.Context, name string) (float64, error)
	GetNextLevelForScoreFunc func(ctx context.Context, userID uint64, score int32) (*pb.Level, error)
	AttachLevelToUserFunc    func(ctx context.Context, userID, levelID uint64) error
}

func (m *MockLevelRepository) GetUserLatestLevel(ctx context.Context, userID uint64) (*pb.Level, error) {
	return m.GetUserLatestLevelFunc(ctx, userID)
}
func (m *MockLevelRepository) GetLevelsBelowScore(ctx context.Context, score int32) ([]*pb.Level, error) {
	return m.GetLevelsBelowScoreFunc(ctx, score)
}
func (m *MockLevelRepository) GetNextLevel(ctx context.Context, currentScore int32) (*pb.Level, error) {
	return m.GetNextLevelFunc(ctx, currentScore)
}
func (m *MockLevelRepository) GetAllLevels(ctx context.Context) ([]*pb.Level, error) {
	return m.GetAllLevelsFunc(ctx)
}
func (m *MockLevelRepository) FindByID(ctx context.Context, id uint64) (*pb.Level, error) {
	return m.FindByIDFunc(ctx, id)
}
func (m *MockLevelRepository) FindBySlug(ctx context.Context, slug string) (*pb.Level, error) {
	return m.FindBySlugFunc(ctx, slug)
}
func (m *MockLevelRepository) GetLevelGeneralInfo(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error) {
	return m.GetLevelGeneralInfoFunc(ctx, levelID)
}
func (m *MockLevelRepository) GetLevelGem(ctx context.Context, levelID uint64) (*pb.LevelGem, error) {
	return m.GetLevelGemFunc(ctx, levelID)
}
func (m *MockLevelRepository) GetLevelGift(ctx context.Context, levelID uint64) (*pb.LevelGift, error) {
	return m.GetLevelGiftFunc(ctx, levelID)
}
func (m *MockLevelRepository) GetLevelLicenses(ctx context.Context, levelID uint64) (*pb.LevelLicense, error) {
	return m.GetLevelLicensesFunc(ctx, levelID)
}
func (m *MockLevelRepository) GetLevelPrize(ctx context.Context, levelID uint64) (*pb.LevelPrize, error) {
	return m.GetLevelPrizeFunc(ctx, levelID)
}
func (m *MockLevelRepository) HasUserReceivedPrize(ctx context.Context, userID, prizeID uint64) (bool, error) {
	return m.HasUserReceivedPrizeFunc(ctx, userID, prizeID)
}
func (m *MockLevelRepository) RecordReceivedPrize(ctx context.Context, userID, prizeID uint64) error {
	return m.RecordReceivedPrizeFunc(ctx, userID, prizeID)
}
func (m *MockLevelRepository) GetVariableRate(ctx context.Context, name string) (float64, error) {
	return m.GetVariableRateFunc(ctx, name)
}
func (m *MockLevelRepository) GetNextLevelForScore(ctx context.Context, userID uint64, score int32) (*pb.Level, error) {
	return m.GetNextLevelForScoreFunc(ctx, userID, score)
}
func (m *MockLevelRepository) AttachLevelToUser(ctx context.Context, userID, levelID uint64) error {
	return m.AttachLevelToUserFunc(ctx, userID, levelID)
}

type MockUserLogRepository struct {
	GetUserScoreFunc            func(ctx context.Context, userID uint64) (int32, error)
	GetUserLogFunc              func(ctx context.Context, userID uint64) (*pb.UserLog, error)
	CalculateScoreFunc          func(ctx context.Context, userID uint64) (int32, error)
	UpdateScoreFunc             func(ctx context.Context, userID uint64, score int32) error
	UpdateTransactionsCountFunc func(ctx context.Context, userID uint64, count string) error
	IncrementDepositFunc        func(ctx context.Context, userID uint64, amount string) error
	GetTotalFollowersFunc       func(ctx context.Context, userID uint64) (int32, error)
	UpdateFollowersCountFunc    func(ctx context.Context, userID uint64, totalFollowers int32) error
	UpdateActivityHoursFunc     func(ctx context.Context, userID uint64, totalMinutes int32) error
}

func (m *MockUserLogRepository) GetUserScore(ctx context.Context, userID uint64) (int32, error) {
	return m.GetUserScoreFunc(ctx, userID)
}
func (m *MockUserLogRepository) GetUserLog(ctx context.Context, userID uint64) (*pb.UserLog, error) {
	return m.GetUserLogFunc(ctx, userID)
}
func (m *MockUserLogRepository) CalculateScore(ctx context.Context, userID uint64) (int32, error) {
	return m.CalculateScoreFunc(ctx, userID)
}
func (m *MockUserLogRepository) UpdateScore(ctx context.Context, userID uint64, score int32) error {
	return m.UpdateScoreFunc(ctx, userID, score)
}
func (m *MockUserLogRepository) UpdateTransactionsCount(ctx context.Context, userID uint64, count string) error {
	return m.UpdateTransactionsCountFunc(ctx, userID, count)
}
func (m *MockUserLogRepository) IncrementDeposit(ctx context.Context, userID uint64, amount string) error {
	return m.IncrementDepositFunc(ctx, userID, amount)
}
func (m *MockUserLogRepository) GetTotalFollowers(ctx context.Context, userID uint64) (int32, error) {
	return m.GetTotalFollowersFunc(ctx, userID)
}
func (m *MockUserLogRepository) UpdateFollowersCount(ctx context.Context, userID uint64, totalFollowers int32) error {
	return m.UpdateFollowersCountFunc(ctx, userID, totalFollowers)
}
func (m *MockUserLogRepository) UpdateActivityHours(ctx context.Context, userID uint64, totalMinutes int32) error {
	return m.UpdateActivityHoursFunc(ctx, userID, totalMinutes)
}

type MockActivityRepository struct {
	CreateActivityFunc           func(ctx context.Context, req *pb.LogActivityRequest) (uint64, error)
	CreateUserEventFunc          func(ctx context.Context, userID uint64, event, ip, device string, status int8) error
	FindByUserIDFunc             func(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, error)
	GetLatestActivityFunc        func(ctx context.Context, userID uint64) (*pb.UserActivity, error)
	UpdateActivityFunc           func(ctx context.Context, activityID uint64, endTime time.Time, totalMinutes int32) error
	GetTotalActivityMinutesFunc  func(ctx context.Context, userID uint64) (int32, error)
	GetVariableRateFunc          func(ctx context.Context, name string) (float64, error)
	GetSignificantTradeCountFunc func(ctx context.Context, userID uint64, minIrrAmount, minPscAmount float64) (int32, error)
}

func (m *MockActivityRepository) CreateActivity(ctx context.Context, req *pb.LogActivityRequest) (uint64, error) {
	return m.CreateActivityFunc(ctx, req)
}
func (m *MockActivityRepository) CreateUserEvent(ctx context.Context, userID uint64, event, ip, device string, status int8) error {
	return m.CreateUserEventFunc(ctx, userID, event, ip, device, status)
}
func (m *MockActivityRepository) FindByUserID(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, error) {
	return m.FindByUserIDFunc(ctx, userID, limit)
}
func (m *MockActivityRepository) GetLatestActivity(ctx context.Context, userID uint64) (*pb.UserActivity, error) {
	return m.GetLatestActivityFunc(ctx, userID)
}
func (m *MockActivityRepository) UpdateActivity(ctx context.Context, activityID uint64, endTime time.Time, totalMinutes int32) error {
	return m.UpdateActivityFunc(ctx, activityID, endTime, totalMinutes)
}
func (m *MockActivityRepository) GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error) {
	return m.GetTotalActivityMinutesFunc(ctx, userID)
}
func (m *MockActivityRepository) GetVariableRate(ctx context.Context, name string) (float64, error) {
	return m.GetVariableRateFunc(ctx, name)
}
func (m *MockActivityRepository) GetSignificantTradeCount(ctx context.Context, userID uint64, minIrrAmount, minPscAmount float64) (int32, error) {
	return m.GetSignificantTradeCountFunc(ctx, userID, minIrrAmount, minPscAmount)
}
