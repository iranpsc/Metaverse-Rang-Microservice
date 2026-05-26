package handler

import (
	"context"
	"encoding/json"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"metargb/training-service/internal/service"
)

const (
	headerVideoUserInteraction    = "x-video-user-interaction"
	headerCommentUserInteractions = "x-comment-user-interactions"
	headerUserID                  = "x-user-id"
)

func userIDFromContext(ctx context.Context, fallback uint64) uint64 {
	if fallback > 0 {
		return fallback
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ids := md.Get(headerUserID); len(ids) > 0 {
			if id, err := strconv.ParseUint(ids[0], 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

func setVideoInteractionHeader(ctx context.Context, interaction *bool) {
	if interaction == nil {
		return
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs(headerVideoUserInteraction, strconv.FormatBool(*interaction)))
}

func setCommentInteractionsHeader(ctx context.Context, comments []*service.CommentDetails) {
	if len(comments) == 0 {
		return
	}

	interactions := make(map[string]bool, len(comments))
	for _, comment := range comments {
		if comment == nil || comment.UserInteraction == nil {
			continue
		}
		interactions[strconv.FormatUint(comment.Comment.ID, 10)] = *comment.UserInteraction
	}
	if len(interactions) == 0 {
		return
	}

	payload, err := json.Marshal(interactions)
	if err != nil {
		return
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs(headerCommentUserInteractions, string(payload)))
}
