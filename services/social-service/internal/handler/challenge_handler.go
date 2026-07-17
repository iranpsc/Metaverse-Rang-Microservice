package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "metarang/shared/pb/social"
	"metarang/social-service/internal/models"
	"metarang/social-service/internal/service"
)

type challengeHandler struct {
	pb.UnimplementedChallengeServiceServer
	challengeService service.ChallengeService
}

func RegisterChallengeHandler(grpcServer *grpc.Server, challengeService service.ChallengeService) {
	pb.RegisterChallengeServiceServer(grpcServer, &challengeHandler{
		challengeService: challengeService,
	})
}

func (h *challengeHandler) GetTimings(ctx context.Context, req *pb.GetTimingsRequest) (*pb.GetTimingsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	timings, err := h.challengeService.GetTimings(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get timings: %v", err)
	}

	return &pb.GetTimingsResponse{
		Data: &pb.TimingsData{
			DisplayAdInterval:       timings.DisplayAdInterval,
			DisplayQuestionInterval: timings.DisplayQuestionInterval,
			DisplayAnswerInterval:   timings.DisplayAnswerInterval,
			Participants:            timings.Participants,
			CorrectAnswers:          timings.CorrectAnswers,
			WrongAnswers:            timings.WrongAnswers,
		},
	}, nil
}

func (h *challengeHandler) GetQuestion(ctx context.Context, req *pb.GetQuestionRequest) (*pb.GetQuestionResponse, error) {
	// Use user ID from request (set by gateway from authenticated user)
	userID := req.UserId
	if userID == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	question, err := h.challengeService.GetQuestion(ctx, userID)
	if err != nil {
		if errors.Is(err, service.ErrNoUnansweredQuestions) {
			return nil, status.Errorf(codes.NotFound, "no unanswered questions available")
		}
		return nil, status.Errorf(codes.Internal, "failed to get question: %v", err)
	}

	return &pb.GetQuestionResponse{
		Data: convertQuestionResourceToProto(question),
	}, nil
}

func (h *challengeHandler) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	// Validate required fields
	if req.QuestionId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "question_id is required")
	}
	if req.AnswerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "answer_id is required")
	}

	// Use user ID from request (set by gateway from authenticated user)
	userID := req.UserId
	if userID == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	question, err := h.challengeService.SubmitAnswer(ctx, userID, req.QuestionId, req.AnswerId)
	if err != nil {
		return nil, mapChallengeError(err)
	}

	return &pb.SubmitAnswerResponse{
		Data: convertQuestionResourceToProto(question),
	}, nil
}

func (h *challengeHandler) GetAdvertisement(ctx context.Context, _ *pb.GetAdvertisementRequest) (*pb.GetAdvertisementResponse, error) {
	advertisements, err := h.challengeService.GetAdvertisement(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get advertisements: %v", err)
	}

	resources := make([]*pb.AdvertisementResource, 0, len(advertisements))
	for _, advertisement := range advertisements {
		resources = append(resources, &pb.AdvertisementResource{
			Code:            advertisement.Code,
			Title:           advertisement.Title,
			Description:     advertisement.Description,
			InvestmentValue: advertisement.InvestmentValue,
			EndsAt:          advertisement.EndsAt,
			VideoUrl:        advertisement.VideoURL,
			ImageUrl:        advertisement.ImageURL,
			Url:             advertisement.URL,
			InvestmentAsset: advertisement.InvestmentAsset,
		})
	}

	return &pb.GetAdvertisementResponse{Advertisements: resources}, nil
}

func convertQuestionResourceToProto(resource *models.QuestionResource) *pb.QuestionResource {
	answerResources := make([]*pb.AnswerResource, 0, len(resource.Answers))
	for _, answer := range resource.Answers {
		answerResources = append(answerResources, &pb.AnswerResource{
			Id:             answer.ID,
			Title:          answer.Title,
			Image:          answer.Image,
			IsCorrect:      answer.IsCorrect,
			VotePercentage: answer.VotePercentage,
		})
	}

	return &pb.QuestionResource{
		Id:           resource.ID,
		Title:        resource.Title,
		Image:        resource.Image,
		Prize:        resource.Prize,
		Participants: int32(resource.Participants),
		Views:        int32(resource.Views),
		CreatorCode:  resource.CreatorCode,
		Answers:      answerResources,
	}
}

func mapChallengeError(err error) error {
	switch {
	case errors.Is(err, service.ErrQuestionNotFound):
		return status.Errorf(codes.NotFound, "question not found")
	case errors.Is(err, service.ErrAnswerNotFound):
		return status.Errorf(codes.NotFound, "answer not found")
	case errors.Is(err, service.ErrAnswerMismatch):
		return status.Errorf(codes.InvalidArgument, "answer does not belong to the given question")
	case errors.Is(err, service.ErrAlreadyAnswered):
		return status.Errorf(codes.PermissionDenied, "user has already answered this question")
	case errors.Is(err, service.ErrNoUnansweredQuestions):
		return status.Errorf(codes.NotFound, "no unanswered questions available")
	default:
		return status.Errorf(codes.Internal, "operation failed: %v", err)
	}
}
