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

type stubFollowSvc struct {
	getFollowers func(context.Context, uint64) ([]*models.FollowResource, error)
	getFollowing func(context.Context, uint64) ([]*models.FollowResource, error)
	follow       func(context.Context, uint64, uint64) error
	unfollow     func(context.Context, uint64, uint64) error
	remove       func(context.Context, uint64, uint64) error
}

func (s *stubFollowSvc) GetFollowers(ctx context.Context, userID uint64) ([]*models.FollowResource, error) {
	if s.getFollowers != nil {
		return s.getFollowers(ctx, userID)
	}
	return nil, nil
}

func (s *stubFollowSvc) GetFollowing(ctx context.Context, userID uint64) ([]*models.FollowResource, error) {
	if s.getFollowing != nil {
		return s.getFollowing(ctx, userID)
	}
	return nil, nil
}

func (s *stubFollowSvc) Follow(ctx context.Context, userID, targetUserID uint64) error {
	if s.follow != nil {
		return s.follow(ctx, userID, targetUserID)
	}
	return nil
}

func (s *stubFollowSvc) Unfollow(ctx context.Context, userID, targetUserID uint64) error {
	if s.unfollow != nil {
		return s.unfollow(ctx, userID, targetUserID)
	}
	return nil
}

func (s *stubFollowSvc) Remove(ctx context.Context, userID, targetUserID uint64) error {
	if s.remove != nil {
		return s.remove(ctx, userID, targetUserID)
	}
	return nil
}

func TestFollowHandler_GetFollowing_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{
			getFollowing: func(ctx context.Context, uid uint64) ([]*models.FollowResource, error) {
				return []*models.FollowResource{{ID: 2, Name: "B", Code: "c"}}, nil
			},
		})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	resp, err := cli.GetFollowing(context.Background(), &pb.GetFollowingRequest{UserId: 3})
	if err != nil || len(resp.Data) != 1 {
		t.Fatal(err, resp)
	}
}

func TestFollowHandler_GetFollowers_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{
			getFollowers: func(ctx context.Context, uid uint64) ([]*models.FollowResource, error) {
				return []*models.FollowResource{{ID: 1, Name: "A"}}, nil
			},
		})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	resp, err := cli.GetFollowers(context.Background(), &pb.GetFollowersRequest{UserId: 9})
	if err != nil || len(resp.Data) != 1 {
		t.Fatal(err, resp)
	}
}

func TestFollowHandler_Follow_AlreadyFollowing_PermissionDenied(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{
			follow: func(ctx context.Context, userID, targetUserID uint64) error {
				return service.ErrAlreadyFollowing
			},
		})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Follow(context.Background(), &pb.FollowRequest{UserId: 1, TargetUserId: 2})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestFollowHandler_Follow_Self_PermissionDenied(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{
			follow: func(ctx context.Context, userID, targetUserID uint64) error {
				return service.ErrCannotFollowSelf
			},
		})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Follow(context.Background(), &pb.FollowRequest{UserId: 1, TargetUserId: 1})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestFollowHandler_Follow_ProfileLimitation(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{
			follow: func(ctx context.Context, userID, targetUserID uint64) error {
				return service.ErrProfileLimitation
			},
		})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Follow(context.Background(), &pb.FollowRequest{UserId: 1, TargetUserId: 2})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestFollowHandler_Unfollow_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Unfollow(context.Background(), &pb.UnfollowRequest{UserId: 1, TargetUserId: 2})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFollowHandler_Remove_OK(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Remove(context.Background(), &pb.RemoveRequest{UserId: 1, TargetUserId: 2})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFollowHandler_Unfollow_ZeroIDs(t *testing.T) {
	conn, cleanup := testutil.DialBufConn(func(gs *grpc.Server) {
		handler.RegisterFollowHandler(gs, &stubFollowSvc{})
	})
	defer cleanup()
	cli := pb.NewFollowServiceClient(conn)
	_, err := cli.Unfollow(context.Background(), &pb.UnfollowRequest{})
	if err != nil {
		t.Fatal(err)
	}
}
