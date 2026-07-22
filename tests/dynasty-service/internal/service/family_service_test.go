package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

func TestFamilyService_GetFamily(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db))
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

	svc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db))
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
