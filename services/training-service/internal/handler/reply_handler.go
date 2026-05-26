package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonpb "metargb/shared/pb/common"
	trainingpb "metargb/shared/pb/training"
	"metargb/training-service/internal/service"
)

type ReplyHandler struct {
	trainingpb.UnimplementedReplyServiceServer
	service *service.ReplyService
}

func RegisterReplyHandler(grpcServer *grpc.Server, svc *service.ReplyService) {
	handler := &ReplyHandler{service: svc}
	trainingpb.RegisterReplyServiceServer(grpcServer, handler)
}

// GetReplies retrieves replies for a comment
func (h *ReplyHandler) GetReplies(ctx context.Context, req *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error) {
	page := int32(1)
	perPage := int32(10) // Default per API spec

	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			perPage = req.Pagination.PerPage
		}
	}

	replies, total, err := h.service.GetReplies(ctx, req.CommentId, page, perPage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get replies: %v", err)
	}

	response := &trainingpb.RepliesResponse{
		Replies: make([]*trainingpb.CommentResponse, 0, len(replies)),
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}

	for _, reply := range replies {
		response.Replies = append(response.Replies, h.buildReplyResponse(reply))
	}

	return response, nil
}

// AddReply creates a new reply
func (h *ReplyHandler) AddReply(ctx context.Context, req *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error) {
	reply, err := h.service.AddReply(ctx, req.ParentCommentId, req.UserId, req.Content)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return h.buildReplyResponse(reply), nil
}

// UpdateReply updates a reply
func (h *ReplyHandler) UpdateReply(ctx context.Context, req *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error) {
	reply, err := h.service.UpdateReply(ctx, req.ReplyId, req.UserId, req.Content)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return h.buildReplyResponse(reply), nil
}

// DeleteReply deletes a reply
func (h *ReplyHandler) DeleteReply(ctx context.Context, req *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error) {
	if err := h.service.DeleteReply(ctx, req.ReplyId, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete reply: %v", err)
	}

	return &commonpb.Empty{}, nil
}

// AddReplyInteraction adds or updates a user's interaction on a reply
func (h *ReplyHandler) AddReplyInteraction(ctx context.Context, req *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error) {
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = getIPAddressFromContext(ctx)
	}

	if err := h.service.AddReplyInteraction(ctx, req.ReplyId, req.UserId, req.Liked, ipAddress); err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

func (h *ReplyHandler) buildReplyResponse(reply *service.CommentDetails) *trainingpb.CommentResponse {
	// Reuse the same structure as comment response
	resp := &trainingpb.CommentResponse{
		Id:        reply.Comment.ID,
		VideoId:   reply.Comment.CommentableID,
		UserId:    reply.Comment.UserID,
		Content:   reply.Comment.Content,
		CreatedAt: reply.CreatedAtJalali,
		UpdatedAt: reply.UpdatedAtJalali,
	}

	if reply.Comment.ParentID != nil {
		resp.ParentId = *reply.Comment.ParentID
	}

	if reply.User != nil {
		resp.User = &commonpb.UserBasic{
			Id:    reply.User.ID,
			Name:  reply.User.Name,
			Code:  reply.User.Code,
			Email: reply.User.Email,
		}
		if reply.User.ProfilePhoto != "" {
			resp.User.ProfilePhoto = buildUploadURL(reply.User.ProfilePhoto)
		}
	}

	if reply.Stats != nil {
		resp.Stats = &trainingpb.CommentStats{
			LikesCount:    reply.Stats.LikesCount,
			DislikesCount: reply.Stats.DislikesCount,
			RepliesCount:  reply.Stats.RepliesCount,
		}
	}

	return resp
}

func getIPAddressFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
			return ips[0]
		}
		if ips := md.Get("x-real-ip"); len(ips) > 0 {
			return ips[0]
		}
	}
	return "unknown"
}
