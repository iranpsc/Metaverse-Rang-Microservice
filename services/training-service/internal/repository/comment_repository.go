package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"metargb/training-service/internal/models"
)

// CommentRepositoryInterface defines persistence for comments, replies, reports, and interactions.
type CommentRepositoryInterface interface {
	GetComments(ctx context.Context, videoID uint64, page, perPage int32) ([]*models.Comment, int32, error)
	GetReplies(ctx context.Context, commentID uint64, page, perPage int32) ([]*models.Comment, int32, error)
	GetCommentByID(ctx context.Context, commentID uint64) (*models.Comment, error)
	AddComment(ctx context.Context, videoID, userID uint64, content string) (*models.Comment, error)
	UpdateComment(ctx context.Context, commentID, userID uint64, content string) error
	DeleteComment(ctx context.Context, commentID, userID uint64) error
	AddReply(ctx context.Context, parentCommentID, userID uint64, content string) (*models.Comment, error)
	UpdateReply(ctx context.Context, replyID, userID uint64, content string) error
	DeleteReply(ctx context.Context, replyID, userID uint64) error
	GetCommentStats(ctx context.Context, commentID uint64) (*models.CommentStats, error)
	AddCommentInteraction(ctx context.Context, commentID, userID uint64, liked bool, ipAddress string) error
	AddReplyInteraction(ctx context.Context, replyID, userID uint64, liked bool, ipAddress string) error
	ReportComment(ctx context.Context, videoID, commentID, userID uint64, content string) error
	GetUserInteractionsForComments(ctx context.Context, commentIDs []uint64, userID uint64) (map[uint64]bool, error)
}

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

// GetComments retrieves top-level comments for a video (parent_id IS NULL)
func (r *CommentRepository) GetComments(ctx context.Context, videoID uint64, page, perPage int32) ([]*models.Comment, int32, error) {
	query := `
		SELECT id, user_id, parent_id, commentable_type, commentable_id, content, created_at, updated_at
		FROM comments
		WHERE commentable_type = 'App\\Models\\Video' AND commentable_id = ? AND parent_id IS NULL
		ORDER BY (
			SELECT COUNT(*) FROM interactions
			WHERE likeable_type = 'App\\Models\\Comment' AND likeable_id = comments.id AND liked = 1
		) DESC
	`
	countQuery := `
		SELECT COUNT(*) 
		FROM comments
		WHERE commentable_type = 'App\\Models\\Video' AND commentable_id = ? AND parent_id IS NULL
	`

	// Get total count
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, videoID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count comments: %w", err)
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += " LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, videoID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get comments: %w", err)
	}
	defer rows.Close()

	var comments []*models.Comment
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(
			&comment.ID,
			&comment.UserID,
			&comment.ParentID,
			&comment.CommentableType,
			&comment.CommentableID,
			&comment.Content,
			&comment.CreatedAt,
			&comment.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan comment: %w", err)
		}
		comments = append(comments, &comment)
	}

	return comments, total, nil
}

// GetCommentByID retrieves a comment by ID
func (r *CommentRepository) GetCommentByID(ctx context.Context, commentID uint64) (*models.Comment, error) {
	query := `
		SELECT id, user_id, parent_id, commentable_type, commentable_id, content, created_at, updated_at
		FROM comments
		WHERE id = ?
	`

	var comment models.Comment
	err := r.db.QueryRowContext(ctx, query, commentID).Scan(
		&comment.ID,
		&comment.UserID,
		&comment.ParentID,
		&comment.CommentableType,
		&comment.CommentableID,
		&comment.Content,
		&comment.CreatedAt,
		&comment.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	return &comment, nil
}

// AddComment creates a new comment
func (r *CommentRepository) AddComment(ctx context.Context, videoID, userID uint64, content string) (*models.Comment, error) {
	query := `
		INSERT INTO comments (user_id, parent_id, commentable_type, commentable_id, content, created_at, updated_at)
		VALUES (?, NULL, 'App\\Models\\Video', ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, userID, videoID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get comment ID: %w", err)
	}

	return r.GetCommentByID(ctx, uint64(id))
}

// UpdateComment updates a comment's content
func (r *CommentRepository) UpdateComment(ctx context.Context, commentID, userID uint64, content string) error {
	query := `
		UPDATE comments
		SET content = ?, updated_at = NOW()
		WHERE id = ? AND user_id = ?
	`

	result, err := r.db.ExecContext(ctx, query, content, commentID, userID)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("comment not found or user not authorized")
	}

	return nil
}

// DeleteComment deletes a comment and its interactions
func (r *CommentRepository) DeleteComment(ctx context.Context, commentID, userID uint64) error {
	// First verify ownership
	comment, err := r.GetCommentByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}
	if comment.UserID != userID {
		return fmt.Errorf("user not authorized to delete this comment")
	}

	// Delete interactions first (cascade should handle this, but being explicit)
	_, err = r.db.ExecContext(ctx, `
		DELETE FROM interactions 
		WHERE likeable_type = 'App\\Models\\Comment' AND likeable_id = ?
	`, commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment interactions: %w", err)
	}

	// Delete the comment (cascade will delete replies)
	_, err = r.db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// GetReplies retrieves replies for a comment
func (r *CommentRepository) GetReplies(ctx context.Context, commentID uint64, page, perPage int32) ([]*models.Comment, int32, error) {
	query := `
		SELECT id, user_id, parent_id, commentable_type, commentable_id, content, created_at, updated_at
		FROM comments
		WHERE parent_id = ?
		ORDER BY created_at ASC
	`
	countQuery := "SELECT COUNT(*) FROM comments WHERE parent_id = ?"

	// Get total count
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, commentID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count replies: %w", err)
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += " LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, commentID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get replies: %w", err)
	}
	defer rows.Close()

	var replies []*models.Comment
	for rows.Next() {
		var reply models.Comment
		if err := rows.Scan(
			&reply.ID,
			&reply.UserID,
			&reply.ParentID,
			&reply.CommentableType,
			&reply.CommentableID,
			&reply.Content,
			&reply.CreatedAt,
			&reply.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan reply: %w", err)
		}
		replies = append(replies, &reply)
	}

	return replies, total, nil
}

// AddReply creates a new reply to a comment
// Note: Replies are always attached to the top-level parent comment
func (r *CommentRepository) AddReply(ctx context.Context, parentCommentID, userID uint64, content string) (*models.Comment, error) {
	// Get the parent comment to find the top-level parent
	parentComment, err := r.GetCommentByID(ctx, parentCommentID)
	if err != nil {
		return nil, err
	}
	if parentComment == nil {
		return nil, fmt.Errorf("parent comment not found")
	}

	// Find the top-level parent (if parent has a parent, use that)
	topLevelParentID := parentCommentID
	if parentComment.ParentID != nil {
		topLevelParentID = *parentComment.ParentID
	}

	// Get the video ID from the parent comment
	videoID := parentComment.CommentableID

	// Create reply attached to top-level parent
	query := `
		INSERT INTO comments (user_id, parent_id, commentable_type, commentable_id, content, created_at, updated_at)
		VALUES (?, ?, 'App\\Models\\Video', ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, userID, topLevelParentID, videoID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to add reply: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get reply ID: %w", err)
	}

	return r.GetCommentByID(ctx, uint64(id))
}

// UpdateReply updates a reply's content
func (r *CommentRepository) UpdateReply(ctx context.Context, replyID, userID uint64, content string) error {
	return r.UpdateComment(ctx, replyID, userID, content)
}

// DeleteReply deletes a reply
func (r *CommentRepository) DeleteReply(ctx context.Context, replyID, userID uint64) error {
	return r.DeleteComment(ctx, replyID, userID)
}

// GetCommentStats retrieves statistics for a comment
func (r *CommentRepository) GetCommentStats(ctx context.Context, commentID uint64) (*models.CommentStats, error) {
	stats := &models.CommentStats{}

	// Get likes count
	likeQuery := "SELECT COUNT(*) FROM interactions WHERE likeable_type = 'App\\Models\\Comment' AND likeable_id = ? AND liked = 1"
	r.db.QueryRowContext(ctx, likeQuery, commentID).Scan(&stats.LikesCount)

	// Get dislikes count
	dislikeQuery := "SELECT COUNT(*) FROM interactions WHERE likeable_type = 'App\\Models\\Comment' AND likeable_id = ? AND liked = 0"
	r.db.QueryRowContext(ctx, dislikeQuery, commentID).Scan(&stats.DislikesCount)

	// Get replies count
	replyQuery := "SELECT COUNT(*) FROM comments WHERE parent_id = ?"
	r.db.QueryRowContext(ctx, replyQuery, commentID).Scan(&stats.RepliesCount)

	return stats, nil
}

// GetUserInteractionsForComments returns liked state per comment for the given user.
func (r *CommentRepository) GetUserInteractionsForComments(ctx context.Context, commentIDs []uint64, userID uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool)
	if len(commentIDs) == 0 || userID == 0 {
		return result, nil
	}

	placeholders := make([]string, len(commentIDs))
	args := make([]interface{}, 0, len(commentIDs)+1)
	args = append(args, userID)
	for i, id := range commentIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT likeable_id, liked
		FROM interactions
		WHERE likeable_type = 'App\\Models\\Comment'
		  AND user_id = ?
		  AND likeable_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment interactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var commentID uint64
		var liked bool
		if err := rows.Scan(&commentID, &liked); err != nil {
			return nil, fmt.Errorf("failed to scan comment interaction: %w", err)
		}
		result[commentID] = liked
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate comment interactions: %w", err)
	}

	return result, nil
}

// AddCommentInteraction adds or updates a user's interaction on a comment
func (r *CommentRepository) AddCommentInteraction(ctx context.Context, commentID, userID uint64, liked bool, ipAddress string) error {
	query := `
		INSERT INTO interactions (likeable_type, likeable_id, user_id, liked, ip_address, created_at, updated_at) 
		VALUES ('App\\Models\\Comment', ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE liked = ?, ip_address = ?, updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, commentID, userID, liked, ipAddress, liked, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to add comment interaction: %w", err)
	}

	return nil
}

// AddReplyInteraction adds or updates a user's interaction on a reply
func (r *CommentRepository) AddReplyInteraction(ctx context.Context, replyID, userID uint64, liked bool, ipAddress string) error {
	return r.AddCommentInteraction(ctx, replyID, userID, liked, ipAddress)
}

// ReportComment creates a report for a comment
func (r *CommentRepository) ReportComment(ctx context.Context, videoID, commentID, userID uint64, content string) error {
	query := `
		INSERT INTO comment_reports (user_id, commentable_type, commentable_id, comment_id, content, status, created_at, updated_at)
		VALUES (?, 'App\\Models\\Video', ?, ?, ?, 0, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, userID, videoID, commentID, content)
	if err != nil {
		return fmt.Errorf("failed to report comment: %w", err)
	}

	return nil
}
