package repository

import (
	"context"

	"metargb/training-service/internal/models"
)

// CommentRepositoryInterface defines the interface for comment repository operations
type CommentRepositoryInterface interface {
	GetComments(ctx context.Context, videoID uint64, page, perPage int32) ([]*models.Comment, int32, error)
	GetCommentByID(ctx context.Context, commentID uint64) (*models.Comment, error)
	AddComment(ctx context.Context, videoID, userID uint64, content string) (*models.Comment, error)
	UpdateComment(ctx context.Context, commentID, userID uint64, content string) error
	DeleteComment(ctx context.Context, commentID, userID uint64) error
	GetReplies(ctx context.Context, commentID uint64, page, perPage int32) ([]*models.Comment, int32, error)
	AddReply(ctx context.Context, parentCommentID, userID uint64, content string) (*models.Comment, error)
	UpdateReply(ctx context.Context, replyID, userID uint64, content string) error
	DeleteReply(ctx context.Context, replyID, userID uint64) error
	GetCommentStats(ctx context.Context, commentID uint64) (*models.CommentStats, error)
	GetUserInteractionsForComments(ctx context.Context, commentIDs []uint64, userID uint64) (map[uint64]bool, error)
	AddCommentInteraction(ctx context.Context, commentID, userID uint64, liked bool, ipAddress string) error
	AddReplyInteraction(ctx context.Context, replyID, userID uint64, liked bool, ipAddress string) error
	ReportComment(ctx context.Context, videoID, commentID, userID uint64, content string) error
}
