package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
)

type stubHelperForGetMe struct{}

func (stubHelperForGetMe) GetHourlyProfitTimePercentage(context.Context, uint64) (float64, error) {
	return 11.5, nil
}
func (stubHelperForGetMe) GetScorePercentageToNextLevel(context.Context, uint64, int32) (float64, error) {
	return 22.5, nil
}
func (stubHelperForGetMe) GetUserLevel(context.Context, uint64) (*service.LevelInfo, error) {
	return &service.LevelInfo{ID: 3, Title: "Gold", Description: "d", Score: 50, Slug: "gold"}, nil
}
func (stubHelperForGetMe) GetUserWallet(context.Context, uint64) (*service.WalletInfo, error) {
	return nil, nil
}
func (stubHelperForGetMe) CreateWallet(context.Context, uint64) error        { return nil }
func (stubHelperForGetMe) CreateUserVariables(context.Context, uint64) error { return nil }
func (stubHelperForGetMe) Close() error                                      { return nil }

func TestGetMe_RichPaths(t *testing.T) {
	ctx := context.Background()
	users := map[uint64]*models.User{
		1: {
			ID: 1, Name: "Old", Email: "a@x.com", Code: "hm-1", Score: 40,
			AccessToken:   sql.NullString{String: "at", Valid: true},
			WalletAddress: sql.NullString{String: "0xabc", Valid: true},
		},
	}
	userRepo := newFakeUserRepository(users)
	userRepo.getSettingsFunc = func(_ context.Context, userID uint64) (*models.Settings, error) {
		return &models.Settings{UserID: userID, AutomaticLogout: 45}, nil
	}
	userRepo.getKYCFunc = func(context.Context, uint64) (*models.KYC, error) {
		return &models.KYC{
			UserID: 1, Fname: "Ali", Lname: "Karimi", Status: 1,
			Birthdate: sql.NullTime{Time: time.Date(1990, 2, 3, 0, 0, 0, 0, time.UTC), Valid: true},
		}, nil
	}
	userRepo.getUnreadNotificationsCountFunc = func(context.Context, uint64) (int32, error) { return 7, nil }
	userRepo.getLatestProfilePhotoURLFunc = func(context.Context, uint64) (string, error) {
		return "/uploads/p.jpg", nil
	}

	tokenRepo := newFakeTokenRepository()
	tokenRepo.validateTokenFunc = func(_ context.Context, token string) (*models.User, error) {
		return users[1], nil
	}
	tokenRepo.walletLogin["tok"] = true

	svc := service.NewAuthService(
		userRepo, tokenRepo, newFakeCacheRepository(),
		nil, stubHelperForGetMe{}, nil,
		"", "", "", "", "",
	)

	details, err := svc.GetMe(ctx, "tok")
	require.NoError(t, err)
	require.Equal(t, "Ali Karimi", details.Name)
	require.True(t, details.HasWallet)
	require.Equal(t, "0xabc", details.WalletAddress)
	require.True(t, details.WalletLogin)
	require.Equal(t, "at", details.AccessToken)
	require.True(t, details.VerifiedKYC)
	require.NotEmpty(t, details.Birthdate)
	require.Equal(t, "/uploads/p.jpg", details.Image)
	require.Equal(t, int32(7), details.UnreadNotificationsCount)
	require.NotNil(t, details.Level)
	require.Equal(t, 22.5, details.ScorePercentageToNextLevel)
	require.Equal(t, 11.5, details.HourlyProfitTimePercentage)
}
