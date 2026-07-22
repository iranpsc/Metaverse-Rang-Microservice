package policy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/policy"
)

func TestJoinRequestPolicy_CanView(t *testing.T) {
	p := policy.NewJoinRequestPolicy()

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)
	viewerUserID := uint64(1)

	t.Run("Success_AsSender", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0,
			Relationship: "offspring",
		}

		canView := p.CanView(ctx, viewerUserID, request)
		assert.True(t, canView)
	})

	t.Run("Success_AsReceiver", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0,
			Relationship: "offspring",
		}

		canView := p.CanView(ctx, toUserID, request)
		assert.True(t, canView)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		unauthorizedUserID := uint64(3)
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0,
			Relationship: "offspring",
		}

		canView := p.CanView(ctx, unauthorizedUserID, request)
		assert.False(t, canView)
	})
}

func TestJoinRequestPolicy_CanDelete(t *testing.T) {
	p := policy.NewJoinRequestPolicy()

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canDelete := p.CanDelete(ctx, fromUserID, request)
		assert.True(t, canDelete)
	})

	t.Run("NotPending", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       1, // accepted
			Relationship: "offspring",
		}

		canDelete := p.CanDelete(ctx, fromUserID, request)
		assert.False(t, canDelete)
	})

	t.Run("NotSender", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canDelete := p.CanDelete(ctx, toUserID, request)
		assert.False(t, canDelete)
	})
}

func TestJoinRequestPolicy_CanAccept(t *testing.T) {
	p := policy.NewJoinRequestPolicy()

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canAccept := p.CanAccept(ctx, toUserID, request)
		assert.True(t, canAccept)
	})

	t.Run("NotReceiver", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canAccept := p.CanAccept(ctx, fromUserID, request)
		assert.False(t, canAccept)
	})

	t.Run("NotPending", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       1, // already accepted
			Relationship: "offspring",
		}

		canAccept := p.CanAccept(ctx, toUserID, request)
		assert.False(t, canAccept)
	})
}

func TestJoinRequestPolicy_CanReject(t *testing.T) {
	p := policy.NewJoinRequestPolicy()

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("Success", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canReject := p.CanReject(ctx, toUserID, request)
		assert.True(t, canReject)
	})

	t.Run("NotReceiver", func(t *testing.T) {
		request := &models.JoinRequest{
			ID:           1,
			FromUser:     fromUserID,
			ToUser:       toUserID,
			Status:       0, // pending
			Relationship: "offspring",
		}

		canReject := p.CanReject(ctx, fromUserID, request)
		assert.False(t, canReject)
	})
}
