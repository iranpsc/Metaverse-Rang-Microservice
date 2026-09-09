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
	"metarang/dynasty-service/internal/policy"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	"metarang/dynasty-service/internal/validation"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
)

func TestLocale_SetProjectLocale(t *testing.T) {
	handler.SetProjectLocale("FA")
	handler.SetProjectLocale("en")
	handler.SetProjectLocale("de") // normalizes to en
}

func TestDynastyPolicy_SetClients(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := policy.NewDynastyPolicy(repository.NewDynastyRepository(db))
	p.SetClients(struct{}{}, struct{}{})
}

func TestFamilyHandler_SetChildPermissions_AllFlags(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	permSvc := service.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewJoinRequestRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewDynastyRepository(db),
	)
	h := handler.NewFamilyHandler(nil, permSvc)
	ctx := context.Background()
	now := time.Now()
	parent, child := uint64(1), uint64(2)

	flags := []struct {
		perms *dynastypb.ChildPermissions
		col   string
	}{
		{&dynastypb.ChildPermissions{BFR: true}, "BFR"},
		{&dynastypb.ChildPermissions{SF: true}, "SF"},
		{&dynastypb.ChildPermissions{W: true}, "W"},
		{&dynastypb.ChildPermissions{JU: true}, "JU"},
		{&dynastypb.ChildPermissions{DM: true}, "DM"},
		{&dynastypb.ChildPermissions{PIUP: true}, "PIUP"},
		{&dynastypb.ChildPermissions{PITC: true}, "PITC"},
		{&dynastypb.ChildPermissions{PIC: true}, "PIC"},
		{&dynastypb.ChildPermissions{ESOO: true}, "ESOO"},
		{&dynastypb.ChildPermissions{COTB: true}, "COTB"},
	}
	for _, tc := range flags {
		mock.ExpectQuery("SELECT TIMESTAMPDIFF").
			WithArgs(child).
			WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(true))
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(parent).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, parent, 100, now, now))
		mock.ExpectQuery("SELECT id, dynasty_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, 1, now, now))
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
		mock.ExpectQuery("SELECT id, family_id, user_id").
			WithArgs(uint64(1), int32(1000), int32(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).
				AddRow(1, 1, parent, "owner", now, now).
				AddRow(2, 1, child, "offspring", now, now))
		mock.ExpectQuery("SELECT id, user_id").
			WithArgs(child).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "user_id", "verified", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at",
			}).AddRow(1, child, true, false, false, false, false, false, false, false, false, false, false, now, now))
		mock.ExpectExec("UPDATE children_permissions SET "+tc.col).
			WithArgs(true, child).
			WillReturnResult(sqlmock.NewResult(0, 1))

		_, err := h.SetChildPermissions(ctx, &dynastypb.SetChildPermissionsRequest{
			ParentUserId: parent,
			ChildUserId:  child,
			Permissions:  tc.perms,
		})
		require.NoError(t, err, tc.col)
	}

	_, err = h.SetChildPermissions(ctx, &dynastypb.SetChildPermissionsRequest{
		ParentUserId: parent,
		ChildUserId:  child,
		Permissions:  &dynastypb.ChildPermissions{},
	})
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrizeHandler_GetPrizesHappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now()
	svc := service.NewPrizeService(db, repository.NewPrizeRepository(db), nil, nil, nil)
	h := handler.NewPrizeHandler(svc)

	mock.ExpectQuery("SELECT rp.id, rp.user_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"rp.id", "rp.user_id", "rp.prize_id", "rp.message", "rp.created_at", "rp.updated_at",
			"dp.id", "dp.member", "dp.satisfaction", "dp.introduction_profit_increase",
			"dp.accumulated_capital_reserve", "dp.data_storage", "dp.psc",
		}).AddRow(3, 1, 1, "m", now, now, 1, "offspring", 0.1, 0.2, 0.3, 0.4, 1000))

	resp, err := h.GetPrizes(context.Background(), &dynastypb.GetPrizesRequest{
		UserId:     1,
		Pagination: &commonpb.PaginationRequest{Page: 0, PerPage: 0},
	})
	require.NoError(t, err)
	require.Len(t, resp.Prizes, 1)
	assert.Equal(t, "offspring", resp.Prizes[0].Member)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestJoinRequestHandler_AcceptAndSearchAndDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now()

	joinSvc := service.NewJoinRequestService(
		repository.NewJoinRequestRepository(db),
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		nil,
		validation.NewFamilyValidator(repository.NewValidationRepository(db)),
		nil,
		"",
	)
	permSvc := service.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewJoinRequestRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewDynastyRepository(db),
	)
	searchSvc := service.NewUserSearchService(db, nil, nil)
	h := handler.NewJoinRequestHandler(joinSvc, permSvc, searchSvc)
	ctx := context.Background()
	fromUser, toUser := uint64(10), uint64(20)

	mock.ExpectQuery("SELECT id, from_user, to_user").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from_user", "to_user", "status", "relationship", "message", "created_at", "updated_at"}).
			AddRow(1, fromUser, toUser, 0, "brother", nil, now, now))
	mock.ExpectExec("UPDATE join_requests SET status").
		WithArgs(1, uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
			AddRow(1, fromUser, 100, now, now))
	mock.ExpectQuery("SELECT id, dynasty_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
			AddRow(1, 1, now, now))
	mock.ExpectExec("INSERT INTO family_members").
		WithArgs(sqlmock.AnyArg(), toUser, "brother").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT TIMESTAMPDIFF").
		WithArgs(fromUser).
		WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
	mock.ExpectQuery("SELECT TIMESTAMPDIFF").
		WithArgs(toUser).
		WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))

	_, err = h.AcceptJoinRequest(ctx, &dynastypb.AcceptJoinRequestRequest{RequestId: 1, UserId: toUser})
	require.NoError(t, err)

	mock.ExpectQuery("SELECT id, BFR, SF, W, JU, DM").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB", "created_at", "updated_at",
		}).AddRow(1, true, true, true, true, true, true, true, true, true, true, now, now))
	def, err := h.GetDefaultPermissions(ctx, &dynastypb.GetDefaultPermissionsRequest{Relationship: "offspring"})
	require.NoError(t, err)
	require.NotNil(t, def.Permissions)

	mock.ExpectQuery("FROM users u").
		WithArgs("%ali%", "%ali%", "%ali%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "display_name"}).AddRow(1, "A", "Ali", "Ali Test"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(1)).
		WillReturnError(sql.ErrNoRows)

	search, err := h.SearchUsers(ctx, &dynastypb.SearchUsersRequest{SearchTerm: "ali"})
	require.NoError(t, err)
	require.NotNil(t, search)
	require.Len(t, search.Data, 1)
	assert.False(t, search.Data[0].Verified)
	assert.Equal(t, int32(0), search.Data[0].Age)
	assert.Empty(t, search.Data[0].Levels)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyHandler_GetUserDynasty_Existing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now()
	h := handler.NewDynastyHandler(service.NewDynastyService(
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		"",
	))
	userID := uint64(7)

	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
			AddRow(1, userID, 100, now, now))
	mock.ExpectQuery("SELECT id, dynasty_id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
			AddRow(1, 1, now, now))
	mock.ExpectQuery("SELECT").
		WithArgs(uint64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability"}).
			AddRow(100, "1", "a", "d", "15000"))
	mock.ExpectQuery("SELECT").
		WithArgs(userID, uint64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "density", "stability", "area"}))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	resp, err := h.GetUserDynasty(context.Background(), &dynastypb.GetUserDynastyRequest{UserId: userID})
	require.NoError(t, err)
	assert.True(t, resp.UserHasDynasty)
	require.NotNil(t, resp.DynastyFeature)
	require.Len(t, resp.Features, 1)
	assert.Equal(t, uint64(100), resp.Features[0].Id)
	assert.Equal(t, "15000", resp.Features[0].Stability)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyService_FeatureColorVariants(t *testing.T) {
	for _, tc := range []struct {
		karbari string
	}{{"m"}, {"a"}, {"x"}} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

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
				AddRow(1, 10, 100, now, now.Add(-time.Hour)))
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow(tc.karbari, 10000.0))
		// debtAmount = 100 -> > 0
		mock.ExpectExec("INSERT INTO debts").
			WithArgs(uint64(10), 100.0, "update-dynasty-feature").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO locked_features").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE feature_properties SET label").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE dynasties SET feature_id").
			WithArgs(uint64(200), uint64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, svc.UpdateDynastyFeature(context.Background(), 1, 200, 10), tc.karbari)
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}
