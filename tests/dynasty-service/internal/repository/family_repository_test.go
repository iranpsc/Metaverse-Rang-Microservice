package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"

	"metarang/dynasty-service/internal/models"
)

func TestFamilyRepository_CreateAndCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewFamilyRepository(db)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO families").WithArgs(uint64(1)).WillReturnResult(sqlmock.NewResult(11, 1))
	fam, err := r.CreateFamily(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(11), fam.ID)

	member := &models.FamilyMember{FamilyID: 11, UserID: 2, Relationship: "offspring"}
	mock.ExpectExec("INSERT INTO family_members").WithArgs(uint64(11), uint64(2), "offspring").WillReturnResult(sqlmock.NewResult(22, 1))
	require.NoError(t, r.CreateFamilyMember(ctx, member))
	assert.Equal(t, uint64(22), member.ID)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id`).WithArgs(uint64(11)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := r.GetFamilyMemberCount(ctx, 11)
	require.NoError(t, err)
	assert.Equal(t, int32(2), count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyRepository_GetByDynastyAndMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewFamilyRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, dynasty_id, created_at, updated_at").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).AddRow(11, 1, now, now))
	fam, err := r.GetFamilyByDynastyID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, fam)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id`).WithArgs(uint64(11)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, family_id, user_id, relationship").WithArgs(uint64(11), int32(10), int32(0)).WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).AddRow(1, 11, 2, "offspring", now, now))
	members, total, err := r.GetFamilyMembers(ctx, 11, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	assert.Len(t, members, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}
