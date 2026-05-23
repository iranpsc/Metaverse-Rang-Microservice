package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "metargb/shared/pb/auth"
)

func TestUserListLevelToHTTP(t *testing.T) {
	lvl := &pb.Level{
		Id:       3,
		Title:    "Citizen",
		Slug:     "citizen-baguette",
		ImageUrl: "http://admin.example.com/uploads/level.png",
	}

	out := userListLevelToHTTP(lvl)
	require.NotNil(t, out)
	assert.Equal(t, uint64(3), out["id"])
	assert.Equal(t, "Citizen", out["name"])
	assert.Equal(t, "citizen-baguette", out["slug"])
	assert.Equal(t, "http://admin.example.com/uploads/level.png", out["image"])
	_, hasScore := out["score"]
	assert.False(t, hasScore)
}

func TestBuildListUserItemHTTP(t *testing.T) {
	item := &pb.UserListItem{
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
	}

	out := buildListUserItemHTTP(item)
	assert.Equal(t, uint64(1), out["id"])
	assert.Equal(t, "Test User", out["name"])
	assert.Equal(t, "hm-1", out["code"])
	assert.Equal(t, int32(50), out["score"])
	assert.Equal(t, "https://cdn.example.com/photo.jpg", out["profile_photo"])

	levels, ok := out["levels"].(map[string]interface{})
	require.True(t, ok)

	current, ok := levels["current"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "reporter-baguette", current["slug"])

	previous, ok := levels["previous"].([]interface{})
	require.True(t, ok)
	require.Len(t, previous, 1)
	first, ok := previous[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "citizen-baguette", first["slug"])
}

func TestBuildListUserItemHTTP_EmptyLevels(t *testing.T) {
	item := &pb.UserListItem{
		Id:    2,
		Name:  "No Levels",
		Code:  "hm-2",
		Score: 0,
	}

	out := buildListUserItemHTTP(item)
	levels, ok := out["levels"].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, levels["current"])
	previous, ok := levels["previous"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, previous)
}

func TestBuildListUsersHTTPResponse(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users?page=1", nil)
	req.Host = "api.example.test"

	resp := &pb.ListUsersResponse{
		Data: []*pb.UserListItem{
			{Id: 1, Name: "User One", Code: "hm-1", Score: 10, Levels: &pb.UserLevelInfo{}},
		},
		Meta: &pb.PaginationMeta{
			CurrentPage: 1,
			NextPageUrl: "?page=2",
		},
	}

	out := buildListUsersHTTPResponse(req, resp)

	data, ok := out["data"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)

	links, ok := out["links"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, links["first"], "/api/users")
	assert.Nil(t, links["last"])
	assert.Nil(t, links["prev"])
	assert.NotNil(t, links["next"])

	meta, ok := out["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, int32(1), meta["current_page"])
	assert.Equal(t, int32(20), meta["per_page"])
	assert.Equal(t, 1, meta["from"])
	assert.Equal(t, 1, meta["to"])
	assert.Contains(t, meta["path"], "/api/users")
}

func TestWriteJSONListUsersSkipsDoubleWrap(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Host = "api.example.test"

	resp := &pb.ListUsersResponse{
		Data: []*pb.UserListItem{{Id: 1, Name: "User", Code: "hm-1", Levels: &pb.UserLevelInfo{}}},
		Meta: &pb.PaginationMeta{CurrentPage: 1},
	}

	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, buildListUsersHTTPResponse(req, resp))

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
