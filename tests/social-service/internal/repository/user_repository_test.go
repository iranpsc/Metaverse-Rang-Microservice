package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"metarang/social-service/internal/repository"
	"metarang/social-service/internal/testutil"
)

func TestUserRepository_GetUserBasicInfo_Missing(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	info, err := repo.GetUserBasicInfo(context.Background(), 9999999999999999001)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		t.Skip("users table not present:", err)
	}
	require.NoError(t, err)
	require.Nil(t, info)
}

func TestUserRepository_IsUserOnline_InvalidUser(t *testing.T) {
	db := testutil.OpenMySQLOrSkip(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	on, err := repo.IsUserOnline(context.Background(), 9999999999999999002)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		t.Skip("users table not present:", err)
	}
	require.NoError(t, err)
	require.False(t, on)
}
