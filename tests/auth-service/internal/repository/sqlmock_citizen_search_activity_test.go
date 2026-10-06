package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

func TestCitizenRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewCitizenRepository(db)
	ctx := context.Background()
	now := time.Now()

	privacy, _ := json.Marshal(map[string]interface{}{"score": true, "name": 1, "x": "true", "y": "0", "z": 2.0})
	mock.ExpectQuery("FROM users").WithArgs("hm-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(1)))
	mock.ExpectQuery("FROM settings").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"privacy"}).AddRow(string(privacy)))
	info, err := repo.GetCitizenUserInfoByCode(ctx, "hm-1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), info.UserID)
	require.Equal(t, int32(1), info.Privacy["score"])
	require.Equal(t, int32(1), info.Privacy["name"])
	require.Equal(t, int32(0), info.Privacy["y"])
	require.Equal(t, int32(1), info.Privacy["level"], "defaults must fill keys omitted from stored JSON")

	mock.ExpectQuery("FROM users").WithArgs("missing").WillReturnError(sql.ErrNoRows)
	info, err = repo.GetCitizenUserInfoByCode(ctx, "missing")
	require.NoError(t, err)
	require.Nil(t, info)

	mock.ExpectQuery("FROM users").WithArgs("hm-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "phone", "code", "score", "email_verified_at"}).
			AddRow(uint64(1), "n", "a@x.com", "09", "hm-1", int32(10), now))
	mock.ExpectQuery("FROM kycs").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM settings").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "privacy"}).AddRow(uint64(1), uint64(1), `{"score":true}`))
	mock.ExpectQuery("FROM images").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(uint64(1), "/p.jpg"))
	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	profile, err := repo.GetCitizenByCode(ctx, "hm-1")
	require.NoError(t, err)
	require.Equal(t, "hm-1", profile.Code)
	require.Len(t, profile.ProfilePhotos, 1)
	require.True(t, profile.Privacy["score"])
	require.True(t, profile.Privacy["name"], "unrelated public fields must remain visible")

	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery("FROM users u").WithArgs(uint64(1), 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "created_at"}))
	refs, meta, err := repo.GetCitizenReferrals(ctx, 1, "", 1, 10)
	require.NoError(t, err)
	require.Empty(t, refs)
	require.Equal(t, int32(1), meta.CurrentPage)

	mock.ExpectQuery("FROM referral_order_histories").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "created_at"}).AddRow(uint64(1), int64(100), now))
	orders, err := repo.GetCitizenReferralOrders(ctx, 2)
	require.NoError(t, err)
	require.Len(t, orders, 1)

	expectCitizenReferralChartQueries(mock, 0, 0, nil, nil)
	chart, err := repo.GetCitizenReferralChartData(ctx, 1, "yearly")
	require.NoError(t, err)
	require.Equal(t, "0", chart.TotalReferralsCount)
	require.Equal(t, "0", chart.TotalReferralOrdersAmount)
	require.Empty(t, chart.ChartData)

	expectCitizenReferralChartQueries(mock, 1, 50,
		[]chartBucketRow{{label: "2024/01", count: 1}},
		[]chartBucketRow{{label: "2024/01", amount: 50}},
	)
	chart, err = repo.GetCitizenReferralChartData(ctx, 1, "yearly")
	require.NoError(t, err)
	require.Equal(t, "1", chart.TotalReferralsCount)
	require.Equal(t, "50", chart.TotalReferralOrdersAmount)
	require.Regexp(t, `^\d{4}/\d{2}$`, chart.ChartData[0].Label)
	require.NotEqual(t, "2024/01", chart.ChartData[0].Label)

	cur, prev, err := repo.GetCitizenLevels(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, cur)
	require.Nil(t, prev)

	mock.ExpectQuery("FROM users").WithArgs("hm-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "phone", "code", "score", "email_verified_at"}).
			AddRow(uint64(2), "n", "a@x.com", "09", "hm-2", int32(10), now))
	mock.ExpectQuery("FROM kycs").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "fname", "lname", "melli_code", "status", "birthdate"}).
			AddRow(uint64(1), uint64(2), "A", "B", "001", int32(1), now))
	mock.ExpectQuery("FROM settings").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "privacy"}).AddRow(uint64(1), uint64(2), `{"score":true}`))
	mock.ExpectQuery("FROM images").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}))
	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "occupation", "education", "memory", "loved_city", "loved_country",
			"loved_language", "problem_solving", "prediction", "about", "passions",
		}).AddRow(uint64(1), uint64(2), "o", "e", "m", "c", "ir", "fa", "ps", "pr", "a", `{"music":true}`))
	profile, err = repo.GetCitizenByCode(ctx, "hm-2")
	require.NoError(t, err)
	require.NotNil(t, profile.KYC)
	require.NotNil(t, profile.PersonalInfo)
	require.True(t, profile.PersonalInfo.Passions["music"])
}

func TestCitizenRepository_GetCitizenByCode_IntegerPrivacyJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewCitizenRepository(db)
	ctx := context.Background()
	now := time.Now()

	// After POST /api/privacy the column is rewritten as 0|1 integers.
	integerPrivacy := `{"phone":1,"name":1,"email":0,"level":1,"score":1}`
	mock.ExpectQuery("FROM users").WithArgs("hm-9").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "phone", "code", "score", "email_verified_at"}).
			AddRow(uint64(9), "n", "a@x.com", "09", "hm-9", int32(10), now))
	mock.ExpectQuery("FROM kycs").WithArgs(uint64(9)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM settings").WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "privacy"}).AddRow(uint64(1), uint64(9), integerPrivacy))
	mock.ExpectQuery("FROM images").WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}))
	mock.ExpectQuery("FROM personal_infos").WithArgs(uint64(9)).WillReturnError(sql.ErrNoRows)

	profile, err := repo.GetCitizenByCode(ctx, "hm-9")
	require.NoError(t, err)
	require.NotNil(t, profile)
	require.True(t, profile.Privacy["name"], "name must stay visible after an unrelated privacy update")
	require.True(t, profile.Privacy["level"])
	require.True(t, profile.Privacy["score"])
	require.True(t, profile.Privacy["occupation"], "missing keys must keep public defaults")
	require.True(t, profile.Privacy["phone"], "updated key must apply")
	require.False(t, profile.Privacy["email"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSearchRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewSearchRepository(db)
	ctx := context.Background()
	now := time.Now()

	empty, err := repo.SearchUsers(ctx, "   ")
	require.NoError(t, err)
	require.Empty(t, empty)

	cols := []string{
		"id", "name", "email", "phone", "code", "referrer_id", "score",
		"last_seen", "created_at", "updated_at", "email_verified_at", "phone_verified_at",
		"kyc_id", "user_id", "fname", "lname", "melli_code", "melli_card",
		"video", "verify_text_id", "province", "gender", "status", "birthdate",
		"errors", "kyc_created_at", "kyc_updated_at",
	}
	mock.ExpectQuery("FROM users u").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			uint64(1), "n", "a@x.com", "09", "hm-1", nil, int32(5),
			now, now, now, now, now,
			nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil,
			nil, nil, nil,
		))
	mock.ExpectQuery("FROM images").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "imageable_type", "imageable_id", "url", "created_at", "updated_at"}))
	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int32(2)))
	mock.ExpectQuery("FROM level_user").WithArgs(uint64(1)).WillReturnError(sql.ErrNoRows)
	users, err := repo.SearchUsers(ctx, "n")
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, int32(2), users[0].Followers)

	mock.ExpectQuery("FROM isic_codes").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code"}).AddRow(uint64(1), "code", int64(11)))
	isic, err := repo.SearchIsicCodes(ctx, "co")
	require.NoError(t, err)
	require.Len(t, isic, 1)
	require.Equal(t, uint64(11), isic[0].Code)
}

func TestActivityRepository_MoreSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewActivityRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("UPDATE user_event_reports").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateUserEventReportStatus(ctx, 1, 2))
	mock.ExpectExec("UPDATE user_event_reports").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.CloseUserEventReport(ctx, 1))

	mock.ExpectExec("INSERT INTO user_event_report_responses").WillReturnResult(sqlmock.NewResult(3, 1))
	resp := &models.UserEventReportResponse{UserEventReportID: 1, Response: "r", ResponserName: "n"}
	require.NoError(t, repo.CreateUserEventReportResponse(ctx, resp))

	mock.ExpectQuery("FROM user_event_report_responses").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_event_report_id", "response", "responser_name", "created_at", "updated_at"}).
			AddRow(uint64(3), uint64(1), "r", "n", now, now))
	list, err := repo.GetUserEventReportResponses(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mock.ExpectExec("UPDATE user_activities").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateActivity(ctx, &models.UserActivity{ID: 1, Total: 5, End: sql.NullTime{Time: now, Valid: true}}))

	mock.ExpectExec("UPDATE user_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateUserLog(ctx, &models.UserLog{ID: 1, UserID: 1}))

	mock.ExpectExec("UPDATE user_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.IncrementLogField(ctx, 1, "score", 1))
}

func TestCitizenRepository_PersianReferralSearchUsesKYCName(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
		func(expectedSQL, actualSQL string) error {
			if strings.Contains(actualSQL, "LOWER(") {
				return fmt.Errorf("LOWER() drops Persian LIKE matches: %s", actualSQL)
			}
			expect, err := regexp.Compile(expectedSQL)
			if err != nil {
				return err
			}
			if !expect.MatchString(actualSQL) {
				return fmt.Errorf("could not match actual sql: %q with expected regexp %q", actualSQL, expectedSQL)
			}
			return nil
		},
	)))
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewCitizenRepository(db)
	ctx := context.Background()
	now := time.Now()
	const pattern = "%عباس%"

	mock.ExpectQuery("k.fname").
		WithArgs(uint64(7), pattern, pattern, pattern, pattern, pattern).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery("k.fname").
		WithArgs(uint64(7), pattern, pattern, pattern, pattern, pattern, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "created_at"}).
			AddRow(uint64(9), "hm-9", "john", now))
	mock.ExpectQuery("FROM kycs").WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"fname", "lname"}).AddRow("عباس", "رضایی"))
	mock.ExpectQuery("FROM images").WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("/a.jpg"))

	refs, meta, err := repo.GetCitizenReferrals(ctx, 7, "  عباس  ", 1, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, "عباس رضایی", refs[0].Name)
	require.Equal(t, int32(1), meta.CurrentPage)
	require.NoError(t, mock.ExpectationsWereMet())
}
