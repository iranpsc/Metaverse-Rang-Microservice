package client_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"metarang/buildings-service/internal/client"
	pb "metarang/shared/pb/levels"
)

type levelStub struct {
	pb.LevelServiceClient
	err error
}

func (s levelStub) GetUserLevel(_ context.Context, r *pb.GetUserLevelRequest, _ ...grpc.CallOption) (*pb.UserLevelResponse, error) {
	return &pb.UserLevelResponse{UserScore: int32(r.UserId)}, s.err
}
func (s levelStub) GetAllLevels(context.Context, *pb.GetAllLevelsRequest, ...grpc.CallOption) (*pb.LevelsResponse, error) {
	return &pb.LevelsResponse{Levels: []*pb.Level{{Slug: "bronze"}}}, s.err
}

func TestLevelsClient(t *testing.T) {
	c := client.NewLevelsClientFromGRPC(levelStub{})
	if response, err := c.GetUserLevel(context.Background(), 8); err != nil || response.UserScore != 8 {
		t.Fatalf("level=%+v err=%v", response, err)
	}
	if response, err := c.GetAllLevels(context.Background()); err != nil || len(response.Levels) != 1 {
		t.Fatalf("levels=%+v err=%v", response, err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	bad := client.NewLevelsClientFromGRPC(levelStub{err: errors.New("down")})
	if _, err := bad.GetUserLevel(context.Background(), 8); err == nil {
		t.Fatal("missing level error")
	}
	if _, err := bad.GetAllLevels(context.Background()); err == nil {
		t.Fatal("missing levels error")
	}
}
