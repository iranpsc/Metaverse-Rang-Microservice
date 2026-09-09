package handler_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	levelspb "metarang/shared/pb/levels"
)

func TestFamilyHandler_NilServiceErrors(t *testing.T) {
	h := handler.NewFamilyHandler(nil, nil)
	ctx := context.Background()

	_, err := h.GetFamily(ctx, &dynastypb.GetFamilyRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())

	_, err = h.GetFamilyMembers(ctx, &dynastypb.GetFamilyMembersRequest{})
	require.Error(t, err)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFamilyHandler_SetChildPermissions_Validation(t *testing.T) {
	h := handler.NewFamilyHandler(nil, &service.PermissionService{})
	ctx := context.Background()

	_, err := h.SetChildPermissions(ctx, &dynastypb.SetChildPermissionsRequest{Permissions: nil})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFamilyHandler_GetFamilyMembers_IncludesUserLevel(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	levels := &stubFamilyLevelsPort{
		resp: &levelspb.UserLevelResponse{
			LatestLevel: &levelspb.Level{Id: 7, Name: "Silver", Slug: "2"},
		},
	}
	familySvc := service.NewFamilyService(
		repository.NewFamilyRepository(db),
		repository.NewDynastyRepository(db),
		levels,
	)
	h := handler.NewFamilyHandler(familySvc, nil)
	now := time.Now()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(uint64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, family_id, user_id").
		WithArgs(uint64(11), int32(10), int32(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "family_id", "user_id", "relationship", "created_at", "updated_at"}).
			AddRow(3, 11, 42, "offspring", now, now))
	mock.ExpectQuery("SELECT id, code, name FROM users").
		WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).AddRow(42, "U42", "Child"))
	mock.ExpectQuery("SELECT url FROM images").
		WithArgs(uint64(42)).
		WillReturnError(sql.ErrNoRows)

	resp, err := h.GetFamilyMembers(context.Background(), &dynastypb.GetFamilyMembersRequest{
		FamilyId:   11,
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	require.NoError(t, err)
	require.Len(t, resp.Members, 1)
	require.NotNil(t, resp.Members[0].UserInfo)
	assert.Equal(t, uint64(42), resp.Members[0].UserInfo.Id)
	assert.Equal(t, "U42", resp.Members[0].UserInfo.Code)
	assert.Equal(t, "Child", resp.Members[0].UserInfo.Name)
	assert.Equal(t, "Silver", resp.Members[0].UserInfo.Level)
	require.NoError(t, mock.ExpectationsWereMet())
}
