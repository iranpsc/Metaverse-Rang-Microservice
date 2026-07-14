package handler

import (
	"context"

	"metarang/levels-service/internal/service"
	pb "metarang/shared/pb/levels"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChallengeHandler struct {
	pb.UnimplementedChallengeServiceServer
	service *service.ChallengeService
}

func NewChallengeHandler(service *service.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{
		service: service,
	}
}

// GetQuestion retrieves a random unanswered question for the user
// Implements Laravel's ChallengeController@getQuestion
func (h *ChallengeHandler) GetQuestion(ctx context.Context, req *pb.GetQuestionRequest) (*pb.QuestionResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	question, hasQuestion, err := h.service.GetQuestion(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get question: %v", err)
	}

	return &pb.QuestionResponse{
		Question:    question,
		HasQuestion: hasQuestion,
	}, nil
}

// SubmitAnswer submits an answer to a question and returns the result
// Implements Laravel's ChallengeController@answerResult
func (h *ChallengeHandler) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.AnswerResultResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}
	if req.QuestionId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "question_id is required")
	}
	if req.AnswerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "answer_id is required")
	}

	isCorrect, prizeAwarded, question, err := h.service.SubmitAnswer(ctx, req.UserId, req.QuestionId, req.AnswerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to submit answer: %v", err)
	}

	return &pb.AnswerResultResponse{
		IsCorrect:    isCorrect,
		PrizeAwarded: prizeAwarded,
		Question:     question,
	}, nil
}

// GetTimings retrieves challenge timing configuration and user stats
// Implements Laravel's ChallengeController@getTimings
func (h *ChallengeHandler) GetTimings(ctx context.Context, req *pb.GetTimingsRequest) (*pb.TimingsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	timings, err := h.service.GetTimings(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get timings: %v", err)
	}

	return timings, nil
}

// GetAdvertisement retrieves the static list of challenge advertisers.
func (h *ChallengeHandler) GetAdvertisement(ctx context.Context, req *pb.GetAdvertisementRequest) (*pb.GetAdvertisementResponse, error) {
	ads, err := h.service.GetAdvertisement(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get advertisements: %v", err)
	}

	pbAds := make([]*pb.Advertisement, 0, len(ads))
	for _, ad := range ads {
		pbAds = append(pbAds, &pb.Advertisement{
			Code:            ad.Code,
			Title:           ad.Title,
			Description:     ad.Description,
			InvestmentValue: ad.InvestmentValue,
			EndsAt:          ad.EndsAt,
			VideoUrl:        ad.VideoURL,
			ImageUrl:        ad.ImageURL,
		})
	}

	return &pb.GetAdvertisementResponse{Advertisements: pbAds}, nil
}
