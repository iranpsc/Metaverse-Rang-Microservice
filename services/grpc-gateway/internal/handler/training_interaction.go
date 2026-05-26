package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"google.golang.org/grpc/metadata"

	"metargb/grpc-gateway/internal/middleware"
)

const (
	headerVideoUserInteraction    = "x-video-user-interaction"
	headerCommentUserInteractions = "x-comment-user-interactions"
	headerUserID                  = "x-user-id"
)

func (h *TrainingHandler) trainingContextWithUser(r *http.Request) context.Context {
	ctx := r.Context()
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil || userCtx.UserID == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, headerUserID, strconv.FormatUint(userCtx.UserID, 10))
}

func applyVideoInteractionHeader(resp map[string]interface{}, header metadata.MD) {
	if vals := header.Get(headerVideoUserInteraction); len(vals) > 0 {
		resp["user_interaction"] = vals[0] == "true"
	}
}

func applyCommentInteractionsHeader(comments []map[string]interface{}, header metadata.MD) {
	vals := header.Get(headerCommentUserInteractions)
	if len(vals) == 0 {
		return
	}

	var interactions map[string]bool
	if err := json.Unmarshal([]byte(vals[0]), &interactions); err != nil {
		return
	}

	for i, comment := range comments {
		idVal, ok := comment["id"]
		if !ok {
			continue
		}
		var idStr string
		switch v := idVal.(type) {
		case uint64:
			idStr = strconv.FormatUint(v, 10)
		case float64:
			idStr = strconv.FormatUint(uint64(v), 10)
		case int:
			idStr = strconv.Itoa(v)
		case int64:
			idStr = strconv.FormatInt(v, 10)
		default:
			continue
		}
		if liked, ok := interactions[idStr]; ok {
			comments[i]["user_interaction"] = liked
		}
	}
}

func parseLikedFromRequest(r *http.Request) (bool, error) {
	if likedStr := r.URL.Query().Get("liked"); likedStr != "" {
		return likedStr == "1" || likedStr == "true", nil
	}

	var body struct {
		Liked bool `json:"liked"`
	}
	if err := decodeRequestBody(r, &body); err != nil {
		return false, err
	}
	return body.Liked, nil
}
