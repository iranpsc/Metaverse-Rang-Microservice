package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

func TestSettingsAndUserRepoGaps_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	now := time.Now()
	settingsRepo := repository.NewSettingsRepository(db)
	userRepo := repository.NewUserRepository(db, "https://gw")

	mock.ExpectQuery("FROM settings").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	s, err := settingsRepo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), s.UserID)
	require.Equal(t, int32(55), s.AutomaticLogout)

	cols := []string{
		"id", "user_id", "status", "level", "details", "checkout_days_count", "automatic_logout",
		"privacy", "notifications", "created_at", "updated_at",
	}
	mock.ExpectQuery("FROM settings").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(9), uint64(2), true, true, true, uint32(3), int32(55),
			`{"score":1}`, `{"announcements_sms":true}`, now, now,
		))
	s, err = settingsRepo.FindByUserID(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, uint64(9), s.ID)
	require.Equal(t, 1, s.Privacy["score"])

	mock.ExpectQuery("FROM settings").WithArgs(uint64(4)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(12), uint64(4), true, true, true, uint32(3), int32(55),
			`{"score":true,"phone":false,"name":true}`, `{"announcements_sms":true}`, now, now,
		))
	s, err = settingsRepo.FindByUserID(ctx, 4)
	require.NoError(t, err)
	require.Equal(t, 1, s.Privacy["score"])
	require.Equal(t, 0, s.Privacy["phone"])
	require.Equal(t, 1, s.Privacy["name"])
	require.Equal(t, 1, s.Privacy["level"], "boolean JSON must not wipe unrelated default keys")

	mock.ExpectQuery("FROM settings").WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(9), uint64(2), true, true, true, uint32(3), int32(55),
			`{bad`, `not-json`, now, now,
		))
	s, err = settingsRepo.FindByID(ctx, 9)
	require.NoError(t, err)
	require.NotNil(t, s.Privacy)
	require.NotNil(t, s.Notifications)

	mock.ExpectQuery("FROM settings").WithArgs(uint64(3)).WillReturnError(sql.ErrNoRows)
	s, err = userRepo.GetSettings(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), s.UserID)

	mock.ExpectQuery("SELECT COUNT").WithArgs("App\\User", uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int32(4)))
	n, err := userRepo.GetUnreadNotificationsCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int32(4), n)

	mock.ExpectExec("INSERT INTO settings").WillReturnResult(sqlmock.NewResult(11, 1))
	require.NoError(t, userRepo.CreateSettings(ctx, &models.Settings{
		UserID: 1, Status: true, Level: true, Details: true,
		CheckoutDaysCount: 3, AutomaticLogout: 55,
		Privacy: models.DefaultPrivacySettings(), Notifications: models.DefaultNotificationSettings(),
	}))

	// Referrals with a row exercises KYC/photo lookups
	citizen := repository.NewCitizenRepository(db)
	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(1), "%ali%", "%ali%").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("FROM users u").
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "created_at"}).
			AddRow(uint64(2), "hm-2", "ali", now))
	mock.ExpectQuery("FROM kycs").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"fname", "lname"}).AddRow("A", "B"))
	mock.ExpectQuery("FROM images").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("/p.jpg"))
	refs, meta, err := citizen.GetCitizenReferrals(ctx, 1, "ali", 1, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, "A B", refs[0].Name)
	require.Equal(t, "/p.jpg", refs[0].Image)
	require.Equal(t, int32(1), meta.CurrentPage)
}
