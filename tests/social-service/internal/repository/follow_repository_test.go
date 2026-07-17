package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"metarang/social-service/internal/repository"
	"metarang/social-service/internal/testutil"
)

func TestFollowRepository_CreateExistsDelete(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewFollowRepository(db)
	a := uint64(time.Now().UnixNano()%900000 + 800001)
	b := uint64(time.Now().UnixNano()%900000 + 810001)

	err := repo.Create(context.Background(), a, b)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		t.Skip("follows table not present:", err)
	}
	require.NoError(t, err)

	ok, err := repo.Exists(context.Background(), a, b)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, repo.Delete(context.Background(), a, b))

	ok2, err := repo.Exists(context.Background(), a, b)
	require.NoError(t, err)
	require.False(t, ok2)
}

func TestFollowRepository_GetFollowersEmptyUser(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewFollowRepository(db)
	ids, err := repo.GetFollowers(context.Background(), 999999999999900001)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		t.Skip("follows table not present:", err)
	}
	require.NoError(t, err)
	require.NotNil(t, ids)
}
