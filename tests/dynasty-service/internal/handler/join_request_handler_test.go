package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"

	"metarang/dynasty-service/internal/service"
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
