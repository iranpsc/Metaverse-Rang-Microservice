package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/service"
	authpb "metarang/shared/pb/auth"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/jalali"
)

type stubKYCPort struct {
	resp  *authpb.KYCResponse
	err   error
	calls []uint64
}

func (s *stubKYCPort) GetKYC(_ context.Context, userID uint64) (*authpb.KYCResponse, error) {
	s.calls = append(s.calls, userID)
	return s.resp, s.err
}

type stubLevelsPort struct {
	resp     *levelspb.UserLevelResponse
	err      error
	gems     map[uint64]*levelspb.LevelGem
	gemErr   error
	calls    []uint64
	gemCalls []uint64
}

func (s *stubLevelsPort) GetUserLevel(_ context.Context, userID uint64) (*levelspb.UserLevelResponse, error) {
	s.calls = append(s.calls, userID)
	return s.resp, s.err
}

func (s *stubLevelsPort) GetLevelGem(_ context.Context, levelID uint64) (*levelspb.LevelGem, error) {
	s.gemCalls = append(s.gemCalls, levelID)
	if s.gemErr != nil {
		return nil, s.gemErr
	}
	if s.gems == nil {
		return nil, nil
	}
	return s.gems[levelID], nil
}

func TestUserSearchService_SearchUsers_EnrichesVerifiedAgeAndLevels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	birth := time.Now().UTC().AddDate(-25, 0, -1)
	jalaliBirth := jalali.CarbonToJalali(birth)

	kyc := &stubKYCPort{
		resp: &authpb.KYCResponse{
			Status:    1,
			Fname:     "Ali",
			Lname:     "Test",
			Birthdate: jalaliBirth,
		},
	}
	levels := &stubLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{
				Id:   3,
				Name: "Gold",
				Slug: "3",
			},
			PreviousLevels: []*levelspb.Level{
				{Id: 1, Name: "Bronze", Slug: "1"},
				{Id: 2, Name: "Silver", Slug: "2"},
			},
		},
		gems: map[uint64]*levelspb.LevelGem{
			1: {Id: 11, Name: "Bronze Gem", PngFile: "https://gem/bronze.png"},
			2: {Id: 12, Name: "Silver Gem", PngFile: "https://gem/silver.png"},
			3: {Id: 13, Name: "Gold Gem", PngFile: "https://gem/gold.png"},
		},
	}

	svc := service.NewUserSearchService(db, kyc, levels)
	ctx := context.Background()

	mock.ExpectQuery("FROM users u").
		WithArgs("%ali%", "%ali%", "%ali%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(1, "U100", "Ali"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://img"))

	results, err := svc.SearchUsers(ctx, "ali", 20)
	require.NoError(t, err)
	require.Len(t, results, 1)

	got := results[0]
	assert.Equal(t, "Ali Test", got.Name)
	assert.True(t, got.Verified)
	assert.Equal(t, int32(25), got.Age)
	assert.Equal(t, "Gold", got.Level)
	require.Len(t, got.Levels, 3)
	// descending by slug: 3, 2, 1
	assert.Equal(t, "3", got.Levels[0].Slug)
	assert.Equal(t, uint64(3), got.Levels[0].ID)
	require.NotNil(t, got.Levels[0].Gem)
	assert.Equal(t, uint64(13), got.Levels[0].Gem.ID)
	assert.Equal(t, "Gold Gem", got.Levels[0].Gem.Name)
	assert.Equal(t, "https://gem/gold.png", got.Levels[0].Gem.Image)
	assert.Equal(t, "2", got.Levels[1].Slug)
	assert.Equal(t, "1", got.Levels[2].Slug)
	assert.Equal(t, []uint64{1}, kyc.calls)
	assert.Equal(t, []uint64{1}, levels.calls)
	assert.ElementsMatch(t, []uint64{1, 2, 3}, levels.gemCalls)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSearchService_SearchUsers_UnverifiedKYCAndLevelsFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	kyc := &stubKYCPort{
		resp: &authpb.KYCResponse{
			Status:    0,
			Birthdate: "1370/01/01",
		},
	}
	levels := &stubLevelsPort{err: errors.New("levels down")}

	svc := service.NewUserSearchService(db, kyc, levels)
	ctx := context.Background()

	mock.ExpectQuery("FROM users u").
		WithArgs("%bob%", "%bob%", "%bob%", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(2, "U200", "Bob"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(2)).
		WillReturnError(assert.AnError)

	results, err := svc.SearchUsers(ctx, "bob", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Verified)
	assert.Equal(t, "Bob", results[0].Name)
	assert.Equal(t, int32(0), results[0].Age)
	assert.Empty(t, results[0].Level)
	assert.Empty(t, results[0].Levels)
	assert.Nil(t, results[0].Image)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSearchService_SearchUsers_NilClients_StillReturnsBaseFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewUserSearchService(db, nil, nil)
	ctx := context.Background()

	mock.ExpectQuery("FROM users u").
		WithArgs("%ali%", "%ali%", "%ali%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(1, "U100", "Ali"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://img"))

	results, err := svc.SearchUsers(ctx, "ali", 20)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Ali", results[0].Name)
	assert.False(t, results[0].Verified)
	assert.Equal(t, int32(0), results[0].Age)
	assert.Empty(t, results[0].Levels)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSearchService_SearchUsers_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewUserSearchService(db, nil, nil)
	ctx := context.Background()

	mock.ExpectQuery("FROM users u").WithArgs("%bad%", "%bad%", "%bad%", 10).WillReturnError(assert.AnError)
	_, err = svc.SearchUsers(ctx, "bad", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search users")
}

func TestUserSearchService_SearchUsers_KYCErrorDoesNotFailSearch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	kyc := &stubKYCPort{err: errors.New("auth down")}
	levels := &stubLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{Id: 1, Name: "Bronze", Slug: "1"},
		},
		gems: map[uint64]*levelspb.LevelGem{
			1: {Id: 9, Name: "B Gem", PngFile: "b.png"},
		},
	}
	svc := service.NewUserSearchService(db, kyc, levels)

	mock.ExpectQuery("FROM users u").
		WithArgs("%x%", "%x%", "%x%", 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(9, "U9", "X"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://x"))

	results, err := svc.SearchUsers(context.Background(), "x", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Verified)
	assert.Equal(t, int32(0), results[0].Age)
	assert.Equal(t, "Bronze", results[0].Level)
	require.Len(t, results[0].Levels, 1)
	assert.Equal(t, "1", results[0].Levels[0].Slug)
	require.NotNil(t, results[0].Levels[0].Gem)
	assert.Equal(t, "B Gem", results[0].Levels[0].Gem.Name)
}

func TestUserSearchService_SearchUsers_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewUserSearchService(db, nil, nil)
	mock.ExpectQuery("FROM users u").
		WithArgs("%bad%", "%bad%", "%bad%", 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow("not-a-uint", "U", "N"))
	_, err = svc.SearchUsers(context.Background(), "bad", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to scan user")
}

func TestUserSearchService_SearchUsers_BirthdateAndLevelVariants(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	laterAnniversary := time.Now().UTC().AddDate(-25, 1, 0)
	kyc := &stubKYCPort{
		resp: &authpb.KYCResponse{
			Status:    1,
			Birthdate: laterAnniversary.Format("2006-01-02"),
		},
	}
	levels := &stubLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{
				Id:   10,
				Name: "Gold",
				Slug: "gold",
				Gem:  &levelspb.LevelGem{Id: 99, Name: "Gold Gem", PngFile: "g.png"},
			},
			PreviousLevels: []*levelspb.Level{
				nil,
				{Id: 10, Name: "Gold dup", Slug: "gold"},
				{Id: 2, Name: "Silver", Slug: "silver"},
				{Id: 1, Name: "Bronze", Slug: "bronze", Gem: &levelspb.LevelGem{Id: 0, Name: ""}},
			},
		},
		gems: map[uint64]*levelspb.LevelGem{
			1: {Id: 11, Name: "Bronze Gem", PngFile: "b.png"},
		},
	}
	svc := service.NewUserSearchService(db, kyc, levels)

	mock.ExpectQuery("FROM users u").
		WithArgs("%z%", "%z%", "%z%", 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(3, "U3", "Z"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(3)).
		WillReturnError(sql.ErrNoRows)

	results, err := svc.SearchUsers(context.Background(), "z", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Verified)
	assert.Equal(t, int32(24), results[0].Age)
	assert.Nil(t, results[0].Image)
	require.Len(t, results[0].Levels, 3)
	assert.Equal(t, "silver", results[0].Levels[0].Slug)
	assert.Equal(t, "gold", results[0].Levels[1].Slug)
	assert.Equal(t, "bronze", results[0].Levels[2].Slug)
	require.NotNil(t, results[0].Levels[1].Gem)
	assert.Equal(t, uint64(99), results[0].Levels[1].Gem.ID)
	require.NotNil(t, results[0].Levels[2].Gem)
	assert.Equal(t, "Bronze Gem", results[0].Levels[2].Gem.Name)
	assert.ElementsMatch(t, []uint64{1, 2}, levels.gemCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSearchService_SearchUsers_InvalidAndEmptyBirthdates(t *testing.T) {
	cases := []struct {
		name      string
		birthdate string
	}{
		{name: "Empty", birthdate: ""},
		{name: "Invalid", birthdate: "not-a-date"},
		{name: "FutureGregorian", birthdate: "2099-01-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			kyc := &stubKYCPort{resp: &authpb.KYCResponse{Status: 1, Birthdate: tc.birthdate}}
			svc := service.NewUserSearchService(db, kyc, &stubLevelsPort{resp: nil})
			mock.ExpectQuery("FROM users u").
				WithArgs("%q%", "%q%", "%q%", 3).
				WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
					AddRow(4, "U4", "Q"))
			mock.ExpectQuery("SELECT url FROM images").
				WithArgs(uint64(4)).
				WillReturnError(sql.ErrNoRows)
			results, err := svc.SearchUsers(context.Background(), "q", 3)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, int32(0), results[0].Age)
			assert.Empty(t, results[0].Levels)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserSearchService_SearchUsers_NilKYCAndGetLevelGemError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	kyc := &stubKYCPort{resp: nil}
	levels := &stubLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: nil,
			PreviousLevels: []*levelspb.Level{
				{Id: 7, Name: "Seven", Slug: "7"},
			},
		},
		gemErr: errors.New("gem down"),
	}
	svc := service.NewUserSearchService(db, kyc, levels)
	mock.ExpectQuery("FROM users u").
		WithArgs("%n%", "%n%", "%n%", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow(8, "U8", "N"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(8)).
		WillReturnError(sql.ErrNoRows)

	results, err := svc.SearchUsers(context.Background(), "n", 2)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Verified)
	assert.Equal(t, int32(0), results[0].Age)
	assert.Empty(t, results[0].Level)
	require.Len(t, results[0].Levels, 1)
	assert.Equal(t, uint64(7), results[0].Levels[0].ID)
	assert.Nil(t, results[0].Levels[0].Gem)
	require.NoError(t, mock.ExpectationsWereMet())
}
