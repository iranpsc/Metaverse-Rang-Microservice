// Package handler provides gRPC handlers for the levels service.
package handler

import (
	"context"

	"metarang/levels-service/internal/service"
	pb "metarang/shared/pb/levels"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ActivityHandler struct {
	pb.UnimplementedActivityServiceServer
	service *service.ActivityService
}

func NewActivityHandler(service *service.ActivityService) *ActivityHandler {
	return &ActivityHandler{
		service: service,
	}
}

// LogActivity records user activity (login, logout, etc.)
// Implements logic from Laravel's UserObserver
func (h *ActivityHandler) LogActivity(ctx context.Context, req *pb.LogActivityRequest) (*pb.LogActivityResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}
	if req.EventType == "" {
		return nil, status.Errorf(codes.InvalidArgument, "event_type is required")
	}

	activityID, err := h.service.LogActivity(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to log activity: %v", err)
	}

	return &pb.LogActivityResponse{
		Success:    true,
		ActivityId: activityID,
	}, nil
}

// GetUserActivities retrieves user's activity history
func (h *ActivityHandler) GetUserActivities(ctx context.Context, req *pb.GetUserActivitiesRequest) (*pb.UserActivitiesResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	activities, userLog, err := h.service.GetUserActivities(ctx, req.UserId, req.Limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user activities: %v", err)
	}

	return &pb.UserActivitiesResponse{
		Activities: activities,
		UserLog:    userLog,
	}, nil
}

// UpdateActivityScore recalculates and updates user score
// Implements Laravel's UserObserver@calculateScore
func (h *ActivityHandler) UpdateActivityScore(ctx context.Context, req *pb.UpdateActivityScoreRequest) (*pb.UpdateActivityScoreResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	newScore, levelUp, newLevelID, err := h.service.UpdateActivityScore(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update activity score: %v", err)
	}

	return &pb.UpdateActivityScoreResponse{
		Success:    true,
		NewScore:   newScore,
		LevelUp:    levelUp,
		NewLevelId: newLevelID,
	}, nil
}

// RecordTrade records a trade transaction for score calculation
// Implements Laravel's UserObserver@traded
func (h *ActivityHandler) RecordTrade(ctx context.Context, req *pb.RecordTradeRequest) (*pb.RecordTradeResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	err := h.service.RecordTrade(ctx, req.UserId, req.IrrAmount, req.PscAmount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record trade: %v", err)
	}

	return &pb.RecordTradeResponse{
		Success: true,
	}, nil
}

// RecordDeposit records a deposit for score calculation
// Implements Laravel's UserObserver@deposit
func (h *ActivityHandler) RecordDeposit(ctx context.Context, req *pb.RecordDepositRequest) (*pb.RecordDepositResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	err := h.service.RecordDeposit(ctx, req.UserId, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record deposit: %v", err)
	}

	return &pb.RecordDepositResponse{
		Success: true,
	}, nil
}

// RecordFollower records a new follower for score calculation
// Implements Laravel's UserObserver@followed
func (h *ActivityHandler) RecordFollower(ctx context.Context, req *pb.RecordFollowerRequest) (*pb.RecordFollowerResponse, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}

	err := h.service.RecordFollower(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record follower: %v", err)
	}

	return &pb.RecordFollowerResponse{
		Success: true,
	}, nil
}
