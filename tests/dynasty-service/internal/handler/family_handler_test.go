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
