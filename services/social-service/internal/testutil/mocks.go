package testutil

import (
	"context"

	"metarang/social-service/internal/models"
	"metarang/social-service/internal/repository"
)

// MockChallengeRepository implements repository.ChallengeRepository for tests.
type MockChallengeRepository struct {
	GetRandomUnansweredQuestionFunc   func(ctx context.Context, userID uint64) (*models.Question, error)
	GetQuestionByIDFunc               func(ctx context.Context, questionID uint64) (*models.Question, error)
	GetAnswersByQuestionIDFunc        func(ctx context.Context, questionID uint64) ([]*models.Answer, error)
	IncrementQuestionViewsFunc        func(ctx context.Context, questionID uint64) error
	IncrementQuestionParticipantsFunc func(ctx context.Context, questionID uint64) error
	CreateUserAnswerFunc              func(ctx context.Context, userID, questionID, answerID uint64) error
	HasUserAnsweredFunc               func(ctx context.Context, userID, questionID uint64) (bool, error)
	GetUserAnswerCountFunc            func(ctx context.Context, userID uint64, isCorrect bool) (int32, error)
	GetTotalParticipantsCountFunc     func(ctx context.Context) (int32, error)
	GetSystemVariableFunc             func(ctx context.Context, slug string) (float64, error)
	GetAnswerVoteCountFunc            func(ctx context.Context, answerID uint64) (int32, error)
	GetQuestionTotalAnswersFunc       func(ctx context.Context, questionID uint64) (int32, error)
}

func (m *MockChallengeRepository) GetRandomUnansweredQuestion(ctx context.Context, userID uint64) (*models.Question, error) {
	if m.GetRandomUnansweredQuestionFunc != nil {
		return m.GetRandomUnansweredQuestionFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockChallengeRepository) GetQuestionByID(ctx context.Context, questionID uint64) (*models.Question, error) {
	if m.GetQuestionByIDFunc != nil {
		return m.GetQuestionByIDFunc(ctx, questionID)
	}
	return nil, nil
}

func (m *MockChallengeRepository) GetAnswersByQuestionID(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
	if m.GetAnswersByQuestionIDFunc != nil {
		return m.GetAnswersByQuestionIDFunc(ctx, questionID)
	}
	return nil, nil
}

func (m *MockChallengeRepository) IncrementQuestionViews(ctx context.Context, questionID uint64) error {
	if m.IncrementQuestionViewsFunc != nil {
		return m.IncrementQuestionViewsFunc(ctx, questionID)
	}
	return nil
}

func (m *MockChallengeRepository) IncrementQuestionParticipants(ctx context.Context, questionID uint64) error {
	if m.IncrementQuestionParticipantsFunc != nil {
		return m.IncrementQuestionParticipantsFunc(ctx, questionID)
	}
	return nil
}

func (m *MockChallengeRepository) CreateUserAnswer(ctx context.Context, userID, questionID, answerID uint64) error {
	if m.CreateUserAnswerFunc != nil {
		return m.CreateUserAnswerFunc(ctx, userID, questionID, answerID)
	}
	return nil
}

func (m *MockChallengeRepository) HasUserAnswered(ctx context.Context, userID, questionID uint64) (bool, error) {
	if m.HasUserAnsweredFunc != nil {
		return m.HasUserAnsweredFunc(ctx, userID, questionID)
	}
	return false, nil
}

func (m *MockChallengeRepository) GetUserAnswerCount(ctx context.Context, userID uint64, isCorrect bool) (int32, error) {
	if m.GetUserAnswerCountFunc != nil {
		return m.GetUserAnswerCountFunc(ctx, userID, isCorrect)
	}
	return 0, nil
}

func (m *MockChallengeRepository) GetTotalParticipantsCount(ctx context.Context) (int32, error) {
	if m.GetTotalParticipantsCountFunc != nil {
		return m.GetTotalParticipantsCountFunc(ctx)
	}
	return 0, nil
}

func (m *MockChallengeRepository) GetSystemVariable(ctx context.Context, slug string) (float64, error) {
	if m.GetSystemVariableFunc != nil {
		return m.GetSystemVariableFunc(ctx, slug)
	}
	return 15, nil
}

func (m *MockChallengeRepository) GetAnswerVoteCount(ctx context.Context, answerID uint64) (int32, error) {
	if m.GetAnswerVoteCountFunc != nil {
		return m.GetAnswerVoteCountFunc(ctx, answerID)
	}
	return 0, nil
}

func (m *MockChallengeRepository) GetQuestionTotalAnswers(ctx context.Context, questionID uint64) (int32, error) {
	if m.GetQuestionTotalAnswersFunc != nil {
		return m.GetQuestionTotalAnswersFunc(ctx, questionID)
	}
	return 0, nil
}

// MockFollowRepository implements repository.FollowRepository for tests.
type MockFollowRepository struct {
	CreateFunc       func(ctx context.Context, followerID, followingID uint64) error
	DeleteFunc       func(ctx context.Context, followerID, followingID uint64) error
	ExistsFunc       func(ctx context.Context, followerID, followingID uint64) (bool, error)
	GetFollowersFunc func(ctx context.Context, userID uint64) ([]uint64, error)
	GetFollowingFunc func(ctx context.Context, userID uint64) ([]uint64, error)
}

func (m *MockFollowRepository) Create(ctx context.Context, followerID, followingID uint64) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, followerID, followingID)
	}
	return nil
}

func (m *MockFollowRepository) Delete(ctx context.Context, followerID, followingID uint64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, followerID, followingID)
	}
	return nil
}

func (m *MockFollowRepository) Exists(ctx context.Context, followerID, followingID uint64) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, followerID, followingID)
	}
	return false, nil
}

func (m *MockFollowRepository) GetFollowers(ctx context.Context, userID uint64) ([]uint64, error) {
	if m.GetFollowersFunc != nil {
		return m.GetFollowersFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockFollowRepository) GetFollowing(ctx context.Context, userID uint64) ([]uint64, error) {
	if m.GetFollowingFunc != nil {
		return m.GetFollowingFunc(ctx, userID)
	}
	return nil, nil
}

// MockUserRepository implements repository.UserRepository for tests.
type MockUserRepository struct {
	GetUserBasicInfoFunc func(ctx context.Context, userID uint64) (*repository.UserBasicInfo, error)
	GetUserLevelFunc     func(ctx context.Context, userID uint64) (string, error)
	GetProfilePhotosFunc func(ctx context.Context, userID uint64) ([]string, error)
	IsUserOnlineFunc     func(ctx context.Context, userID uint64) (bool, error)
}

func (m *MockUserRepository) GetUserBasicInfo(ctx context.Context, userID uint64) (*repository.UserBasicInfo, error) {
	if m.GetUserBasicInfoFunc != nil {
		return m.GetUserBasicInfoFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockUserRepository) GetUserLevel(ctx context.Context, userID uint64) (string, error) {
	if m.GetUserLevelFunc != nil {
		return m.GetUserLevelFunc(ctx, userID)
	}
	return "", nil
}

func (m *MockUserRepository) GetProfilePhotos(ctx context.Context, userID uint64) ([]string, error) {
	if m.GetProfilePhotosFunc != nil {
		return m.GetProfilePhotosFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockUserRepository) IsUserOnline(ctx context.Context, userID uint64) (bool, error) {
	if m.IsUserOnlineFunc != nil {
		return m.IsUserOnlineFunc(ctx, userID)
	}
	return false, nil
}

// MockCommercialClient implements client.CommercialClient for tests.
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

// MockAuthClient implements client.AuthClient for tests.
type MockAuthClient struct {
	CanFollowFunc func(ctx context.Context, callerUserID, targetUserID uint64) (bool, error)
	CloseFunc     func() error
}

func (m *MockAuthClient) CanFollow(ctx context.Context, callerUserID, targetUserID uint64) (bool, error) {
	if m.CanFollowFunc != nil {
		return m.CanFollowFunc(ctx, callerUserID, targetUserID)
	}
	return true, nil
}

func (m *MockAuthClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockLevelsClient implements client.LevelsClient for tests.
type MockLevelsClient struct {
	RecordFollowerFunc func(ctx context.Context, userID uint64) error
	CloseFunc          func() error
}

func (m *MockLevelsClient) RecordFollower(ctx context.Context, userID uint64) error {
	if m.RecordFollowerFunc != nil {
		return m.RecordFollowerFunc(ctx, userID)
	}
	return nil
}

func (m *MockLevelsClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
