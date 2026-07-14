package levels_service_test

import (
	"context"
	"testing"
	"time"

	"metarang/levels-service/internal/mocks"
	"metarang/levels-service/internal/service"
	pb "metarang/shared/pb/levels"
	"metarang/shared/pkg/helpers"
)

func TestChallengeServiceGetQuestion(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			GetRandomUnansweredQuestionFunc: func(ctx context.Context, userID uint64) (*pb.Question, error) {
				return &pb.Question{Id: 11}, nil
			},
			IncrementViewsFunc: func(ctx context.Context, questionID uint64) error { return nil },
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	question, found, err := svc.GetQuestion(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !found || question == nil || question.Id != 11 {
		t.Fatalf("expected question id 11")
	}
}

func TestChallengeServiceSubmitAnswerCorrect(t *testing.T) {
	addCalls := 0

	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			ValidateAnswerFunc:          func(ctx context.Context, questionID, answerID uint64) (bool, error) { return true, nil },
			HasUserAnsweredQuestionFunc: func(ctx context.Context, userID, questionID uint64) (bool, error) { return false, nil },
			RecordUserAnswerFunc:        func(ctx context.Context, userID, questionID, answerID uint64) error { return nil },
			IncrementParticipantsFunc:   func(ctx context.Context, questionID uint64) error { return nil },
			CheckAnswerFunc:             func(ctx context.Context, answerID, questionID uint64) (bool, string, error) { return true, "1200", nil },
			GetQuestionByIDFunc: func(ctx context.Context, questionID uint64) (*pb.Question, error) {
				return &pb.Question{Id: questionID}, nil
			},
			GetVariableRateFunc: func(ctx context.Context, name string) (float64, error) { return 100, nil },
		},
		&mocks.MockCommercialClient{
			AddBalanceFunc: func(ctx context.Context, userID uint64, asset string, amount float64) error {
				addCalls++
				return nil
			},
		},
		"EN",
		"http://localhost:8000",
	)

	correct, prize, question, err := svc.SubmitAnswer(context.Background(), 1, 2, 3)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !correct || prize != "1200" || question == nil {
		t.Fatalf("expected correct answer with prize")
	}
	if addCalls != 1 {
		t.Fatalf("expected one wallet update, got %d", addCalls)
	}
}

func TestChallengeServiceSubmitAnswerAlreadyAnswered(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			ValidateAnswerFunc:          func(ctx context.Context, questionID, answerID uint64) (bool, error) { return true, nil },
			HasUserAnsweredQuestionFunc: func(ctx context.Context, userID, questionID uint64) (bool, error) { return true, nil },
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	if _, _, _, err := svc.SubmitAnswer(context.Background(), 1, 2, 3); err == nil {
		t.Fatalf("expected already answered error")
	}
}

func TestChallengeServiceGetTimings(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			GetChallengeIntervalsFunc: func(ctx context.Context) (int32, int32, int32, error) { return 10, 20, 30, nil },
			GetUserAnswerCountsFunc:   func(ctx context.Context, userID uint64) (int32, int32, error) { return 2, 1, nil },
			GetTotalParticipantsFunc:  func(ctx context.Context) (int32, error) { return 9, nil },
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	resp, err := svc.GetTimings(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.DisplayAdInterval != 10 || resp.CorrectAnswers != 2 || resp.Participants != 9 {
		t.Fatalf("unexpected timings response: %+v", resp)
	}
}

func TestChallengeServiceGetQuestionNotFound(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			GetRandomUnansweredQuestionFunc: func(ctx context.Context, userID uint64) (*pb.Question, error) {
				return nil, nil
			},
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	q, found, err := svc.GetQuestion(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if found || q != nil {
		t.Fatalf("expected no question")
	}
}

func TestChallengeServiceGetQuestionRepoError(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			GetRandomUnansweredQuestionFunc: func(ctx context.Context, userID uint64) (*pb.Question, error) {
				return nil, assertErr{}
			},
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	if _, _, err := svc.GetQuestion(context.Background(), 1); err == nil {
		t.Fatalf("expected repo error")
	}
}

func TestChallengeServiceSubmitAnswerInvalidAnswer(t *testing.T) {
	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{
			ValidateAnswerFunc: func(ctx context.Context, questionID, answerID uint64) (bool, error) { return false, nil },
		},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	if _, _, _, err := svc.SubmitAnswer(context.Background(), 1, 2, 3); err == nil {
		t.Fatalf("expected invalid answer error")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "assert error" }

func TestChallengeServiceGetAdvertisementEN(t *testing.T) {
	t.Setenv("APP_LOCALE", "EN")

	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{},
		&mocks.MockCommercialClient{},
		"EN",
		"http://localhost:8000",
	)

	ads, err := svc.GetAdvertisement(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(ads) != 7 {
		t.Fatalf("expected 7 advertisers, got %d", len(ads))
	}

	first := ads[0]
	if first.Code != "bn-1000" {
		t.Fatalf("expected code bn-1000, got %q", first.Code)
	}
	if first.Title != "Matrix exit box" {
		t.Fatalf("expected EN title Matrix exit box, got %q", first.Title)
	}
	if first.Description != "Banking services in Metaverse" {
		t.Fatalf("expected EN description Banking services in Metaverse, got %q", first.Description)
	}
	if first.InvestmentValue != "1000000" {
		t.Fatalf("expected investment_value 1000000, got %q", first.InvestmentValue)
	}
	if first.EndsAt != "2028/11/05" {
		t.Fatalf("expected ends_at 2028/11/05 for EN locale, got %q", first.EndsAt)
	}
	wantVideo := "http://localhost:8000/uploads/challenge/advertisement/bn-1000/bn-1000.mp4"
	wantImage := "http://localhost:8000/uploads/challenge/advertisement/bn-1000/bn-1000.jpg"
	if first.VideoURL != wantVideo {
		t.Fatalf("expected video_url %q, got %q", wantVideo, first.VideoURL)
	}
	if first.ImageURL != wantImage {
		t.Fatalf("expected image_url %q, got %q", wantImage, first.ImageURL)
	}
	for i, ad := range ads[1:] {
		wantVideo = "http://localhost:8000/uploads/challenge/advertisement/" + ad.Code + "/" + ad.Code + ".mp4"
		wantImage = "http://localhost:8000/uploads/challenge/advertisement/" + ad.Code + "/" + ad.Code + ".jpg"
		if ad.VideoURL != wantVideo {
			t.Fatalf("advertiser[%d] expected video_url %q, got %q", i+1, wantVideo, ad.VideoURL)
		}
		if ad.ImageURL != wantImage {
			t.Fatalf("advertiser[%d] expected image_url %q, got %q", i+1, wantImage, ad.ImageURL)
		}
	}
}

func TestChallengeServiceGetAdvertisementFA(t *testing.T) {
	t.Setenv("APP_LOCALE", "FA")

	svc := service.NewChallengeService(
		&mocks.MockChallengeRepository{},
		&mocks.MockCommercialClient{},
		"FA",
		"http://localhost:8000",
	)

	ads, err := svc.GetAdvertisement(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(ads) != 7 {
		t.Fatalf("expected 7 advertisers, got %d", len(ads))
	}

	first := ads[0]
	if first.Code != "bn-1000" {
		t.Fatalf("expected code bn-1000, got %q", first.Code)
	}
	if first.Title != "صندوق خروج از ماتریکس" {
		t.Fatalf("expected FA title, got %q", first.Title)
	}
	if first.Description != "ارائه خدمات بانکی نوین در دنیای متاورس" {
		t.Fatalf("expected FA description, got %q", first.Description)
	}
	if first.InvestmentValue != "1000000" {
		t.Fatalf("expected investment_value 1000000, got %q", first.InvestmentValue)
	}

	endsAt := time.Date(2028, 11, 5, 0, 0, 0, 0, time.UTC)
	wantEndsAt := helpers.FormatJalaliDate(endsAt)
	if first.EndsAt != wantEndsAt {
		t.Fatalf("expected Jalali ends_at %q for FA locale, got %q", wantEndsAt, first.EndsAt)
	}
}
