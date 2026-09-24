package handler_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/handler"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	"metarang/dynasty-service/internal/validation"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	levelspb "metarang/shared/pb/levels"
)

type stubFamilyLevelsPort struct {
	resp *levelspb.UserLevelResponse
	err  error
}

func (s *stubFamilyLevelsPort) GetUserLevel(context.Context, uint64) (*levelspb.UserLevelResponse, error) {
	return s.resp, s.err
}

func TestJoinRequestHandler_SendAndListHappyPaths(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	joinSvc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		nil,
		"",
	)
	h := handler.NewJoinRequestHandler(joinSvc, nil, nil)
	ctx := context.Background()
	now := time.Now()
	fromUser, toUser := uint64(1), uint64(2)

	mock.ExpectQuery("SELECT TIMESTAMPDIFF").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser, toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(fromUser, toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
			AddRow(uint64(1), fromUser, uint64(100), now, now))
	mock.ExpectQuery("SELECT id, dynasty_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
			AddRow(uint64(1), uint64(1), now, now))
	mock.ExpectQuery("SELECT message FROM dynasty_messages").
		WithArgs("reciever_message").
		WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("[sender-name] invited [reciever-name] as [relationship]"))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(fromUser, "C1", "Alice"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(fromUser).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(toUser, "C2", "Bob"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(toUser).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO join_requests").
		WithArgs(fromUser, toUser, 0, "brother", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(toUser, "C2", "Bob"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(toUser).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM dynasty_prizes").
		WithArgs("brother").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member", "satisfaction", "introduction_profit_increase",
			"accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at",
		}).AddRow(1, "brother", 0.5, 0.1, 0.2, 0.3, 10, now, now))

	resp, err := h.SendJoinRequest(ctx, &dynastypb.SendJoinRequestRequest{
		FromUserId: fromUser, ToUserId: toUser, Relationship: "brother",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Alice invited Bob as برادر", resp.Message)
	assert.NotNil(t, resp.ToUserInfo)
	assert.NotNil(t, resp.RequestPrize)

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(fromUser, int32(10), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(11, fromUser, toUser, 0, "brother", "hi", now, now))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(toUser, "C2", "Bob"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(toUser).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM dynasty_prizes").
		WithArgs("brother").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member", "satisfaction", "introduction_profit_increase",
			"accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at",
		}).AddRow(1, "brother", 0.5, 0.1, 0.2, 0.3, 10, now, now))

	list, err := h.GetSentRequests(ctx, &dynastypb.GetSentRequestsRequest{
		UserId:     fromUser,
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	require.NoError(t, err)
	require.Len(t, list.Requests, 1)
	assert.Equal(t, "hi", list.Requests[0].Message)

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(toUser, int32(10), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(11, fromUser, toUser, 0, "sister", "welcome", now, now))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(fromUser, "C1", "Alice"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(fromUser).
		WillReturnError(sql.ErrNoRows)

	recv, err := h.GetReceivedRequests(ctx, &dynastypb.GetReceivedRequestsRequest{
		UserId:     toUser,
		Pagination: &commonpb.PaginationRequest{Page: 0, PerPage: 0}, // exercise defaults
	})
	require.NoError(t, err)
	require.Len(t, recv.Requests, 1)
	assert.Equal(t, "welcome", recv.Requests[0].Message)

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(11, fromUser, toUser, 0, "brother", "hi", now, now))
	mock.ExpectQuery("SELECT u.id, u.code, u.name").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(toUser, "C2", "Bob"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(toUser).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("FROM dynasty_prizes").
		WithArgs("brother").
		WillReturnError(sql.ErrNoRows)

	one, err := h.GetJoinRequest(ctx, &dynastypb.GetJoinRequestRequest{RequestId: 11, UserId: fromUser})
	require.NoError(t, err)
	require.NotNil(t, one)

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(11, fromUser, toUser, 0, "brother", nil, now, now))
	mock.ExpectExec("UPDATE join_requests SET status").
		WithArgs(-1, uint64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = h.RejectJoinRequest(ctx, &dynastypb.RejectJoinRequestRequest{RequestId: 11, UserId: toUser})
	require.NoError(t, err)

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(12, fromUser, toUser, 0, "brother", nil, now, now))
	mock.ExpectExec("DELETE FROM join_requests").
		WithArgs(uint64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = h.DeleteJoinRequest(ctx, &dynastypb.DeleteJoinRequestRequest{RequestId: 12, UserId: fromUser})
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyHandler_GetFamilyAndMembersHappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	levels := &stubFamilyLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{Id: 3, Name: "Gold", Slug: "3"},
		},
	}
	familySvc := service.NewFamilyService(repository.NewFamilyRepository(db), repository.NewDynastyRepository(db), levels)
	h := handler.NewFamilyHandler(familySvc, nil)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery("SELECT id, dynasty_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
			AddRow(1, 9, now, now))
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, family_id, user_id").
		WithArgs(uint64(1), int32(1000), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).
			AddRow(1, 1, 5, "owner", now, now))
	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "O", "Owner"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnError(sql.ErrNoRows)

	resp, err := h.GetFamily(ctx, &dynastypb.GetFamilyRequest{FamilyId: 1, DynastyId: 9})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Members, 1)
	require.NotNil(t, resp.Members[0].UserInfo)
	assert.Equal(t, "Gold", resp.Members[0].UserInfo.Level)

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, family_id, user_id").
		WithArgs(uint64(1), int32(10), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).
			AddRow(1, 1, 5, "owner", now, now))
	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(5, "O", "Owner"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(5)).
		WillReturnError(sql.ErrNoRows)

	members, err := h.GetFamilyMembers(ctx, &dynastypb.GetFamilyMembersRequest{
		FamilyId:   1,
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	require.NoError(t, err)
	require.Len(t, members.Members, 1)
	require.NotNil(t, members.Members[0].UserInfo)
	assert.Equal(t, uint64(5), members.Members[0].UserInfo.Id)
	assert.Equal(t, "O", members.Members[0].UserInfo.Code)
	assert.Equal(t, "Owner", members.Members[0].UserInfo.Name)
	assert.Equal(t, "Gold", members.Members[0].UserInfo.Level)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyService_UpdateDynastyFeature_PenaltyPath(t *testing.T) {
	// covers featureColorByKarbari via service UpdateDynastyFeature within 30 days
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewDynastyService(
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		"",
	)
	now := time.Now()
	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
			AddRow(1, 10, 100, now, now.Add(-24*time.Hour)))
	mock.ExpectQuery("SELECT fp.karbari, fp.stability").
		WithArgs(uint64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("t", 20000.0))
	mock.ExpectExec("INSERT INTO debts").
		WithArgs(uint64(10), 200.0, "update-dynasty-feature").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO locked_features").
		WithArgs(uint64(100), "dynasty-feature-change", sqlmock.AnyArg(), 0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE feature_properties SET label").
		WithArgs("locked", uint64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE dynasties SET feature_id").
		WithArgs(uint64(200), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, svc.UpdateDynastyFeature(context.Background(), 1, 200, 10))
	require.NoError(t, mock.ExpectationsWereMet())
}
