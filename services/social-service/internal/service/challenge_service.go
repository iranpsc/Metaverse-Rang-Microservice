// Package service implements social business logic.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"metarang/shared/pkg/helpers"
	"metarang/social-service/internal/client"
	"metarang/social-service/internal/lang"
	"metarang/social-service/internal/models"
	"metarang/social-service/internal/repository"
)

var (
	ErrQuestionNotFound      = errors.New("question not found")
	ErrAnswerNotFound        = errors.New("answer not found")
	ErrAnswerMismatch        = errors.New("answer does not belong to the given question")
	ErrAlreadyAnswered       = errors.New("user has already answered this question")
	ErrNoUnansweredQuestions = errors.New("no unanswered questions available")
)

type ChallengeService interface {
	GetTimings(ctx context.Context, userID uint64) (*models.TimingsData, error)
	GetQuestion(ctx context.Context, userID uint64) (*models.QuestionResource, error)
	SubmitAnswer(ctx context.Context, userID, questionID, answerID uint64) (*models.QuestionResource, error)
	GetAdvertisement(ctx context.Context) ([]models.Advertisement, error)
}

type ChallengeConfig struct {
	Locale     string
	ProjectURL string
}

type challengeService struct {
	challengeRepo    repository.ChallengeRepository
	commercialClient client.CommercialClient
	locale           string
	projectURL       string
}

func NewChallengeService(challengeRepo repository.ChallengeRepository, commercialClient client.CommercialClient, configs ...ChallengeConfig) ChallengeService {
	config := ChallengeConfig{Locale: "en"}
	if len(configs) > 0 {
		config = configs[0]
	}

	return &challengeService{
		challengeRepo:    challengeRepo,
		commercialClient: commercialClient,
		locale:           lang.NormalizeLocale(config.Locale),
		projectURL:       strings.TrimSuffix(strings.TrimSpace(config.ProjectURL), "/"),
	}
}

type advertisementSeed struct {
	code            string
	titleKey        string
	descriptionKey  string
	investmentValue string
	endsAtGregorian time.Time
}

var challengeAdvertisements = []advertisementSeed{
	{"bn-1000", "Matrix exit box", "Banking services in Metaverse", "1000000", time.Date(2028, 11, 5, 0, 0, 0, 0, time.UTC)},
	{"bn-1001", "Quantum trade hub", "Next-gen digital trading desk", "2500000", time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)},
	{"bn-1002", "Neon vault reserve", "Secure multi-asset custody", "1750000", time.Date(2029, 1, 20, 0, 0, 0, 0, time.UTC)},
	{"bn-1003", "Oracle signal funds", "AI-driven market intelligence", "3200000", time.Date(2028, 3, 8, 0, 0, 0, 0, time.UTC)},
	{"bn-1004", "Pulse liquidity pool", "Cross-chain liquidity provision", "900000", time.Date(2027, 12, 1, 0, 0, 0, 0, time.UTC)},
	{"bn-1005", "Horizon credit lane", "Metaverse-native lending rails", "4100000", time.Date(2030, 4, 12, 0, 0, 0, 0, time.UTC)},
	{"bn-1006", "Eclipse yield studio", "Structured yield products", "1500000", time.Date(2029, 9, 30, 0, 0, 0, 0, time.UTC)},
}

func (s *challengeService) advertisementAssetURL(code, extension string) string {
	path := fmt.Sprintf("/uploads/challenge/advertisement/%s/%s.%s", code, code, extension)
	if s.projectURL == "" {
		return path
	}
	return s.projectURL + path
}

func (s *challengeService) GetAdvertisement(ctx context.Context) ([]models.Advertisement, error) {
	_ = ctx
	advertisements := make([]models.Advertisement, 0, len(challengeAdvertisements))
	for _, seed := range challengeAdvertisements {
		endsAt := seed.endsAtGregorian.Format("2006/01/02")
		if s.locale == "fa" {
			endsAt = helpers.FormatJalaliDate(seed.endsAtGregorian)
		}

		advertisements = append(advertisements, models.Advertisement{
			Code:            seed.code,
			Title:           lang.T(s.locale, seed.titleKey),
			Description:     lang.T(s.locale, seed.descriptionKey),
			InvestmentValue: seed.investmentValue,
			EndsAt:          endsAt,
			VideoURL:        s.advertisementAssetURL(seed.code, "mp4"),
			ImageURL:        s.advertisementAssetURL(seed.code, "jpg"),
			URL:             "https://metarang.com/fa/citizens/" + seed.code,
			InvestmentAsset: "red",
		})
	}
	return advertisements, nil
}

func (s *challengeService) GetTimings(ctx context.Context, userID uint64) (*models.TimingsData, error) {
	// Get system variables for intervals
	displayAdInterval, err := s.challengeRepo.GetSystemVariable(ctx, "challenge_display_ad_interval")
	if err != nil {
		displayAdInterval = 15.0 // Default fallback
	}

	displayQuestionInterval, err := s.challengeRepo.GetSystemVariable(ctx, "challenge_display_question_interval")
	if err != nil {
		displayQuestionInterval = 15.0 // Default fallback
	}

	displayAnswerInterval, err := s.challengeRepo.GetSystemVariable(ctx, "challenge_display_answer_interval")
	if err != nil {
		displayAnswerInterval = 15.0 // Default fallback
	}

	// Get total participants count
	participants, err := s.challengeRepo.GetTotalParticipantsCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants count: %w", err)
	}

	// Get user's correct and wrong answers
	correctAnswers, err := s.challengeRepo.GetUserAnswerCount(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get correct answers count: %w", err)
	}

	wrongAnswers, err := s.challengeRepo.GetUserAnswerCount(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get wrong answers count: %w", err)
	}

	return &models.TimingsData{
		DisplayAdInterval:       int32(displayAdInterval),
		DisplayQuestionInterval: int32(displayQuestionInterval),
		DisplayAnswerInterval:   int32(displayAnswerInterval),
		Participants:            participants,
		CorrectAnswers:          correctAnswers,
		WrongAnswers:            wrongAnswers,
	}, nil
}

func (s *challengeService) GetQuestion(ctx context.Context, userID uint64) (*models.QuestionResource, error) {
	// Get a random unanswered question
	question, err := s.challengeRepo.GetRandomUnansweredQuestion(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}
	if question == nil {
		return nil, ErrNoUnansweredQuestions
	}

	// Increment views
	if err := s.challengeRepo.IncrementQuestionViews(ctx, question.ID); err != nil {
		// Log error but continue
		fmt.Printf("failed to increment views: %v\n", err)
	}

	// Get answers
	answers, err := s.challengeRepo.GetAnswersByQuestionID(ctx, question.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}

	// Convert answers to resource (without is_correct and vote_percentage)
	answerResources := make([]models.AnswerResource, 0, len(answers))
	for _, answer := range answers {
		answerResources = append(answerResources, models.AnswerResource{
			ID:    answer.ID,
			Title: answer.Title,
			Image: answer.Image,
		})
	}

	return &models.QuestionResource{
		ID:           question.ID,
		Title:        question.Title,
		Image:        question.Image,
		Prize:        question.Prize,
		Participants: question.Participants,
		Views:        question.Views,
		CreatorCode:  question.CreatorCode,
		Answers:      answerResources,
	}, nil
}

func (s *challengeService) SubmitAnswer(ctx context.Context, userID, questionID, answerID uint64) (*models.QuestionResource, error) {
	// Get question
	question, err := s.challengeRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	// Get all answers for this question
	answers, err := s.challengeRepo.GetAnswersByQuestionID(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}

	// Verify answer belongs to question
	answerFound := false
	var selectedAnswer *models.Answer
	for _, answer := range answers {
		if answer.ID == answerID {
			answerFound = true
			selectedAnswer = answer
			break
		}
	}
	if !answerFound {
		return nil, ErrAnswerMismatch
	}

	// A user gets exactly one attempt per question, regardless of correctness.
	hasAnswered, err := s.challengeRepo.HasUserAnswered(ctx, userID, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check previous answer: %w", err)
	}
	if hasAnswered {
		return nil, ErrAlreadyAnswered
	}

	// Create user answer record
	if err := s.challengeRepo.CreateUserAnswer(ctx, userID, questionID, answerID); err != nil {
		return nil, fmt.Errorf("failed to create user answer: %w", err)
	}

	// Increment participants for the user's single accepted attempt.
	if err := s.challengeRepo.IncrementQuestionParticipants(ctx, questionID); err != nil {
		// Log error but continue
		fmt.Printf("failed to increment participants: %v\n", err)
	}

	// If answer is correct, credit PSC to user's wallet
	if selectedAnswer.IsCorrect {
		if s.commercialClient != nil {
			prizeAmount := float64(question.Prize)
			if err := s.commercialClient.AddBalance(ctx, userID, "psc", prizeAmount); err != nil {
				// Log error but don't fail the answer submission
				fmt.Printf("failed to credit prize to wallet: %v\n", err)
			}
		}
	}

	// Get updated question with new participants count
	updatedQuestion, err := s.challengeRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		// Use original question if update fails
		updatedQuestion = question
	}

	// Calculate vote percentages for each answer
	totalAnswers, err := s.challengeRepo.GetQuestionTotalAnswers(ctx, questionID)
	if err != nil {
		totalAnswers = 1 // Avoid division by zero
	}

	// Build answer resources with is_correct and vote_percentage
	answerResources := make([]models.AnswerResource, 0, len(answers))
	for _, answer := range answers {
		voteCount, err := s.challengeRepo.GetAnswerVoteCount(ctx, answer.ID)
		if err != nil {
			voteCount = 0
		}

		// Calculate vote percentage (rounded down)
		var votePercentage int32
		if totalAnswers > 0 {
			votePercentage = int32(math.Floor(float64(voteCount) / float64(totalAnswers) * 100))
		}

		answerResources = append(answerResources, models.AnswerResource{
			ID:             answer.ID,
			Title:          answer.Title,
			Image:          answer.Image,
			IsCorrect:      answer.IsCorrect,
			VotePercentage: votePercentage,
		})
	}

	return &models.QuestionResource{
		ID:           updatedQuestion.ID,
		Title:        updatedQuestion.Title,
		Image:        updatedQuestion.Image,
		Prize:        updatedQuestion.Prize,
		Participants: updatedQuestion.Participants,
		Views:        updatedQuestion.Views,
		CreatorCode:  updatedQuestion.CreatorCode,
		Answers:      answerResources,
	}, nil
}
