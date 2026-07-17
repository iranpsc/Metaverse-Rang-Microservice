package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	socialpb "metarang/shared/pb/social"

	"metarang/grpc-gateway/internal/handler"
	"metarang/grpc-gateway/internal/testutil"
)

func newSocialHandler(t *testing.T, follow *testutil.MockFollowService, user *testutil.MockUserService) *handler.SocialHandler {
	t.Helper()
	conn, cleanup := testutil.DialSocialConn(follow, user)
	t.Cleanup(cleanup)
	return handler.NewSocialHandler(conn, conn)
}

func sampleFollowResources(n int) []*socialpb.FollowResource {
	out := make([]*socialpb.FollowResource, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, &socialpb.FollowResource{
			Id:            uint64(i),
			Name:          "User",
			Code:          "c",
			Level:         "lvl1",
			Online:        i%2 == 0,
			ProfilePhotos: []string{"http://p"},
			Followed:      i == 1,
			Can: &socialpb.FollowPermissions{
				Follow:         i != 1,
				Unfollow:       i == 1,
				RemoveFollower: i == 2,
			},
		})
	}
	return out
}

func TestGetFollowers_PaginatedShapeAndPerPage(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.GetFollowersFunc = func(_ context.Context, req *socialpb.GetFollowersRequest) (*socialpb.GetFollowersResponse, error) {
		require.Equal(t, uint64(42), req.UserId)
		return &socialpb.GetFollowersResponse{Data: sampleFollowResources(12)}, nil
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/followers", nil), 42)
	w := httptest.NewRecorder()
	h.GetFollowers(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data := body["data"].([]interface{})
	require.Len(t, data, 10)

	meta := body["meta"].(map[string]interface{})
	assert.EqualValues(t, 1, meta["current_page"])
	assert.EqualValues(t, 10, meta["per_page"])
	assert.EqualValues(t, 1, meta["from"])
	assert.EqualValues(t, 10, meta["to"])

	links := body["links"].(map[string]interface{})
	assert.NotNil(t, links["next"])
	assert.Nil(t, links["prev"])

	first := data[0].(map[string]interface{})
	assert.EqualValues(t, 1, first["id"])
	assert.Equal(t, "User", first["name"])
	assert.Equal(t, "c", first["code"])
	assert.Equal(t, "lvl1", first["level"])
	assert.Equal(t, true, first["followed"])
	photos, ok := first["profile_photos"].([]interface{})
	require.True(t, ok)
	require.Len(t, photos, 1)
	_, onlineOK := first["online"]
	assert.True(t, onlineOK)

	can := first["can"].(map[string]interface{})
	assert.Equal(t, false, can["follow"])
	assert.Equal(t, true, can["unfollow"])
	assert.Equal(t, false, can["remove_follower"])
}

func TestGetFollowers_Page2(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.GetFollowersFunc = func(_ context.Context, _ *socialpb.GetFollowersRequest) (*socialpb.GetFollowersResponse, error) {
		return &socialpb.GetFollowersResponse{Data: sampleFollowResources(12)}, nil
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/followers?page=2", nil), 1)
	w := httptest.NewRecorder()
	h.GetFollowers(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data := body["data"].([]interface{})
	require.Len(t, data, 2)
	assert.EqualValues(t, 11, data[0].(map[string]interface{})["id"])

	meta := body["meta"].(map[string]interface{})
	assert.EqualValues(t, 2, meta["current_page"])
	assert.EqualValues(t, 11, meta["from"])
	assert.EqualValues(t, 12, meta["to"])

	links := body["links"].(map[string]interface{})
	assert.Nil(t, links["next"])
	assert.NotNil(t, links["prev"])
}

func TestGetFollowing_Paginated(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.GetFollowingFunc = func(_ context.Context, req *socialpb.GetFollowingRequest) (*socialpb.GetFollowingResponse, error) {
		require.Equal(t, uint64(7), req.UserId)
		return &socialpb.GetFollowingResponse{Data: sampleFollowResources(3)}, nil
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/following", nil), 7)
	w := httptest.NewRecorder()
	h.GetFollowing(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data := body["data"].([]interface{})
	require.Len(t, data, 3)

	meta := body["meta"].(map[string]interface{})
	assert.EqualValues(t, 10, meta["per_page"])
	assert.EqualValues(t, 1, meta["current_page"])

	links := body["links"].(map[string]interface{})
	assert.Nil(t, links["next"])
}

func TestFollow_OK(t *testing.T) {
	var gotFollower, gotTarget uint64
	follow := &testutil.MockFollowService{}
	follow.FollowFunc = func(_ context.Context, req *socialpb.FollowRequest) (*emptypb.Empty, error) {
		gotFollower, gotTarget = req.UserId, req.TargetUserId
		return &emptypb.Empty{}, nil
	}
	h := newSocialHandler(t, follow, &testutil.MockUserService{})

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/follow/99", nil), 1)
	w := httptest.NewRecorder()
	h.Follow(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint64(1), gotFollower)
	assert.Equal(t, uint64(99), gotTarget)
}

func TestFollow_ProfileLimitationDenied(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.FollowFunc = func(_ context.Context, _ *socialpb.FollowRequest) (*emptypb.Empty, error) {
		return nil, status.Error(codes.PermissionDenied, "این کاربر امکان دنبال کردن را  برای شما غیر فعال کرده است.")
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/follow/99", nil), 1)
	w := httptest.NewRecorder()
	h.Follow(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body["error"], "دنبال کردن")
}

func TestFollow_SelfDenied(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.FollowFunc = func(_ context.Context, _ *socialpb.FollowRequest) (*emptypb.Empty, error) {
		return nil, status.Error(codes.PermissionDenied, "cannot follow yourself")
	}
	h := newSocialHandler(t, follow, &testutil.MockUserService{})

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/follow/1", nil), 1)
	w := httptest.NewRecorder()
	h.Follow(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestFollow_AlreadyFollowingDenied(t *testing.T) {
	follow := &testutil.MockFollowService{}
	follow.FollowFunc = func(_ context.Context, _ *socialpb.FollowRequest) (*emptypb.Empty, error) {
		return nil, status.Error(codes.PermissionDenied, "already following this user")
	}
	h := newSocialHandler(t, follow, &testutil.MockUserService{})

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/follow/2", nil), 1)
	w := httptest.NewRecorder()
	h.Follow(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestUnfollow_OK(t *testing.T) {
	var gotFollower, gotTarget uint64
	follow := &testutil.MockFollowService{}
	follow.UnfollowFunc = func(_ context.Context, req *socialpb.UnfollowRequest) (*emptypb.Empty, error) {
		gotFollower, gotTarget = req.UserId, req.TargetUserId
		return &emptypb.Empty{}, nil
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/unfollow/5", nil), 3)
	w := httptest.NewRecorder()
	h.Unfollow(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint64(3), gotFollower)
	assert.Equal(t, uint64(5), gotTarget)
}

func TestRemove_OK(t *testing.T) {
	var gotUser, gotTarget uint64
	follow := &testutil.MockFollowService{}
	follow.RemoveFunc = func(_ context.Context, req *socialpb.RemoveRequest) (*emptypb.Empty, error) {
		gotUser, gotTarget = req.UserId, req.TargetUserId
		return &emptypb.Empty{}, nil
	}
	h := newSocialHandler(t, follow, nil)

	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodGet, "/api/remove/8", nil), 3)
	w := httptest.NewRecorder()
	h.Remove(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint64(3), gotUser)
	assert.Equal(t, uint64(8), gotTarget)
}

func TestGetFollowers_MethodNotAllowed(t *testing.T) {
	h := newSocialHandler(t, &testutil.MockFollowService{}, nil)
	req := testutil.RequestWithUser(httptest.NewRequest(http.MethodPost, "/api/followers", nil), 1)
	w := httptest.NewRecorder()
	h.GetFollowers(w, req)
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
