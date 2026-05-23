package handler

import (
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
		Score:    100,
		ImageUrl: "http://admin.example.com/uploads/level.png",
	}

	out := userListLevelToHTTP(lvl)
	require.NotNil(t, out)
	assert.Equal(t, uint64(3), out["id"])
	assert.Equal(t, "Citizen", out["name"])
	assert.Equal(t, "citizen-baguette", out["slug"])
	assert.Equal(t, int32(100), out["score"])
	assert.Equal(t, "http://admin.example.com/uploads/level.png", out["image"])
}

func TestBuildListUsersLevelsShape(t *testing.T) {
	item := &pb.UserListItem{
		Id:   1,
		Name: "Test User",
		Code: "hm-1",
		Levels: &pb.UserLevelInfo{
			Current: &pb.Level{Id: 2, Title: "Reporter", Slug: "reporter-baguette", ImageUrl: "http://x/img.png"},
			Previous: []*pb.Level{
				{Id: 1, Title: "Citizen", Slug: "citizen-baguette", ImageUrl: "http://x/c.png"},
			},
		},
	}

	userData := map[string]interface{}{
		"id": item.Id,
	}
	if item.Levels != nil {
		levelsData := map[string]interface{}{}
		if item.Levels.Current != nil {
			levelsData["current"] = userListLevelToHTTP(item.Levels.Current)
		}
		if len(item.Levels.Previous) > 0 {
			previous := make([]map[string]interface{}, 0, len(item.Levels.Previous))
			for _, lvl := range item.Levels.Previous {
				previous = append(previous, userListLevelToHTTP(lvl))
			}
			levelsData["previous"] = previous
		}
		userData["levels"] = levelsData
	}

	levels, ok := userData["levels"].(map[string]interface{})
	require.True(t, ok)

	current, ok := levels["current"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "reporter-baguette", current["slug"])

	previous, ok := levels["previous"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, previous, 1)
	assert.Equal(t, "citizen-baguette", previous[0]["slug"])
}
