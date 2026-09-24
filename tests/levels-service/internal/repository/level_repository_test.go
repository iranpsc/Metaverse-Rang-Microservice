package repository_test

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ptime "github.com/yaa110/go-persian-calendar"

	"metarang/levels-service/internal/repository"
)

func TestLevelRepository_FormatImageURL_ViaGetAllLevels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	t.Run("WithAdminPanelURL", func(t *testing.T) {
		repo := repository.NewLevelRepository(db, "https://admin.example.com")
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
			AddRow(1, "Level 1", "level-1", 100, "bg1.jpg", "img1.jpg").
			AddRow(2, "Level 2", "level-2", 200, "", "")
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WillReturnRows(rows)

		levels, err := repo.GetAllLevels(ctx)
		require.NoError(t, err)
		require.Len(t, levels, 2)
		assert.Equal(t, "https://admin.example.com/uploads/img1.jpg", levels[0].ImageUrl)
		assert.Equal(t, "", levels[1].ImageUrl)
	})

	t.Run("WithoutAdminPanelURL", func(t *testing.T) {
		repo := repository.NewLevelRepository(db, "")
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
			AddRow(1, "Level 1", "level-1", 100, "bg1.jpg", "image.jpg")
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WillReturnRows(rows)

		levels, err := repo.GetAllLevels(ctx)
		require.NoError(t, err)
		assert.Equal(t, "/uploads/image.jpg", levels[0].ImageUrl)
	})

	t.Run("AbsoluteURL_Unchanged", func(t *testing.T) {
		repo := repository.NewLevelRepository(db, "https://admin.example.com")
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
			AddRow(1, "Level 1", "level-1", 100, "bg1.jpg", "https://example.com/image.jpg")
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WillReturnRows(rows)

		levels, err := repo.GetAllLevels(ctx)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/image.jpg", levels[0].ImageUrl)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetLevelPrize_JalaliFormat(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "https://admin.example.com")
	ctx := context.Background()

	t.Run("PrizeWithCreatedAt", func(t *testing.T) {
		createdAt := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
		mock.ExpectQuery("SELECT id, level_id, psc, blue, red, yellow, effect, satisfaction, created_at").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "level_id", "psc", "blue", "red", "yellow", "effect", "satisfaction", "created_at"}).
				AddRow(1, 1, 1000, 5, 3, 2, 10, 50.75, createdAt))

		prize, err := repo.GetLevelPrize(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, prize)

		assert.Equal(t, "50.75", prize.Satisfaction)

		pt := ptime.New(createdAt)
		expectedJalali := pt.Format("yyyy/MM/dd HH:mm:ss")
		assert.Equal(t, expectedJalali, prize.CreatedAt)
		assert.Contains(t, prize.CreatedAt, "/")
		assert.Contains(t, prize.CreatedAt, ":")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetLevelGeneralInfo_FileURLs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "https://admin.example.com")
	ctx := context.Background()

	t.Run("GeneralInfoWithFileURLs", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, level_id, score").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "level_id", "score", "rank", "description", "subcategories",
				"persian_font", "english_font", "file_volume", "used_colors", "points", "lines",
				"has_animation", "designer", "model_designer", "creation_date", "png_file", "fbx_file", "gif_file"}).
				AddRow(1, 1, 100, 1, "Description", 2, "Font1", "Font2", 1.5, "Colors", 100, 200, 1,
					"Designer", "Model Designer", "2024-01-01", "png.png", "fbx.fbx", "gif.gif"))

		info, err := repo.GetLevelGeneralInfo(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, "https://admin.example.com/uploads/png.png", info.PngFile)
		assert.Equal(t, "fbx.fbx", info.FbxFile)
		assert.Equal(t, "https://admin.example.com/uploads/gif.gif", info.GifFile)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetLevelPrize_SatisfactionFormatting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "")
	ctx := context.Background()

	testCases := []struct {
		name         string
		satisfaction float64
		expected     string
	}{
		{"TwoDecimalPlaces", 50.75, "50.75"},
		{"OneDecimalPlace", 50.5, "50.50"},
		{"IntegerValue", 50.0, "50.00"},
		{"ManyDecimalPlaces", 50.123456, "50.12"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock.ExpectQuery("SELECT id, level_id, psc, blue, red, yellow, effect, satisfaction, created_at").
				WithArgs(uint64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "level_id", "psc", "blue", "red", "yellow", "effect", "satisfaction", "created_at"}).
					AddRow(1, 1, 1000, 5, 3, 2, 10, tc.satisfaction, nil))

			prize, err := repo.GetLevelPrize(ctx, 1)
			require.NoError(t, err)
			require.NotNil(t, prize)
			assert.Equal(t, tc.expected, prize.Satisfaction)
		})
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetNextLevel(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "https://admin.example.com")
	ctx := context.Background()

	t.Run("Found", func(t *testing.T) {
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WithArgs(int32(100)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
				AddRow(2, "Level 2", "level-2", 200, "bg2.jpg", "img2.jpg"))

		level, err := repo.GetNextLevel(ctx, 100)
		require.NoError(t, err)
		require.NotNil(t, level)
		assert.Equal(t, uint64(2), level.Id)
		assert.Equal(t, "https://admin.example.com/uploads/img2.jpg", level.ImageUrl)
	})

	t.Run("NoNextLevel", func(t *testing.T) {
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WithArgs(int32(9999)).
			WillReturnError(sql.ErrNoRows)

		level, err := repo.GetNextLevel(ctx, 9999)
		require.NoError(t, err)
		assert.Nil(t, level)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetNextLevelForScore(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "")
	ctx := context.Background()

	t.Run("Found", func(t *testing.T) {
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WithArgs(int32(150), uint64(10)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
				AddRow(3, "Level 3", "level-3", 150, "", "uploads/existing.jpg"))

		level, err := repo.GetNextLevelForScore(ctx, 10, 150)
		require.NoError(t, err)
		require.NotNil(t, level)
		assert.Equal(t, "/uploads/existing.jpg", level.ImageUrl)
	})

	t.Run("NoNewLevel", func(t *testing.T) {
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WithArgs(int32(50), uint64(10)).
			WillReturnError(sql.ErrNoRows)

		level, err := repo.GetNextLevelForScore(ctx, 10, 50)
		require.NoError(t, err)
		assert.Nil(t, level)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_AttachLevelToUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "")
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO level_user").
		WithArgs(uint64(5), uint64(2)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.AttachLevelToUser(ctx, 5, 2)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_GetVariableRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "")
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT value FROM system_variables WHERE name = ? LIMIT 1")).
			WithArgs("significant_trade_irr").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("5000"))

		rate, err := repo.GetVariableRate(ctx, "significant_trade_irr")
		require.NoError(t, err)
		assert.Equal(t, 5000.0, rate)
	})

	t.Run("ParseError", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT value FROM system_variables WHERE name = ? LIMIT 1")).
			WithArgs("bad").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("abc"))

		_, err := repo.GetVariableRate(ctx, "bad")
		assert.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLevelRepository_FindBySlug_ImageURLFormatting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewLevelRepository(db, "https://admin.example.com")
	ctx := context.Background()

	t.Run("LevelWithImageURL", func(t *testing.T) {
		mock.ExpectQuery("SELECT l.id, l.name, l.slug, CAST\\(l.score AS UNSIGNED\\) as score").
			WithArgs("level-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "score", "background_image", "image_url"}).
				AddRow(1, "Level 1", "level-1", 100, "bg1.jpg", "img1.jpg"))
		mock.ExpectQuery("SELECT id, level_id, score").
			WithArgs(uint64(1)).
			WillReturnError(sql.ErrNoRows)

		level, err := repo.FindBySlug(ctx, "level-1")
		require.NoError(t, err)
		require.NotNil(t, level)
		assert.Equal(t, "https://admin.example.com/uploads/img1.jpg", level.ImageUrl)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
