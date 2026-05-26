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

type CommentHandler struct {
	trainingpb.UnimplementedCommentServiceServer
	service *service.CommentService
}

func RegisterCommentHandler(grpcServer *grpc.Server, svc *service.CommentService) {
	handler := &CommentHandler{service: svc}
	trainingpb.RegisterCommentServiceServer(grpcServer, handler)
}

// GetComments retrieves top-level comments for a video
func (h *CommentHandler) GetComments(ctx context.Context, req *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error) {
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

	userID := userIDFromContext(ctx, 0)
	comments, total, err := h.service.GetComments(ctx, req.VideoId, page, perPage, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get comments: %v", err)
	}

	response := &trainingpb.CommentsResponse{
		Comments: make([]*trainingpb.CommentResponse, 0, len(comments)),
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}

	for _, comment := range comments {
		response.Comments = append(response.Comments, h.buildCommentResponse(comment))
	}

	setCommentInteractionsHeader(ctx, comments)
	return response, nil
}

// AddComment creates a new comment
func (h *CommentHandler) AddComment(ctx context.Context, req *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error) {
	comment, err := h.service.AddComment(ctx, req.VideoId, req.UserId, req.Content)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return h.buildCommentResponse(comment), nil
}

// UpdateComment updates a comment
func (h *CommentHandler) UpdateComment(ctx context.Context, req *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error) {
	comment, err := h.service.UpdateComment(ctx, req.CommentId, req.UserId, req.Content)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return h.buildCommentResponse(comment), nil
}

// DeleteComment deletes a comment
func (h *CommentHandler) DeleteComment(ctx context.Context, req *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error) {
	if err := h.service.DeleteComment(ctx, req.CommentId, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete comment: %v", err)
	}

	return &commonpb.Empty{}, nil
}

// AddCommentInteraction adds or updates a user's interaction on a comment
func (h *CommentHandler) AddCommentInteraction(ctx context.Context, req *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error) {
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = getIPAddress(ctx)
	}

	if err := h.service.AddCommentInteraction(ctx, req.CommentId, req.UserId, req.Liked, ipAddress); err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

// ReportComment creates a report for a comment
func (h *CommentHandler) ReportComment(ctx context.Context, req *trainingpb.ReportCommentRequest) (*commonpb.Empty, error) {
	// Get video ID from the comment (commentable_id)
	// We need to get the comment first to extract the video ID
	comment, err := h.service.GetCommentByID(ctx, req.CommentId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "comment not found: %v", err)
	}
	if comment == nil {
		return nil, status.Errorf(codes.NotFound, "comment not found")
	}

	// Get video ID from comment's commentable_id
	videoID := comment.Comment.CommentableID

	if err := h.service.ReportComment(ctx, videoID, req.CommentId, req.UserId, req.Content); err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

func (h *CommentHandler) buildCommentResponse(comment *service.CommentDetails) *trainingpb.CommentResponse {
	resp := &trainingpb.CommentResponse{
		Id:        comment.Comment.ID,
		VideoId:   comment.Comment.CommentableID,
		UserId:    comment.Comment.UserID,
		Content:   comment.Comment.Content,
		CreatedAt: comment.CreatedAtJalali,
		UpdatedAt: comment.UpdatedAtJalali,
	}

	if comment.Comment.ParentID != nil {
		resp.ParentId = *comment.Comment.ParentID
	}

	if comment.User != nil {
		resp.User = &commonpb.UserBasic{
			Id:    comment.User.ID,
			Name:  comment.User.Name,
			Code:  comment.User.Code,
			Email: comment.User.Email,
		}
		if comment.User.ProfilePhoto != "" {
			resp.User.ProfilePhoto = buildUploadURL(comment.User.ProfilePhoto)
		}
	}

	if comment.Stats != nil {
		resp.Stats = &trainingpb.CommentStats{
			LikesCount:    comment.Stats.LikesCount,
			DislikesCount: comment.Stats.DislikesCount,
			RepliesCount:  comment.Stats.RepliesCount,
		}
	}

	return resp
}

func getIPAddress(ctx context.Context) string {
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
