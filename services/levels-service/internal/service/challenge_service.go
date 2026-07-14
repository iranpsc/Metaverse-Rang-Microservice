package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"metarang/levels-service/internal/client"
	"metarang/levels-service/internal/lang"
	"metarang/levels-service/internal/models"
	pb "metarang/shared/pb/levels"
	"metarang/shared/pkg/helpers"
)

type challengeRepository interface {
	GetRandomUnansweredQuestion(ctx context.Context, userID uint64) (*pb.Question, error)
	IncrementViews(ctx context.Context, questionID uint64) error
	ValidateAnswer(ctx context.Context, questionID, answerID uint64) (bool, error)
	HasUserAnsweredQuestion(ctx context.Context, userID, questionID uint64) (bool, error)
	RecordUserAnswer(ctx context.Context, userID, questionID, answerID uint64) error
	IncrementParticipants(ctx context.Context, questionID uint64) error
	CheckAnswer(ctx context.Context, answerID, questionID uint64) (bool, string, error)
	GetQuestionByID(ctx context.Context, questionID uint64) (*pb.Question, error)
	GetChallengeIntervals(ctx context.Context) (int32, int32, int32, error)
	GetUserAnswerCounts(ctx context.Context, userID uint64) (int32, int32, error)
	GetTotalParticipants(ctx context.Context) (int32, error)
	GetVariableRate(ctx context.Context, name string) (float64, error)
}

type advertisementSeed struct {
	code            string
	titleKey        string
	descriptionKey  string
	investmentValue string
	endsAtGregorian time.Time
}

var challengeAdvertisements = []advertisementSeed{
	{
		code:            "bn-1000",
		titleKey:        "Matrix exit box",
		descriptionKey:  "Banking services in Metaverse",
		investmentValue: "1000000",
		endsAtGregorian: time.Date(2028, 11, 5, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1001",
		titleKey:        "Quantum trade hub",
		descriptionKey:  "Next-gen digital trading desk",
		investmentValue: "2500000",
		endsAtGregorian: time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1002",
		titleKey:        "Neon vault reserve",
		descriptionKey:  "Secure multi-asset custody",
		investmentValue: "1750000",
		endsAtGregorian: time.Date(2029, 1, 20, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1003",
		titleKey:        "Oracle signal funds",
		descriptionKey:  "AI-driven market intelligence",
		investmentValue: "3200000",
		endsAtGregorian: time.Date(2028, 3, 8, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1004",
		titleKey:        "Pulse liquidity pool",
		descriptionKey:  "Cross-chain liquidity provision",
		investmentValue: "900000",
		endsAtGregorian: time.Date(2027, 12, 1, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1005",
		titleKey:        "Horizon credit lane",
		descriptionKey:  "Metaverse-native lending rails",
		investmentValue: "4100000",
		endsAtGregorian: time.Date(2030, 4, 12, 0, 0, 0, 0, time.UTC),
	},
	{
		code:            "bn-1006",
		titleKey:        "Eclipse yield studio",
		descriptionKey:  "Structured yield products",
		investmentValue: "1500000",
		endsAtGregorian: time.Date(2029, 9, 30, 0, 0, 0, 0, time.UTC),
	},
}

type ChallengeService struct {
	challengeRepo    challengeRepository
	commercialClient client.CommercialClient
	defaultPSCRate   float64
	appLocale        string
	projectURL       string
}

func NewChallengeService(challengeRepo challengeRepository, commercialClient client.CommercialClient, appLocale, projectURL string) *ChallengeService {
	return &ChallengeService{
		challengeRepo:    challengeRepo,
		commercialClient: commercialClient,
		defaultPSCRate:   30000,
		appLocale:        lang.NormalizeLocale(appLocale),
		projectURL:       strings.TrimSuffix(strings.TrimSpace(projectURL), "/"),
	}
}

func (s *ChallengeService) advertisementAssetURL(code, ext string) string {
	path := fmt.Sprintf("/uploads/challenge/advertisement/%s/%s.%s", code, code, ext)
	if s.projectURL == "" {
		return path
	}
	return s.projectURL + path
}

// GetAdvertisement returns the static challenge advertisers list.
// Titles and descriptions are translated using APP_LOCALE; ends_at becomes Jalali when locale is FA.
func (s *ChallengeService) GetAdvertisement(ctx context.Context) ([]models.Advertisement, error) {
	_ = ctx
	locale := s.appLocale
	ads := make([]models.Advertisement, 0, len(challengeAdvertisements))
	for _, seed := range challengeAdvertisements {
		endsAt := seed.endsAtGregorian.Format("2006/01/02")
		if locale == "fa" {
			endsAt = helpers.FormatJalaliDate(seed.endsAtGregorian)
		}
		ads = append(ads, models.Advertisement{
			Code:            seed.code,
			Title:           lang.T(locale, seed.titleKey),
			Description:     lang.T(locale, seed.descriptionKey),
			InvestmentValue: seed.investmentValue,
			EndsAt:          endsAt,
			VideoURL:        s.advertisementAssetURL(seed.code, "mp4"),
			ImageURL:        s.advertisementAssetURL(seed.code, "jpg"),
		})
	}
	return ads, nil
}

// GetQuestion retrieves a random unanswered question for the user
// Implements Laravel: ChallengeController@getQuestion
func (s *ChallengeService) GetQuestion(ctx context.Context, userID uint64) (*pb.Question, bool, error) {
	// Get random unanswered question
	// Laravel: while loop in selectQuestion method
	question, err := s.challengeRepo.GetRandomUnansweredQuestion(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	if question == nil {
		return nil, false, nil
	}

	// Increment views
	// Laravel: $question->increment('views')
	if err := s.challengeRepo.IncrementViews(ctx, question.Id); err != nil {
		return question, true, err
	}

	return question, true, nil
}

// SubmitAnswer submits an answer and returns result
// Implements Laravel: ChallengeController@answerResult
func (s *ChallengeService) SubmitAnswer(ctx context.Context, userID, questionID, answerID uint64) (bool, string, *pb.Question, error) {
	// Validate answer belongs to question
	// Laravel: if ($answer->question->isNot($question))
	isValid, err := s.challengeRepo.ValidateAnswer(ctx, questionID, answerID)
	if err != nil || !isValid {
		return false, "", nil, fmt.Errorf("answer is not valid")
	}

	// Check if user has already answered this question (authorization)
	// Laravel: $this->authorize('answer', $question)
	hasAnswered, err := s.challengeRepo.HasUserAnsweredQuestion(ctx, userID, questionID)
	if err != nil {
		return false, "", nil, err
	}
	if hasAnswered {
		return false, "", nil, fmt.Errorf("question already answered")
	}

	// Record user's answer
	// Laravel: UserQuestionAnswer::create([...])
	if err := s.challengeRepo.RecordUserAnswer(ctx, userID, questionID, answerID); err != nil {
		return false, "", nil, err
	}

	// Increment participants count
	// Laravel: $question->increment('participants')
	if err := s.challengeRepo.IncrementParticipants(ctx, questionID); err != nil {
		return false, "", nil, err
	}

	// Check if answer is correct
	// Laravel: if ($answer->isCorrect())
	isCorrect, prize, err := s.challengeRepo.CheckAnswer(ctx, answerID, questionID)
	if err != nil {
		return false, "", nil, err
	}

	prizeAwarded := "0"
	if isCorrect {
		prizeAmount, parseErr := parseNumericString(prize)
		if parseErr != nil {
			return false, "", nil, parseErr
		}
		pscRate, err := s.challengeRepo.GetVariableRate(ctx, "psc")
		if err != nil || pscRate <= 0 {
			pscRate = s.defaultPSCRate
		}
		if err := s.commercialClient.AddBalance(ctx, userID, "psc", prizeAmount/pscRate); err != nil {
			return false, "", nil, fmt.Errorf("failed to award challenge prize: %w", err)
		}
		prizeAwarded = prize
	}

	// Get question with answers to return
	question, err := s.challengeRepo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return isCorrect, prizeAwarded, nil, err
	}

	return isCorrect, prizeAwarded, question, nil
}

// GetTimings retrieves challenge configuration and user stats
// Implements Laravel: ChallengeController@getTimings
func (s *ChallengeService) GetTimings(ctx context.Context, userID uint64) (*pb.TimingsResponse, error) {
	// Get system variables for intervals
	// Laravel: SystemVariable::getByKey('challenge_display_ad_interval') ?? 15
	adInterval, questionInterval, answerInterval, err := s.challengeRepo.GetChallengeIntervals(ctx)
	if err != nil {
		// Use defaults on error
		adInterval, questionInterval, answerInterval = 15, 15, 15
	}

	// Get user's correct and wrong answers
	// Laravel: $this->getCorrectAnswers() and $this->getWrongAnswers()
	correct, wrong, err := s.challengeRepo.GetUserAnswerCounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get total participants
	// Laravel: UserQuestionAnswer::distinct()->count('user_id')
	participants, err := s.challengeRepo.GetTotalParticipants(ctx)
	if err != nil {
		participants = 0
	}

	return &pb.TimingsResponse{
		DisplayAdInterval:       adInterval,
		DisplayQuestionInterval: questionInterval,
		DisplayAnswerInterval:   answerInterval,
		Participants:            participants,
		CorrectAnswers:          correct,
		WrongAnswers:            wrong,
	}, nil
}
