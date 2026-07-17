package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"metarang/social-service/internal/repository"
	"metarang/social-service/internal/testutil"
)

func TestChallengeRepository_GetTotalParticipantsCount(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewChallengeRepository(db)
	n, err := repo.GetTotalParticipantsCount(context.Background())
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		t.Skip("questions / user_question_answers tables not present:", err)
	}
	require.NoError(t, err)
	require.GreaterOrEqual(t, int(n), 0)
}

func TestChallengeRepository_GetSystemVariable_DefaultOnMissing(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewChallengeRepository(db)
	v, err := repo.GetSystemVariable(context.Background(), "challenge_display_ad_interval________________unknown_slug______________")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
			t.Skip("system_variables table not present:", err)
		}
		require.NoError(t, err)
		return
	}
	require.Equal(t, 15.0, v)
}
