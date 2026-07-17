package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"metarang/social-service/internal/repository"
	"metarang/social-service/internal/service"
	"metarang/social-service/internal/testutil"
)

func TestFollowService_Follow_Self(t *testing.T) {
	svc := service.NewFollowService(&testutil.MockFollowRepository{}, &testutil.MockUserRepository{}, nil, nil)
	err := svc.Follow(context.Background(), 1, 1)
	require.ErrorIs(t, err, service.ErrCannotFollowSelf)
}

func TestFollowService_Follow_AlreadyFollowing(t *testing.T) {
	fr := &testutil.MockFollowRepository{}
	fr.ExistsFunc = func(ctx context.Context, followerID, followingID uint64) (bool, error) {
		return true, nil
	}
	svc := service.NewFollowService(fr, &testutil.MockUserRepository{}, nil, nil)
	err := svc.Follow(context.Background(), 1, 2)
	require.ErrorIs(t, err, service.ErrAlreadyFollowing)
}

func TestFollowService_Follow_OK_RecordsFollower(t *testing.T) {
	var created bool
	var recordedUser uint64
	fr := &testutil.MockFollowRepository{}
	fr.ExistsFunc = func(ctx context.Context, followerID, followingID uint64) (bool, error) {
		return false, nil
	}
	fr.CreateFunc = func(ctx context.Context, followerID, followingID uint64) error {
		created = true
		require.Equal(t, uint64(1), followerID)
		require.Equal(t, uint64(2), followingID)
		return nil
	}
	levels := &testutil.MockLevelsClient{}
	levels.RecordFollowerFunc = func(ctx context.Context, userID uint64) error {
		recordedUser = userID
		return nil
	}
	auth := &testutil.MockAuthClient{}
	auth.CanFollowFunc = func(ctx context.Context, callerUserID, targetUserID uint64) (bool, error) {
		require.Equal(t, uint64(1), callerUserID)
		require.Equal(t, uint64(2), targetUserID)
		return true, nil
	}
	svc := service.NewFollowService(fr, &testutil.MockUserRepository{}, auth, levels)
	err := svc.Follow(context.Background(), 1, 2)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, uint64(2), recordedUser)
}

func TestFollowService_Follow_ProfileLimitation(t *testing.T) {
	created := false
	fr := &testutil.MockFollowRepository{
		ExistsFunc: func(context.Context, uint64, uint64) (bool, error) {
			return false, nil
		},
		CreateFunc: func(context.Context, uint64, uint64) error {
			created = true
			return nil
		},
	}
	auth := &testutil.MockAuthClient{
		CanFollowFunc: func(ctx context.Context, callerUserID, targetUserID uint64) (bool, error) {
			require.Equal(t, uint64(1), callerUserID)
			require.Equal(t, uint64(2), targetUserID)
			return false, nil
		},
	}

	svc := service.NewFollowService(fr, &testutil.MockUserRepository{}, auth, nil)
	err := svc.Follow(context.Background(), 1, 2)

	require.ErrorIs(t, err, service.ErrProfileLimitation)
	require.False(t, created)
}

func TestFollowService_GetFollowers_BuildsResourcesWithCan(t *testing.T) {
	fr := &testutil.MockFollowRepository{}
	fr.GetFollowersFunc = func(ctx context.Context, userID uint64) ([]uint64, error) {
		return []uint64{10}, nil
	}
	// Viewer (99) does not follow 10; 10 does follow 99 → can.remove_follower
	fr.ExistsFunc = func(ctx context.Context, followerID, followingID uint64) (bool, error) {
		if followerID == 99 && followingID == 10 {
			return false, nil
		}
		if followerID == 10 && followingID == 99 {
			return true, nil
		}
		return false, nil
	}
	ur := &testutil.MockUserRepository{}
	ur.GetUserBasicInfoFunc = func(ctx context.Context, userID uint64) (*repository.UserBasicInfo, error) {
		return &repository.UserBasicInfo{ID: userID, Name: "N", Code: "C"}, nil
	}
	ur.GetProfilePhotosFunc = func(ctx context.Context, userID uint64) ([]string, error) {
		return []string{"http://p"}, nil
	}
	ur.GetUserLevelFunc = func(ctx context.Context, userID uint64) (string, error) {
		return "lvl1", nil
	}
	ur.IsUserOnlineFunc = func(ctx context.Context, userID uint64) (bool, error) {
		return true, nil
	}

	svc := service.NewFollowService(fr, ur, nil, nil)
	list, err := svc.GetFollowers(context.Background(), 99)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "N", list[0].Name)
	require.Equal(t, "lvl1", list[0].Level)
	require.True(t, list[0].Online)
	require.False(t, list[0].Followed)
	require.True(t, list[0].Can.Follow)
	require.False(t, list[0].Can.Unfollow)
	require.True(t, list[0].Can.RemoveFollower)
}

func TestFollowService_Unfollow_OK(t *testing.T) {
	var called bool
	fr := &testutil.MockFollowRepository{}
	fr.DeleteFunc = func(ctx context.Context, followerID, followingID uint64) error {
		called = true
		require.Equal(t, uint64(7), followerID)
		require.Equal(t, uint64(8), followingID)
		return nil
	}
	svc := service.NewFollowService(fr, &testutil.MockUserRepository{}, nil, nil)
	require.NoError(t, svc.Unfollow(context.Background(), 7, 8))
	require.True(t, called)
}

func TestFollowService_GetFollowing_FollowedAndCanUnfollow(t *testing.T) {
	fr := &testutil.MockFollowRepository{}
	fr.GetFollowingFunc = func(ctx context.Context, userID uint64) ([]uint64, error) {
		return []uint64{11, 12}, nil
	}
	fr.ExistsFunc = func(ctx context.Context, followerID, followingID uint64) (bool, error) {
		// Viewer (1) follows 11; does not follow 12 (12 skipped because no user info)
		return followerID == 1 && followingID == 11, nil
	}
	ur := &testutil.MockUserRepository{}
	ur.GetUserBasicInfoFunc = func(ctx context.Context, userID uint64) (*repository.UserBasicInfo, error) {
		if userID == 11 {
			return &repository.UserBasicInfo{ID: 11, Name: "U11", Code: "c11"}, nil
		}
		return nil, nil
	}
	svc := service.NewFollowService(fr, ur, nil, nil)
	list, err := svc.GetFollowing(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, uint64(11), list[0].ID)
	require.True(t, list[0].Followed)
	require.False(t, list[0].Can.Follow)
	require.True(t, list[0].Can.Unfollow)
	require.False(t, list[0].Can.RemoveFollower)
}

func TestFollowService_Remove_DeletesReverse(t *testing.T) {
	var delFollower, delFollowing uint64
	fr := &testutil.MockFollowRepository{}
	fr.DeleteFunc = func(ctx context.Context, followerID, followingID uint64) error {
		delFollower, delFollowing = followerID, followingID
		return nil
	}
	svc := service.NewFollowService(fr, &testutil.MockUserRepository{}, nil, nil)
	err := svc.Remove(context.Background(), 5, 9)
	require.NoError(t, err)
	require.Equal(t, uint64(9), delFollower)
	require.Equal(t, uint64(5), delFollowing)
}
