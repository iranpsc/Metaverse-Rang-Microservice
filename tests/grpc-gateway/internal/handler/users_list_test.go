package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "metargb/shared/pb/auth"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newUsersAuthHandler(t *testing.T, user *testutil.MockUserService) *handler.AuthHandler {
	t.Helper()
	conn, cleanup := testutil.DialAuthConn(&testutil.AuthMocks{User: user})
	t.Cleanup(cleanup)
	return handler.NewAuthHandler(conn, nil, "en")
}

func TestListUsers_LevelMapping(t *testing.T) {
	user := &testutil.MockUserService{}
	user.ListUsersFunc = func(_ context.Context, _ *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
		return &pb.ListUsersResponse{
			Data: []*pb.UserListItem{
				{
					Id:           1,
					Name:         "Test User",
					Code:         "hm-1",
					Score:        50,
					ProfilePhoto: "https://cdn.example.com/photo.jpg",
					Levels: &pb.UserLevelInfo{
						Current: &pb.Level{Id: 2, Title: "Reporter", Slug: "reporter-baguette", ImageUrl: "http://x/img.png"},
						Previous: []*pb.Level{
							{Id: 1, Title: "Citizen", Slug: "citizen-baguette", ImageUrl: "http://x/c.png"},
						},
					},
				},
			},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}
	h := newUsersAuthHandler(t, user)

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	item := data[0].(map[string]interface{})

	assert.EqualValues(t, 1, item["id"])
	assert.Equal(t, "Test User", item["name"])
	assert.Equal(t, "hm-1", item["code"])
	assert.EqualValues(t, 50, item["score"])
	assert.Equal(t, "https://cdn.example.com/photo.jpg", item["profile_photo"])

	levels := item["levels"].(map[string]interface{})
	current := levels["current"].(map[string]interface{})
	assert.Equal(t, "reporter-baguette", current["slug"])
	assert.Equal(t, "Reporter", current["name"])
	assert.Equal(t, "http://x/img.png", current["image"])
	_, hasScore := current["score"]
	assert.False(t, hasScore)

	previous := levels["previous"].([]interface{})
	require.Len(t, previous, 1)
	assert.Equal(t, "citizen-baguette", previous[0].(map[string]interface{})["slug"])
}

func TestListUsers_EmptyLevels(t *testing.T) {
	user := &testutil.MockUserService{}
	user.ListUsersFunc = func(_ context.Context, _ *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
		return &pb.ListUsersResponse{
			Data: []*pb.UserListItem{
				{Id: 2, Name: "No Levels", Code: "hm-2", Score: 0, Levels: &pb.UserLevelInfo{}},
			},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}
	h := newUsersAuthHandler(t, user)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	item := body["data"].([]interface{})[0].(map[string]interface{})
	levels := item["levels"].(map[string]interface{})
	assert.Nil(t, levels["current"])
	previous := levels["previous"].([]interface{})
	assert.Empty(t, previous)
}

func TestListUsers_LaravelPagination(t *testing.T) {
	user := &testutil.MockUserService{}
	user.ListUsersFunc = func(_ context.Context, _ *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
		return &pb.ListUsersResponse{
			Data: []*pb.UserListItem{
				{Id: 1, Name: "User One", Code: "hm-1", Score: 10, Levels: &pb.UserLevelInfo{}},
			},
			Meta: &pb.PaginationMeta{
				CurrentPage: 1,
				NextPageUrl: "?page=2",
			},
		}, nil
	}
	h := newUsersAuthHandler(t, user)

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	req.Host = "api.example.test"
	w := httptest.NewRecorder()
	h.ListUsers(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data := body["data"].([]interface{})
	require.Len(t, data, 1)

	links := body["links"].(map[string]interface{})
	assert.Contains(t, links["first"], "/api/users")
	assert.Nil(t, links["last"])
	assert.Nil(t, links["prev"])
	assert.NotNil(t, links["next"])

	meta := body["meta"].(map[string]interface{})
	assert.EqualValues(t, 1, meta["current_page"])
	assert.EqualValues(t, 20, meta["per_page"])
	assert.EqualValues(t, 1, meta["from"])
	assert.EqualValues(t, 1, meta["to"])
	assert.Contains(t, meta["path"], "/api/users")
}

func TestListUsers_NoDoubleWrap(t *testing.T) {
	user := &testutil.MockUserService{}
	user.ListUsersFunc = func(_ context.Context, _ *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
		return &pb.ListUsersResponse{
			Data: []*pb.UserListItem{{Id: 1, Name: "User", Code: "hm-1", Levels: &pb.UserLevelInfo{}}},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}
	h := newUsersAuthHandler(t, user)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Host = "api.example.test"
	w := httptest.NewRecorder()
	h.ListUsers(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	_, hasTopLevelData := body["data"]
	assert.True(t, hasTopLevelData)
	_, hasLinks := body["links"]
	assert.True(t, hasLinks)
	_, hasMeta := body["meta"]
	assert.True(t, hasMeta)

	if inner, ok := body["data"].(map[string]interface{}); ok {
		_, hasDoubleData := inner["data"]
		assert.False(t, hasDoubleData, "response must not double-wrap data")
	}
}
