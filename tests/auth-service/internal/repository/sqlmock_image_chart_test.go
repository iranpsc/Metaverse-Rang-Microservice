package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/repository"
)

func TestUserRepo_FormatImageURLViaLevels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	repo := repository.NewUserRepository(db, "https://admin")
	mock.ExpectQuery("FROM level_user").WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "image_url", "gem_png_file"}).
			AddRow(uint64(2), "L", "l", int32(10), "lvl.png", "gem.png"))
	lvl, err := repo.GetUserLatestLevel(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "https://admin/uploads/lvl.png", lvl.Image)
	require.Equal(t, "https://admin/uploads/gem.png", lvl.GemPngFile)

	repo2 := repository.NewUserRepository(db, "")
	mock.ExpectQuery("FROM levels").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "image_url", "gem_png_file"}).
			AddRow(uint64(1), "P", "p", int32(0), "https://cdn/p.png", "https://cdn/gem.png"))
	levels, err := repo2.GetLevelsBelowScore(ctx, 10)
	require.NoError(t, err)
	require.Len(t, levels, 1)
	require.Equal(t, "https://cdn/p.png", levels[0].Image)
	require.Equal(t, "https://cdn/gem.png", levels[0].GemPngFile)
}

func TestUserRepo_GetUserLatestLevel_SelectsNewestImage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewUserRepository(db, "https://admin")
	mock.ExpectQuery("(?s)App.*Models.*Level.*Level.*ORDER BY i\\.created_at DESC, i\\.id DESC").
		WithArgs(uint64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "image_url", "gem_png_file"}).
			AddRow(uint64(1), "Citizen", "1", int32(10), "current.png", "gem.png"))

	level, err := repo.GetUserLatestLevel(context.Background(), 8)
	require.NoError(t, err)
	require.NotNil(t, level)
	require.Equal(t, "https://admin/uploads/current.png", level.Image)
	require.NoError(t, mock.ExpectationsWereMet())
}

type chartBucketRow struct {
	label  string
	count  int32
	amount int64
}

func expectCitizenReferralChartQueries(mock sqlmock.Sqlmock, totalCount int, totalAmount int64, userBuckets, orderBuckets []chartBucketRow) {
	mock.ExpectQuery(`COUNT\(\*\)\s+FROM users u`).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(totalCount))
	mock.ExpectQuery(`SUM\(roh\.amount\)`).
		WillReturnRows(sqlmock.NewRows([]string{"total_amount"}).AddRow(totalAmount))

	userRows := sqlmock.NewRows([]string{"label", "count"})
	for _, bucket := range userBuckets {
		userRows.AddRow(bucket.label, bucket.count)
	}
	mock.ExpectQuery(`DATE_FORMAT\(u\.created_at`).WillReturnRows(userRows)

	orderRows := sqlmock.NewRows([]string{"label", "total_amount"})
	for _, bucket := range orderBuckets {
		orderRows.AddRow(bucket.label, bucket.amount)
	}
	mock.ExpectQuery(`DATE_FORMAT\(roh\.created_at`).WillReturnRows(orderRows)
}

func TestCitizenReferralChartRanges_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewCitizenRepository(db)
	ctx := context.Background()

	for _, rangeType := range []string{"daily", "weekly", "monthly", "yearly"} {
		expectCitizenReferralChartQueries(mock, 1, 10,
			[]chartBucketRow{{label: "lbl", count: 1}},
			[]chartBucketRow{{label: "lbl", amount: 10}},
		)
		chart, err := repo.GetCitizenReferralChartData(ctx, 1, rangeType)
		require.NoError(t, err)
		require.Equal(t, "1", chart.TotalReferralsCount)
		require.Equal(t, "10", chart.TotalReferralOrdersAmount)
		require.Equal(t, "lbl", chart.ChartData[0].Label)
		require.Equal(t, int32(1), chart.ChartData[0].Count)
		require.Equal(t, int64(10), chart.ChartData[0].TotalAmount)
	}

	expectCitizenReferralChartQueries(mock, 1, 21111,
		[]chartBucketRow{{label: "2023/01", count: 1}},
		[]chartBucketRow{{label: "2023/01", amount: 21111}},
	)
	chart, err := repo.GetCitizenReferralChartData(ctx, 1, "yearly")
	require.NoError(t, err)
	require.Equal(t, "1", chart.TotalReferralsCount)
	require.Equal(t, "21111", chart.TotalReferralOrdersAmount)
	require.Regexp(t, `^\d{4}/\d{2}$`, chart.ChartData[0].Label)
	require.NotEqual(t, "2023/01", chart.ChartData[0].Label)

	expectCitizenReferralChartQueries(mock, 1, 5,
		[]chartBucketRow{{label: "2023/01/15", count: 1}},
		[]chartBucketRow{{label: "2023/01/15", amount: 5}},
	)
	chart, err = repo.GetCitizenReferralChartData(ctx, 1, "daily")
	require.NoError(t, err)
	require.Regexp(t, `^\d{4}/\d{2}/\d{2}$`, chart.ChartData[0].Label)
	require.NotEqual(t, "2023/01/15", chart.ChartData[0].Label)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(25))
	mock.ExpectQuery("FROM users u").
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "created_at"}).
			AddRow(uint64(2), "hm-2", "n", time.Now()))
	mock.ExpectQuery("FROM kycs").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM images").WillReturnError(sql.ErrNoRows)
	refs, meta, err := repo.GetCitizenReferrals(ctx, 1, "n", 2, 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Contains(t, meta.NextPageURL, "page=3")
	require.Contains(t, meta.PrevPageURL, "page=1")
}

func TestCitizenReferralChartData_CountsAllReferralsNotJustThoseWithOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewCitizenRepository(db)
	ctx := context.Background()

	// 7 referred users, only one of whom has an order of 3333 — the same mismatch
	// seen between GET /referrals and GET /referrals/chart?range=yearly.
	expectCitizenReferralChartQueries(mock, 7, 3333,
		[]chartBucketRow{
			{label: "2022/12", count: 6},
			{label: "2023/01", count: 1},
		},
		[]chartBucketRow{{label: "2023/01", amount: 3333}},
	)

	chart, err := repo.GetCitizenReferralChartData(ctx, 1, "yearly")
	require.NoError(t, err)
	require.Equal(t, "7", chart.TotalReferralsCount)
	require.Equal(t, "3333", chart.TotalReferralOrdersAmount)
	require.Len(t, chart.ChartData, 2)

	var totalBucketCount int32
	var totalBucketAmount int64
	for _, point := range chart.ChartData {
		totalBucketCount += point.Count
		totalBucketAmount += point.TotalAmount
		require.Regexp(t, `^\d{4}/\d{2}$`, point.Label)
	}
	require.Equal(t, int32(7), totalBucketCount)
	require.Equal(t, int64(3333), totalBucketAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}
