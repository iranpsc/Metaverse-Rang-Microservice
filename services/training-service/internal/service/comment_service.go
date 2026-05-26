package service

import (
	"context"
	"fmt"

	"metargb/shared/pkg/jalali"
	"metargb/training-service/internal/models"
	"metargb/training-service/internal/repository"
)

type CommentService struct {
	commentRepo repository.CommentRepositoryInterface
	userRepo    repository.UserRepositoryInterface
}

func NewCommentService(commentRepo repository.CommentRepositoryInterface, userRepo repository.UserRepositoryInterface) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		userRepo:    userRepo,
	}
}

// GetComments retrieves top-level comments for a video
func (s *CommentService) GetComments(ctx context.Context, videoID uint64, page, perPage int32, userID uint64) ([]*CommentDetails, int32, error) {
	comments, total, err := s.commentRepo.GetComments(ctx, videoID, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get comments: %w", err)
	}

	commentIDs := make([]uint64, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}

	var userInteractions map[uint64]bool
	if userID > 0 && len(commentIDs) > 0 {
		userInteractions, _ = s.commentRepo.GetUserInteractionsForComments(ctx, commentIDs, userID)
	}

	details := make([]*CommentDetails, 0, len(comments))
	for _, comment := range comments {
		detail, err := s.getCommentDetails(ctx, comment)
		if err != nil {
			continue // Skip comments with errors
		}
		if userInteractions != nil {
			if liked, ok := userInteractions[comment.ID]; ok {
				detail.UserInteraction = &liked
			}
		}
		details = append(details, detail)
	}

	return details, total, nil
}

// AddComment creates a new comment
func (s *CommentService) AddComment(ctx context.Context, videoID, userID uint64, content string) (*CommentDetails, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len(content) > 2000 {
		return nil, fmt.Errorf("content must be at most 2000 characters")
	}

	comment, err := s.commentRepo.AddComment(ctx, videoID, userID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	return s.getCommentDetails(ctx, comment)
}

// UpdateComment updates a comment
func (s *CommentService) UpdateComment(ctx context.Context, commentID, userID uint64, content string) (*CommentDetails, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len(content) > 2000 {
		return nil, fmt.Errorf("content must be at most 2000 characters")
	}

	// Verify ownership
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return nil, fmt.Errorf("comment not found")
	}
	if comment.UserID != userID {
		return nil, fmt.Errorf("%w: update comment", ErrNotAuthorized)
	}

	if err := s.commentRepo.UpdateComment(ctx, commentID, userID, content); err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	// Get updated comment
	updatedComment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated comment: %w", err)
	}

	return s.getCommentDetails(ctx, updatedComment)
}

// DeleteComment deletes a comment
func (s *CommentService) DeleteComment(ctx context.Context, commentID, userID uint64) error {
	return s.commentRepo.DeleteComment(ctx, commentID, userID)
}

// AddCommentInteraction adds or updates a user's interaction on a comment
func (s *CommentService) AddCommentInteraction(ctx context.Context, commentID, userID uint64, liked bool, ipAddress string) error {
	// Verify user is not the comment author
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID == userID {
		return fmt.Errorf("%w: react to own comment", ErrNotAuthorized)
	}

	return s.commentRepo.AddCommentInteraction(ctx, commentID, userID, liked, ipAddress)
}

// ReportComment creates a report for a comment
func (s *CommentService) ReportComment(ctx context.Context, videoID, commentID, userID uint64, content string) error {
	if content == "" {
		return fmt.Errorf("content is required")
	}
	if len(content) > 2000 {
		return fmt.Errorf("content must be at most 2000 characters")
	}

	// Verify user is not the comment author
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID == userID {
		return fmt.Errorf("%w: report own comment", ErrNotAuthorized)
	}

	return s.commentRepo.ReportComment(ctx, videoID, commentID, userID, content)
}

// GetCommentByID retrieves a comment by ID with details
func (s *CommentService) GetCommentByID(ctx context.Context, commentID uint64) (*CommentDetails, error) {
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return nil, fmt.Errorf("comment not found")
	}

	return s.getCommentDetails(ctx, comment)
}

// getCommentDetails enriches a comment with user info and stats
func (s *CommentService) getCommentDetails(ctx context.Context, comment *models.Comment) (*CommentDetails, error) {
	details := &CommentDetails{
		Comment: comment,
	}

	// Get user information
	// We need to get user by ID, but we only have user_id
	// For now, we'll query the users table directly
	user, err := s.userRepo.GetUserByID(ctx, comment.UserID)
	if err == nil && user != nil {
		details.User = user
	}

	// Get stats
	stats, err := s.commentRepo.GetCommentStats(ctx, comment.ID)
	if err == nil {
		details.Stats = stats
	}

	// Format dates as Jalali
	if !comment.CreatedAt.IsZero() {
		details.CreatedAtJalali = jalali.CarbonToJalali(comment.CreatedAt)
	}
	if !comment.UpdatedAt.IsZero() {
		details.UpdatedAtJalali = jalali.CarbonToJalali(comment.UpdatedAt)
	}

	return details, nil
}

// CommentDetails contains a comment with user info and stats
type CommentDetails struct {
	Comment         *models.Comment
	User            *repository.UserBasic
	Stats           *models.CommentStats
	CreatedAtJalali string
	UpdatedAtJalali string
	UserInteraction *bool // true=liked, false=disliked, nil=no interaction
}
