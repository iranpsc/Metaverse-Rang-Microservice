package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/commercial-service/internal/service"
	pb "metarang/shared/pb/commercial"
)

type UserVariableHandler struct {
	pb.UnimplementedUserVariableServiceServer
	userVariableService service.UserVariableService
}

func NewUserVariableHandler(userVariableService service.UserVariableService) *UserVariableHandler {
	return &UserVariableHandler{userVariableService: userVariableService}
}

func RegisterUserVariableHandler(grpcServer *grpc.Server, userVariableService service.UserVariableService) {
	handler := NewUserVariableHandler(userVariableService)
	pb.RegisterUserVariableServiceServer(grpcServer, handler)
}

func (h *UserVariableHandler) CreateUserVariables(ctx context.Context, req *pb.CreateUserVariablesRequest) (*emptypb.Empty, error) {
	if req == nil || req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := h.userVariableService.CreateUserVariables(ctx, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user variables: %v", err)
	}

	return &emptypb.Empty{}, nil
}
