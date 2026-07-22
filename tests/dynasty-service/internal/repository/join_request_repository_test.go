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

func TestJoinRequestRepository_BasicFlow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewJoinRequestRepository(db)
	ctx := context.Background()
	now := time.Now()

	msg := "join"
	req := &models.JoinRequest{FromUser: 1, ToUser: 2, Status: 0, Relationship: "offspring", Message: &msg}
	mock.ExpectExec("INSERT INTO join_requests").WithArgs(uint64(1), uint64(2), int16(0), "offspring", &msg).WillReturnResult(sqlmock.NewResult(9, 1))
	require.NoError(t, r.CreateJoinRequest(ctx, req))
	assert.Equal(t, uint64(9), req.ID)

	mock.ExpectQuery("SELECT id, from_user, to_user, status").WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).AddRow(9, 1, 2, 0, "offspring", msg, now, now))
	got, err := r.GetJoinRequestByID(ctx, 9)
	require.NoError(t, err)
	require.NotNil(t, got)

	mock.ExpectExec("UPDATE join_requests SET status").WithArgs(int16(1), uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.UpdateJoinRequestStatus(ctx, 9, 1))

	mock.ExpectExec("DELETE FROM join_requests WHERE id").WithArgs(uint64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.DeleteJoinRequest(ctx, 9))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestRepository_ListAndAge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewJoinRequestRepository(db)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM join_requests WHERE from_user`).WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM join_requests").WithArgs(uint64(1), int32(10), int32(0)).WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).AddRow(1, 1, 2, 0, "offspring", "m", now, now))
	sent, total, err := r.GetSentRequests(ctx, 1, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	assert.Len(t, sent, 1)

	mock.ExpectQuery(`SELECT TIMESTAMPDIFF\(YEAR, birthdate, CURDATE\(\)\) < 18`).WithArgs(uint64(2)).WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))
	under18, err := r.CheckUserAge(ctx, 2)
	require.NoError(t, err)
	assert.True(t, under18)

	require.NoError(t, mock.ExpectationsWereMet())
}
