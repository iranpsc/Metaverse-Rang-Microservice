package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metarang/shared/pb/social"
	"metarang/social-service/internal/models"
	"metarang/social-service/internal/service"
)

type followHandler struct {
	pb.UnimplementedFollowServiceServer
	followService service.FollowService
}

func RegisterFollowHandler(grpcServer *grpc.Server, followService service.FollowService) {
	pb.RegisterFollowServiceServer(grpcServer, &followHandler{
		followService: followService,
	})
}

func (h *followHandler) GetFollowers(ctx context.Context, req *pb.GetFollowersRequest) (*pb.GetFollowersResponse, error) {
	resources, err := h.followService.GetFollowers(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get followers: %v", err)
	}

	protoResources := make([]*pb.FollowResource, 0, len(resources))
	for _, resource := range resources {
		protoResources = append(protoResources, convertFollowResourceToProto(resource))
	}

	return &pb.GetFollowersResponse{
		Data: protoResources,
	}, nil
}

func (h *followHandler) GetFollowing(ctx context.Context, req *pb.GetFollowingRequest) (*pb.GetFollowingResponse, error) {
	resources, err := h.followService.GetFollowing(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get following: %v", err)
	}

	protoResources := make([]*pb.FollowResource, 0, len(resources))
	for _, resource := range resources {
		protoResources = append(protoResources, convertFollowResourceToProto(resource))
	}

	return &pb.GetFollowingResponse{
		Data: protoResources,
	}, nil
}

func (h *followHandler) Follow(ctx context.Context, req *pb.FollowRequest) (*emptypb.Empty, error) {
	err := h.followService.Follow(ctx, req.UserId, req.TargetUserId)
	if err != nil {
		return nil, mapFollowError(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *followHandler) Unfollow(ctx context.Context, req *pb.UnfollowRequest) (*emptypb.Empty, error) {
	err := h.followService.Unfollow(ctx, req.UserId, req.TargetUserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unfollow: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (h *followHandler) Remove(ctx context.Context, req *pb.RemoveRequest) (*emptypb.Empty, error) {
	err := h.followService.Remove(ctx, req.UserId, req.TargetUserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove follower: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func convertFollowResourceToProto(resource *models.FollowResource) *pb.FollowResource {
	return &pb.FollowResource{
		Id:            resource.ID,
		Name:          resource.Name,
		Code:          resource.Code,
		ProfilePhotos: resource.ProfilePhotos,
		Level:         resource.Level,
		Online:        resource.Online,
		Followed:      resource.Followed,
		Can: &pb.FollowPermissions{
			Follow:         resource.Can.Follow,
			Unfollow:       resource.Can.Unfollow,
			RemoveFollower: resource.Can.RemoveFollower,
		},
	}
}

func mapFollowError(err error) error {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		return status.Errorf(codes.NotFound, "user not found")
	case errors.Is(err, service.ErrCannotFollowSelf):
		// Laravel UserPolicy::follow denies with HTTP 403
		return status.Errorf(codes.PermissionDenied, "cannot follow yourself")
	case errors.Is(err, service.ErrAlreadyFollowing):
		// Laravel UserPolicy::follow denies with HTTP 403
		return status.Errorf(codes.PermissionDenied, "already following this user")
	case errors.Is(err, service.ErrProfileLimitation):
		return status.Errorf(codes.PermissionDenied, "این کاربر امکان دنبال کردن را  برای شما غیر فعال کرده است.")
	default:
		return status.Errorf(codes.Internal, "operation failed: %v", err)
	}
}
