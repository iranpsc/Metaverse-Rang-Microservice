package formatter_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/formatter"
	"metarang/dynasty-service/internal/models"
)

func TestFormatDynastyResponse(t *testing.T) {
	now := time.Now()
	d := &models.Dynasty{ID: 1, UserID: 2, FeatureID: 3, CreatedAt: now}
	photo := "p.jpg"

	t.Run("profit increase when stability > 10000", func(t *testing.T) {
		resp := formatter.FormatDynastyResponse(d, 9, 4, 3, "props", "area", "den", 25000, "1403/01/01", &photo, []formatter.AvailableFeature{{ID: 1}})
		require.NotNil(t, resp)
		assert.True(t, resp.UserHasDynasty)
		assert.Equal(t, uint64(1), resp.ID)
		assert.Equal(t, "p.jpg", resp.ProfileImage)
		assert.Equal(t, "1.500", resp.DynastyFeature.FeatureProfitIncrease)
		assert.Len(t, resp.Features, 1)
	})

	t.Run("zero profit when stability low", func(t *testing.T) {
		resp := formatter.FormatDynastyResponse(d, 1, 0, 3, "p", "a", "d", 5000, "x", nil, nil)
		assert.Equal(t, "0", resp.DynastyFeature.FeatureProfitIncrease)
		assert.Equal(t, "", resp.ProfileImage)
	})
}

func TestFormatSentAndReceivedRequest(t *testing.T) {
	msg := "hello"
	req := &models.JoinRequest{
		ID: 1, FromUser: 2, ToUser: 3, Status: 0, Relationship: "brother",
		Message: &msg, CreatedAt: time.Now(),
	}
	prize := &models.DynastyPrize{
		Satisfaction: 1.5, PSC: 10, IntroductionProfitIncrease: 0.1,
		AccumulatedCapitalReserve: 0.2, DataStorage: 0.3,
	}
	user := formatter.UserBasic{ID: 3, Code: "C", Name: "N"}

	sent := formatter.FormatSentRequest(req, user, prize)
	require.NotNil(t, sent)
	assert.Equal(t, uint64(1), sent.ID)
	assert.Equal(t, "hello", sent.Message)
	require.NotNil(t, sent.Prize)
	assert.Equal(t, 10, sent.Prize.PSC)

	sentNoPrize := formatter.FormatSentRequest(req, user, nil)
	assert.Nil(t, sentNoPrize.Prize)

	recv := formatter.FormatReceivedRequest(req, user)
	assert.Equal(t, "hello", recv.Message)

	req.Message = nil
	recvEmpty := formatter.FormatReceivedRequest(req, user)
	assert.Equal(t, "", recvEmpty.Message)
}

func TestFormatFamilyMemberAndUserSearch(t *testing.T) {
	member := &models.FamilyMember{ID: 5, Relationship: "sister"}
	user := formatter.UserWithLevel{ID: 1, Code: "c", Name: "n", Level: "1"}
	got := formatter.FormatFamilyMember(member, user)
	assert.Equal(t, uint64(5), got.ID)
	assert.Equal(t, "sister", got.Relationship)

	search := formatter.FormatUserSearchResponse([]string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, search["date"])
}
