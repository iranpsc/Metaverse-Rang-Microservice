package handler_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "metarang/shared/pb/social"
	"metarang/social-service/internal/handler"
	"metarang/social-service/internal/models"
	"metarang/social-service/internal/service"
	"metarang/social-service/internal/testutil"
)

type stubChallengeSvc struct {
	getTimings       func(context.Context, uint64) (*models.TimingsData, error)
	getQuestion      func(context.Context, uint64) (*models.QuestionResource, error)
	submitAnswer     func(context.Context, uint64, uint64, uint64) (*models.QuestionResource, error)
	getAdvertisement func(context.Context) ([]models.Advertisement, error)
}

func (s *stubChallengeSvc) GetTimings(ctx context.Context, userID uint64) (*models.TimingsData, error) {
	if s.getTimings != nil {
		return s.getTimings(ctx, userID)
	}
	return &models.TimingsData{}, nil
}

func (s *stubChallengeSvc) GetQuestion(ctx context.Context, userID uint64) (*models.QuestionResource, error) {
	if s.getQuestion != nil {
		return s.getQuestion(ctx, userID)
	}
	return nil, nil
}

func (s *stubChallengeSvc) SubmitAnswer(ctx context.Context, userID, questionID, answerID uint64) (*models.QuestionResource, error) {
	if s.submitAnswer != nil {
		return s.submitAnswer(ctx, userID, questionID, answerID)
	}
	return nil, nil
}

func (s *stubChallengeSvc) GetAdvertisement(ctx context.Context) ([]models.Advertisement, error) {
	if s.getAdvertisement != nil {
		return s.getAdvertisement(ctx)
	}
	return nil, nil
}

func TestChallengeHandler_GetTimings_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{
			getTimings: func(ctx context.Context, uid uint64) (*models.TimingsData, error) {
				return &models.TimingsData{
					DisplayAdInterval: 1, DisplayQuestionInterval: 2, DisplayAnswerInterval: 3,
					Participants: 4, CorrectAnswers: 5, WrongAnswers: 6,
				}, nil
			},
		})
	})
	defer cleanup()
	cli := pb.NewChallengeServiceClient(conn)
	resp, err := cli.GetTimings(context.Background(), &pb.GetTimingsRequest{UserId: 42})
	if err != nil || resp.Data.Participants != 4 {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestChallengeHandler_GetTimings_MissingUserID(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{})
	})
	defer cleanup()
	cli := pb.NewChallengeServiceClient(conn)
	_, err := cli.GetTimings(context.Background(), &pb.GetTimingsRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func TestChallengeHandler_GetQuestion_NotFound(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{
			getQuestion: func(ctx context.Context, uid uint64) (*models.QuestionResource, error) {
				return nil, service.ErrNoUnansweredQuestions
			},
		})
	})
	defer cleanup()
	cli := pb.NewChallengeServiceClient(conn)
	_, err := cli.GetQuestion(context.Background(), &pb.GetQuestionRequest{UserId: 1})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("got %v", err)
	}
}

func TestChallengeHandler_SubmitAnswer_Validation(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{})
	})
	defer cleanup()
	cli := pb.NewChallengeServiceClient(conn)
	_, err := cli.SubmitAnswer(context.Background(), &pb.SubmitAnswerRequest{UserId: 1})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func TestChallengeHandler_SubmitAnswer_AlreadyAnswered(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{
			submitAnswer: func(ctx context.Context, userID, questionID, answerID uint64) (*models.QuestionResource, error) {
				return nil, service.ErrAlreadyAnswered
			},
		})
	})
	defer cleanup()
	cli := pb.NewChallengeServiceClient(conn)
	_, err := cli.SubmitAnswer(context.Background(), &pb.SubmitAnswerRequest{
		UserId: 1, QuestionId: 2, AnswerId: 3,
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestChallengeHandler_GetAdvertisement_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterChallengeHandler(gs, &stubChallengeSvc{
			getAdvertisement: func(ctx context.Context) ([]models.Advertisement, error) {
				return []models.Advertisement{{
					Code:            "bn-1000",
					Title:           "Matrix exit box",
					URL:             "https://metarang.com/fa/citizen/bn-1000",
					InvestmentAsset: "red",
				}}, nil
			},
		})
	})
	defer cleanup()

	cli := pb.NewChallengeServiceClient(conn)
	resp, err := cli.GetAdvertisement(context.Background(), &pb.GetAdvertisementRequest{})
	if err != nil {
		t.Fatalf("GetAdvertisement failed: %v", err)
	}
	if len(resp.Advertisements) != 1 {
		t.Fatalf("expected 1 advertisement, got %d", len(resp.Advertisements))
	}
	if resp.Advertisements[0].Url != "https://metarang.com/fa/citizen/bn-1000" {
		t.Fatalf("unexpected url: %s", resp.Advertisements[0].Url)
	}
	if resp.Advertisements[0].InvestmentAsset != "red" {
		t.Fatalf("unexpected investment_asset: %s", resp.Advertisements[0].InvestmentAsset)
	}
}
