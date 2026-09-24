package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
	levelspb "metarang/shared/pb/levels"
)

type stubUserLevelPort struct {
	resp  *levelspb.UserLevelResponse
	err   error
	calls []uint64
}

func (s *stubUserLevelPort) GetUserLevel(_ context.Context, userID uint64) (*levelspb.UserLevelResponse, error) {
	s.calls = append(s.calls, userID)
	return s.resp, s.err
}

func TestFamilyService_GetFamily(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), nil)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("FROM families WHERE id").WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).AddRow(10, 2, now, now))
	fam, err := svc.GetFamily(ctx, 10, 2)
	require.NoError(t, err)
	require.NotNil(t, fam)
	assert.Equal(t, uint64(2), fam.DynastyID)

	mock.ExpectQuery("FROM families WHERE id").WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).AddRow(10, 2, now, now))
	fam, err = svc.GetFamily(ctx, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, fam)

	mock.ExpectQuery("FROM families WHERE dynasty_id").WithArgs(uint64(2)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).AddRow(10, 2, now, now))
	fam, err = svc.GetFamily(ctx, 0, 2)
	require.NoError(t, err)
	require.NotNil(t, fam)

	mock.ExpectQuery("FROM families WHERE id").WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).AddRow(10, 99, now, now))
	_, err = svc.GetFamily(ctx, 10, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family not found")

	mock.ExpectQuery("FROM families WHERE id").WithArgs(uint64(99)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}))
	_, err = svc.GetFamily(ctx, 99, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family not found")

	_, err = svc.GetFamily(ctx, 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetFamilyMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), nil)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id`).WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, family_id, user_id, relationship").WithArgs(uint64(10), int32(10), int32(0)).WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).AddRow(1, 10, 5, "offspring", now, now))

	members, total, err := svc.GetFamilyMembers(ctx, 10, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	assert.Len(t, members, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetFamily_RepoError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), nil)
	mock.ExpectQuery("FROM families WHERE id").WithArgs(uint64(10)).WillReturnError(assert.AnError)
	_, err = svc.GetFamily(context.Background(), 10, 0)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetUserBasicInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), nil)
	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "C5", "Name5"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://img"))

	info, err := svc.GetUserBasicInfo(context.Background(), 5)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, uint64(5), info.ID)
	assert.Equal(t, "C5", info.Code)
	assert.Equal(t, "Name5", info.Name)
	require.NotNil(t, info.ProfilePhoto)
	assert.Equal(t, "https://img", *info.ProfilePhoto)
	assert.Equal(t, "", info.Level)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetUserBasicInfo_IncludesLatestLevel(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	levels := &stubUserLevelPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{Id: 3, Name: "Gold", Slug: "3"},
		},
	}
	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), levels)

	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "C5", "Name5"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://img"))

	info, err := svc.GetUserBasicInfo(context.Background(), 5)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "Gold", info.Level)
	assert.Equal(t, []uint64{5}, levels.calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetUserBasicInfo_LevelLookupFailureLeavesEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	levels := &stubUserLevelPort{err: errors.New("levels unavailable")}
	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), levels)

	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "C5", "Name5"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnError(sql.ErrNoRows)

	info, err := svc.GetUserBasicInfo(context.Background(), 5)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "", info.Level)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyService_GetUserBasicInfo_NoLatestLevel(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	levels := &stubUserLevelPort{resp: &levelspb.UserLevelResponse{}}
	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), levels)

	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "C5", "Name5"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnError(sql.ErrNoRows)

	info, err := svc.GetUserBasicInfo(context.Background(), 5)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "", info.Level)
	require.NoError(t, mock.ExpectationsWereMet())
}
