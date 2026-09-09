package handler_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	"metarang/dynasty-service/internal/validation"
	dynastypb "metarang/shared/pb/dynasty"
)

func TestJoinRequestHandler_Methods_NilServices(t *testing.T) {
	h := handler.NewJoinRequestHandler(nil, nil, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"SendJoinRequest", func() error { _, err := h.SendJoinRequest(ctx, &dynastypb.SendJoinRequestRequest{}); return err }},
		{"GetSentRequests", func() error { _, err := h.GetSentRequests(ctx, &dynastypb.GetSentRequestsRequest{}); return err }},
		{"GetReceivedRequests", func() error {
			_, err := h.GetReceivedRequests(ctx, &dynastypb.GetReceivedRequestsRequest{})
			return err
		}},
		{"GetJoinRequest", func() error { _, err := h.GetJoinRequest(ctx, &dynastypb.GetJoinRequestRequest{}); return err }},
		{"AcceptJoinRequest", func() error { _, err := h.AcceptJoinRequest(ctx, &dynastypb.AcceptJoinRequestRequest{}); return err }},
		{"RejectJoinRequest", func() error { _, err := h.RejectJoinRequest(ctx, &dynastypb.RejectJoinRequestRequest{}); return err }},
		{"DeleteJoinRequest", func() error { _, err := h.DeleteJoinRequest(ctx, &dynastypb.DeleteJoinRequestRequest{}); return err }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.Internal, st.Code())
		})
	}
}

func TestJoinRequestHandler_ValidationPaths(t *testing.T) {
	h := handler.NewJoinRequestHandler(nil, &service.PermissionService{}, &service.UserSearchService{})
	ctx := context.Background()

	_, err := h.GetDefaultPermissions(ctx, &dynastypb.GetDefaultPermissionsRequest{Relationship: "father"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	_, err = h.SearchUsers(ctx, &dynastypb.SearchUsersRequest{SearchTerm: ""})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestJoinRequestHandler_SendJoinRequest_AppliesFamilyRules(t *testing.T) {
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

	_, err = h.SendJoinRequest(ctx, &dynastypb.SendJoinRequestRequest{
		FromUserId: 1, ToUserId: 2, Relationship: "cousin",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "نوع رابطه نامعتبر است")

	mock.ExpectQuery("SELECT TIMESTAMPDIFF").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"is_under_18"}).AddRow(false))
	_, err = h.SendJoinRequest(ctx, &dynastypb.SendJoinRequestRequest{
		FromUserId: 1, ToUserId: 1, Relationship: "brother",
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "خودتان")
	require.NoError(t, mock.ExpectationsWereMet())
}
