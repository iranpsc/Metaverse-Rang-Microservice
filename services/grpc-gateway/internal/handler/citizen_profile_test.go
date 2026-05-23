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

func TestBuildCitizenProfileHTTPResponse_LaravelShape(t *testing.T) {
	resp := &pb.CitizenProfileResponse{
		ProfilePhotos: []*pb.ProfilePhoto{{Id: 1, Url: "https://example.com/p.jpg"}},
		Kyc: &pb.CitizenKYC{
			Fname:    "Ali",
			Lname:    "Rezaei",
			BirthDate: "1400/01/01",
		},
		Code:                       "hm-2000001",
		Name:                       "Ali Rezaei",
		Position:                   "مدیریت موازی",
		RegisteredAt:               "1400/01/01",
		Score:                      100,
		ScorePercentageToNextLevel: 42,
		Customs: &pb.CitizenCustoms{
			Occupation: "dev",
			Passions:   map[string]string{"music": "http://example.com/uploads/favorites/music.png"},
		},
		CurrentLevel: &pb.CitizenLevel{
			Id:    3,
			Name:  "Citizen",
			Slug:  "citizen-baguette",
			Score: 100,
			Image: "http://admin/uploads/levels/citizen.png",
		},
		AchievedLevels: []*pb.CitizenLevel{
			{Id: 1, Name: "Level 1", Slug: "level-1", Score: 10, Image: "http://admin/uploads/l1.png"},
		},
		Avatar: "https://irpsc.com/gb.glb",
	}

	out := buildCitizenProfileHTTPResponse(resp)

	photos, ok := out["profilePhotos"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, photos, 1)

	kyc, ok := out["kyc"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Ali", kyc["fname"])

	assert.Equal(t, "hm-2000001", out["code"])
	assert.Equal(t, float64(42), out["score_percentage_to_next_level"])

	current, ok := out["current_level"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "citizen-baguette", current["slug"])
	assert.Equal(t, "Citizen", current["name"])

	customs, ok := out["customs"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "dev", customs["occupation"])
	_, hasPassions := customs["passions"]
	assert.True(t, hasPassions)

	achieved, ok := out["achieved_levels"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, achieved, 1)
	assert.Equal(t, "level-1", achieved[0]["slug"])
}

func TestBuildCitizenProfileHTTPResponse_OmitsHiddenScore(t *testing.T) {
	resp := &pb.CitizenProfileResponse{
		Code:                       "hm-1",
		Score:                      -1,
		ScorePercentageToNextLevel: 0,
		ProfilePhotos:              []*pb.ProfilePhoto{},
	}

	out := buildCitizenProfileHTTPResponse(resp)
	_, hasScore := out["score"]
	assert.False(t, hasScore)
}

func TestWriteJSONCitizenProfileSingleDataWrap(t *testing.T) {
	resp := &pb.CitizenProfileResponse{
		Code:                       "hm-1",
		Name:                       "User",
		Score:                      10,
		ScorePercentageToNextLevel: 5,
	}

	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, citizenProfileJSONRoundTrip(buildCitizenProfileHTTPResponse(resp)))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "hm-1", data["code"])
	assert.Equal(t, "User", data["name"])
	_, hasNested := data["data"]
	assert.False(t, hasNested)
}
