// Package models defines domain types for the training service.
package models

import "time"

// Video represents a tutorial video
type Video struct {
	ID                 uint64    `db:"id"`
	VideoSubCategoryID uint64    `db:"video_sub_category_id"`
	Title              string    `db:"title"`
	Slug               *string   `db:"slug"`
	Description        string    `db:"description"`
	FileName           string    `db:"fileName"`
	CreatorCode        string    `db:"creator_code"`
	Image              string    `db:"image"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

// VideoCategory represents a video category
type VideoCategory struct {
	ID          uint64    `db:"id"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description string    `db:"description"`
	Image       string    `db:"image"`
	Icon        *string   `db:"icon"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// VideoSubCategory represents a video subcategory
type VideoSubCategory struct {
	ID              uint64    `db:"id"`
	VideoCategoryID uint64    `db:"video_category_id"`
	Name            string    `db:"name"`
	Slug            string    `db:"slug"`
	Description     string    `db:"description"`
	Image           string    `db:"image"`
	Icon            *string   `db:"icon"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// Comment represents a comment on a video (polymorphic)
type Comment struct {
	ID              uint64    `db:"id"`
	UserID          uint64    `db:"user_id"`
	ParentID        *uint64   `db:"parent_id"`
	CommentableType string    `db:"commentable_type"`
	CommentableID   uint64    `db:"commentable_id"`
	Content         string    `db:"content"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// Interaction represents a like/dislike interaction (polymorphic)
type Interaction struct {
	ID           uint64    `db:"id"`
	UserID       uint64    `db:"user_id"`
	LikeableType string    `db:"likeable_type"`
	LikeableID   uint64    `db:"likeable_id"`
	Liked        bool      `db:"liked"` // true=like, false=dislike
	IPAddress    string    `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// View represents a view tracking entry (polymorphic)
type View struct {
	ID           uint64    `db:"id"`
	ViewableType string    `db:"viewable_type"`
	ViewableID   uint64    `db:"viewable_id"`
	IPAddress    string    `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// CommentReport represents a report on a comment
type CommentReport struct {
	ID              uint64    `db:"id"`
	UserID          uint64    `db:"user_id"`
	CommentableType string    `db:"commentable_type"`
	CommentableID   uint64    `db:"commentable_id"`
	CommentID       uint64    `db:"comment_id"`
	Content         string    `db:"content"`
	Status          int32     `db:"status"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// VideoStats holds aggregated statistics for a video
type VideoStats struct {
	ViewsCount    int32
	LikesCount    int32
	DislikesCount int32
	CommentsCount int32
}

// CommentStats holds aggregated statistics for a comment
type CommentStats struct {
	LikesCount    int32
	DislikesCount int32
	RepliesCount  int32
}

// CategoryStats holds aggregated statistics for a category
type CategoryStats struct {
	VideosCount   int32
	ViewsCount    int32
	LikesCount    int32
	DislikesCount int32
}

// SubCategoryStats holds aggregated statistics for a subcategory
type SubCategoryStats struct {
	VideosCount   int32
	ViewsCount    int32
	LikesCount    int32
	DislikesCount int32
}
