package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"metarang/social-service/internal/models"
	"metarang/social-service/internal/service"
	"metarang/social-service/internal/testutil"
)

func TestChallengeService_GetTimings(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetSystemVariableFunc = func(ctx context.Context, slug string) (float64, error) {
		switch slug {
		case "challenge_display_ad_interval":
			return 10, nil
		default:
			return 15, nil
		}
	}
	repo.GetTotalParticipantsCountFunc = func(ctx context.Context) (int32, error) {
		return 100, nil
	}
	repo.GetUserAnswerCountFunc = func(ctx context.Context, userID uint64, isCorrect bool) (int32, error) {
		if isCorrect {
			return 3, nil
		}
		return 7, nil
	}

	svc := service.NewChallengeService(repo, nil)
	out, err := svc.GetTimings(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, int32(10), out.DisplayAdInterval)
	require.Equal(t, int32(15), out.DisplayQuestionInterval)
	require.Equal(t, int32(100), out.Participants)
	require.Equal(t, int32(3), out.CorrectAnswers)
	require.Equal(t, int32(7), out.WrongAnswers)
}

func TestChallengeService_GetQuestion_NoQuestions(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetRandomUnansweredQuestionFunc = func(ctx context.Context, userID uint64) (*models.Question, error) {
		return nil, nil
	}
	svc := service.NewChallengeService(repo, nil)
	_, err := svc.GetQuestion(context.Background(), 1)
	require.ErrorIs(t, err, service.ErrNoUnansweredQuestions)
}

func TestChallengeService_GetQuestion_OK(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetRandomUnansweredQuestionFunc = func(ctx context.Context, userID uint64) (*models.Question, error) {
		return &models.Question{
			ID: 1, Title: "Q", Image: "img.png", CreatorCode: "C", Prize: 10,
			Participants: 2, Views: 5,
		}, nil
	}
	repo.GetAnswersByQuestionIDFunc = func(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
		return []*models.Answer{
			{ID: 10, Title: "A", Image: "a.png", IsCorrect: true},
		}, nil
	}
	svc := service.NewChallengeService(repo, nil)
	q, err := svc.GetQuestion(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), q.ID)
	require.Len(t, q.Answers, 1)
	require.False(t, q.Answers[0].IsCorrect) // stripped for GET question
}

func TestChallengeService_SubmitAnswer_Mismatch(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetQuestionByIDFunc = func(ctx context.Context, questionID uint64) (*models.Question, error) {
		return &models.Question{ID: questionID}, nil
	}
	repo.GetAnswersByQuestionIDFunc = func(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
		return []*models.Answer{{ID: 1, QuestionID: questionID}}, nil
	}
	svc := service.NewChallengeService(repo, nil)
	_, err := svc.SubmitAnswer(context.Background(), 1, 1, 999)
	require.ErrorIs(t, err, service.ErrAnswerMismatch)
}

func TestChallengeService_SubmitAnswer_AlreadyAnswered(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetQuestionByIDFunc = func(ctx context.Context, questionID uint64) (*models.Question, error) {
		return &models.Question{ID: questionID}, nil
	}
	repo.GetAnswersByQuestionIDFunc = func(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
		return []*models.Answer{{ID: 1, QuestionID: questionID, IsCorrect: false}}, nil
	}
	repo.HasUserAnsweredFunc = func(ctx context.Context, userID, questionID uint64) (bool, error) {
		return true, nil
	}
	svc := service.NewChallengeService(repo, nil)
	_, err := svc.SubmitAnswer(context.Background(), 1, 1, 1)
	require.ErrorIs(t, err, service.ErrAlreadyAnswered)
}

func TestChallengeService_SubmitAnswer_CreditsPSC(t *testing.T) {
	var credited float64
	com := &testutil.MockCommercialClient{}
	com.AddBalanceFunc = func(ctx context.Context, userID uint64, asset string, amount float64) error {
		credited = amount
		require.Equal(t, "psc", asset)
		require.Equal(t, uint64(42), userID)
		return nil
	}

	repo := &testutil.MockChallengeRepository{}
	repo.GetQuestionByIDFunc = func(ctx context.Context, questionID uint64) (*models.Question, error) {
		return &models.Question{ID: questionID, Prize: 25}, nil
	}
	repo.GetAnswersByQuestionIDFunc = func(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
		return []*models.Answer{
			{ID: 1, QuestionID: questionID, IsCorrect: true},
			{ID: 2, QuestionID: questionID, IsCorrect: false},
		}, nil
	}
	repo.HasUserAnsweredFunc = func(ctx context.Context, userID, questionID uint64) (bool, error) {
		return false, nil
	}
	repo.CreateUserAnswerFunc = func(ctx context.Context, userID, questionID, answerID uint64) error {
		return nil
	}
	repo.GetQuestionTotalAnswersFunc = func(ctx context.Context, questionID uint64) (int32, error) {
		return 4, nil
	}
	repo.GetAnswerVoteCountFunc = func(ctx context.Context, answerID uint64) (int32, error) {
		if answerID == 1 {
			return 3, nil
		}
		return 1, nil
	}

	svc := service.NewChallengeService(repo, com)
	_, err := svc.SubmitAnswer(context.Background(), 42, 9, 1)
	require.NoError(t, err)
	require.Equal(t, 25.0, credited)
}

func TestChallengeService_SubmitAnswer_QuestionNotFound(t *testing.T) {
	repo := &testutil.MockChallengeRepository{}
	repo.GetQuestionByIDFunc = func(ctx context.Context, questionID uint64) (*models.Question, error) {
		return nil, nil
	}
	svc := service.NewChallengeService(repo, nil)
	_, err := svc.SubmitAnswer(context.Background(), 1, 99, 1)
	require.ErrorIs(t, err, service.ErrQuestionNotFound)
}

func TestChallengeService_SubmitAnswer_WrongNoCredit(t *testing.T) {
	var credited bool
	com := &testutil.MockCommercialClient{}
	com.AddBalanceFunc = func(ctx context.Context, userID uint64, asset string, amount float64) error {
		credited = true
		return nil
	}

	repo := &testutil.MockChallengeRepository{}
	repo.GetQuestionByIDFunc = func(ctx context.Context, questionID uint64) (*models.Question, error) {
		return &models.Question{ID: questionID, Prize: 25}, nil
	}
	repo.GetAnswersByQuestionIDFunc = func(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
		return []*models.Answer{
			{ID: 1, QuestionID: questionID, IsCorrect: false},
		}, nil
	}
	repo.HasUserAnsweredFunc = func(ctx context.Context, userID, questionID uint64) (bool, error) {
		return false, nil
	}
	repo.CreateUserAnswerFunc = func(ctx context.Context, userID, questionID, answerID uint64) error {
		return nil
	}
	repo.GetQuestionTotalAnswersFunc = func(ctx context.Context, questionID uint64) (int32, error) {
		return 2, nil
	}
	repo.GetAnswerVoteCountFunc = func(ctx context.Context, answerID uint64) (int32, error) {
		return 1, nil
	}

	svc := service.NewChallengeService(repo, com)
	_, err := svc.SubmitAnswer(context.Background(), 42, 9, 1)
	require.NoError(t, err)
	require.False(t, credited)
}

func TestChallengeService_GetAdvertisement_EN(t *testing.T) {
	svc := service.NewChallengeService(
		&testutil.MockChallengeRepository{},
		nil,
		service.ChallengeConfig{
			Locale:     "EN",
			ProjectURL: "http://localhost:8000",
		},
	)

	ads, err := svc.GetAdvertisement(context.Background())
	require.NoError(t, err)
	require.Len(t, ads, 7)

	first := ads[0]
	require.Equal(t, "bn-1000", first.Code)
	require.Equal(t, "Matrix exit box", first.Title)
	require.Equal(t, "Banking services in Metaverse", first.Description)
	require.Equal(t, "1000000", first.InvestmentValue)
	require.Equal(t, "2028/11/05", first.EndsAt)
	require.Equal(t, "http://localhost:8000/uploads/challenge/advertisement/bn-1000/bn-1000.mp4", first.VideoURL)
	require.Equal(t, "http://localhost:8000/uploads/challenge/advertisement/bn-1000/bn-1000.jpg", first.ImageURL)
	require.Equal(t, "https://metarang.com/fa/citizens/bn-1000", first.URL)
	require.Equal(t, "red", first.InvestmentAsset)

	for _, ad := range ads {
		require.Equal(t, "https://metarang.com/fa/citizens/"+ad.Code, ad.URL)
		require.Equal(t, "red", ad.InvestmentAsset)
	}
}

func TestChallengeService_GetAdvertisement_FA(t *testing.T) {
	svc := service.NewChallengeService(
		&testutil.MockChallengeRepository{},
		nil,
		service.ChallengeConfig{
			Locale:     "FA",
			ProjectURL: "http://localhost:8000",
		},
	)

	ads, err := svc.GetAdvertisement(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, ads)
	require.Equal(t, "bn-1000", ads[0].Code)
	require.Equal(t, "صندوق خروج از ماتریکس", ads[0].Title)
	require.Equal(t, "ارائه خدمات بانکی نوین در دنیای متاورس", ads[0].Description)
	require.Equal(t, "https://metarang.com/fa/citizens/bn-1000", ads[0].URL)
	require.Equal(t, "red", ads[0].InvestmentAsset)
}
