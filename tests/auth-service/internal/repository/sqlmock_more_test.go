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

func TestKYCRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewKYCRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("FROM kycs").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	kyc, err := repo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, kyc)

	mock.ExpectExec("INSERT INTO kycs").WillReturnResult(sqlmock.NewResult(3, 1))
	k := &models.KYC{UserID: 1, Fname: "a", Lname: "b", MelliCode: "001", Status: 0, Birthdate: sql.NullTime{Time: now, Valid: true}}
	require.NoError(t, repo.Create(ctx, k))
	require.Equal(t, uint64(3), k.ID)

	mock.ExpectExec("UPDATE kycs").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Update(ctx, k))

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	ok, err := repo.CheckUniqueMelliCode(ctx, "001", 1)
	require.NoError(t, err)
	require.True(t, ok)

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	ok, err = repo.CheckVerifyTextExists(ctx, 9)
	require.NoError(t, err)
	require.True(t, ok)

	mock.ExpectExec("INSERT INTO bank_accounts").WillReturnResult(sqlmock.NewResult(8, 1))
	ba := &models.BankAccount{BankableType: "App\\Models\\User", BankableID: 1, BankName: "Tejarat", ShabaNum: "s", CardNum: "c"}
	require.NoError(t, repo.CreateBankAccount(ctx, ba))

	mock.ExpectQuery("FROM bank_accounts").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "bankable_type", "bankable_id", "bank_name", "shaba_num", "card_num", "status", "errors", "created_at", "updated_at"}).
			AddRow(uint64(8), "App\\Models\\User", uint64(1), "Tejarat", "s", "c", int32(0), nil, now, now))
	list, err := repo.FindBankAccountsByUserID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("FROM bank_accounts").WithArgs(uint64(8)).WillReturnError(sql.ErrNoRows)
	got, err := repo.FindBankAccountByID(ctx, 8)
	require.NoError(t, err)
	require.Nil(t, got)

	mock.ExpectExec("UPDATE bank_accounts").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateBankAccount(ctx, ba))
	mock.ExpectExec("DELETE FROM bank_accounts").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.DeleteBankAccount(ctx, 8))

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	ok, err = repo.CheckUniqueShaba(ctx, "s", 0)
	require.NoError(t, err)
	require.True(t, ok)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	ok, err = repo.CheckUniqueCard(ctx, "c", 0)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestActivityRepository_GetLatestActivityScansLegacyVarcharTimes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewActivityRepository(db)
	start := "2026-10-31 10:20:30"
	end := "2026-10-31 11:20:30"
	mock.ExpectQuery("FROM user_activities").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "start", "end", "total", "ip", "created_at", "updated_at"}).
			AddRow(uint64(9), uint64(1), []byte(start), []byte(end), int32(60), "127.0.0.1", time.Now(), time.Now()))

	activity, err := repo.GetLatestActivity(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, start, activity.Start.Format("2006-01-02 15:04:05"))
	require.True(t, activity.End.Valid)
	require.Equal(t, end, activity.End.Time.Format("2006-01-02 15:04:05"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPersonalInfoRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewPersonalInfoRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	info, err := repo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, info)

	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO personal_infos").WillReturnResult(sqlmock.NewResult(4, 1))
	pi := &models.PersonalInfo{UserID: 1, Passions: models.DefaultPassions()}
	require.NoError(t, repo.Upsert(ctx, pi))

	cols := []string{"id", "user_id", "occupation", "education", "memory", "loved_city", "loved_country",
		"loved_language", "problem_solving", "prediction", "about", "passions"}
	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(uint64(4), uint64(1), "o", nil, nil, nil, nil, nil, nil, nil, nil, `{"music":true}`))
	mock.ExpectExec("UPDATE personal_infos").WillReturnResult(sqlmock.NewResult(0, 1))
	pi.ID = 4
	require.NoError(t, repo.Upsert(ctx, pi))
}

func TestProfilePhotoRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewProfilePhotoRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("INSERT INTO images").WillReturnResult(sqlmock.NewResult(11, 1))
	img, err := repo.Create(ctx, 1, "/u.jpg")
	require.NoError(t, err)
	require.Equal(t, uint64(11), img.ID)

	mock.ExpectQuery("FROM images").
		WillReturnRows(sqlmock.NewRows([]string{"id", "imageable_type", "imageable_id", "url", "created_at", "updated_at"}).
			AddRow(uint64(11), "App\\Models\\User", uint64(1), "/u.jpg", now, now))
	list, err := repo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectQuery("FROM images").WithArgs(uint64(11)).WillReturnError(sql.ErrNoRows)
	got, err := repo.FindByID(ctx, 11)
	require.NoError(t, err)
	require.Nil(t, got)

	mock.ExpectExec("DELETE FROM images").WithArgs(uint64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(ctx, 11))

	mock.ExpectExec("DELETE FROM images").WithArgs(uint64(99)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.Delete(ctx, 99))

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	ok, err := repo.CheckOwnership(ctx, 11, 1)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestSettingsRepository_SQLMockDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewSettingsRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("FROM settings").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	s, err := repo.FindByUserID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int32(55), s.AutomaticLogout)
	require.NotNil(t, s.Privacy)

	mock.ExpectExec("INSERT INTO settings").WillReturnResult(sqlmock.NewResult(2, 1))
	require.NoError(t, repo.Create(ctx, s))

	mock.ExpectExec("UPDATE settings").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Update(ctx, s))

	mock.ExpectQuery("FROM settings").WithArgs(uint64(99)).WillReturnError(sql.ErrNoRows)
	got, err := repo.FindByID(ctx, 99)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestActivityRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewActivityRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("INSERT INTO user_events").WillReturnResult(sqlmock.NewResult(1, 1))
	ev := &models.UserEvent{UserID: 1, Event: "login", IP: "1.1.1.1", Device: "web"}
	require.NoError(t, repo.CreateUserEvent(ctx, ev))

	mock.ExpectQuery("FROM user_events").WillReturnRows(sqlmock.NewRows([]string{
		"id", "user_id", "event", "ip", "device", "status", "created_at", "updated_at",
	}).AddRow(uint64(1), uint64(1), "login", "1.1.1.1", "web", int32(0), now, now))
	got, err := repo.GetUserEventByID(ctx, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	mock.ExpectQuery("FROM user_events").WillReturnError(sql.ErrNoRows)
	got, err = repo.GetUserEventByID(ctx, 1, 9)
	require.NoError(t, err)
	require.Nil(t, got)

	mock.ExpectQuery("FROM user_events").WillReturnRows(sqlmock.NewRows([]string{
		"id", "user_id", "event", "ip", "device", "status", "created_at", "updated_at",
	}))
	list, err := repo.GetUserEventsByUserID(ctx, 1, 1)
	require.NoError(t, err)
	require.Empty(t, list)

	mock.ExpectExec("INSERT INTO user_event_reports").WillReturnResult(sqlmock.NewResult(2, 1))
	rep := &models.UserEventReport{UserEventID: 1, EventDescription: "d"}
	require.NoError(t, repo.CreateUserEventReport(ctx, rep))

	mock.ExpectQuery("FROM user_event_reports").WillReturnError(sql.ErrNoRows)
	r, err := repo.GetUserEventReportByEventID(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, r)

	mock.ExpectExec("INSERT INTO user_activities").WillReturnResult(sqlmock.NewResult(3, 1))
	act := &models.UserActivity{UserID: 1, Start: now, IP: "1.1.1.1"}
	require.NoError(t, repo.CreateActivity(ctx, act))

	mock.ExpectQuery("FROM user_activities").WillReturnError(sql.ErrNoRows)
	la, err := repo.GetLatestActivity(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, la)

	mock.ExpectQuery("COALESCE\\(SUM").WillReturnRows(sqlmock.NewRows([]string{"t"}).AddRow(int32(10)))
	mins, err := repo.GetTotalActivityMinutes(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int32(10), mins)

	mock.ExpectQuery("FROM user_logs").WillReturnError(sql.ErrNoRows)
	log, err := repo.GetUserLog(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, log)

	mock.ExpectExec("INSERT INTO user_logs").WillReturnResult(sqlmock.NewResult(4, 1))
	require.NoError(t, repo.CreateUserLog(ctx, &models.UserLog{UserID: 1}))
}
