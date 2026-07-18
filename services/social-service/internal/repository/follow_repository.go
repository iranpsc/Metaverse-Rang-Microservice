package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FollowRepository interface {
	Create(ctx context.Context, followerID, followingID uint64) error
	Delete(ctx context.Context, followerID, followingID uint64) error
	Exists(ctx context.Context, followerID, followingID uint64) (bool, error)
	GetFollowers(ctx context.Context, userID uint64) ([]uint64, error)
	GetFollowing(ctx context.Context, userID uint64) ([]uint64, error)
}

type followRepository struct {
	db *sql.DB
}

func NewFollowRepository(db *sql.DB) FollowRepository {
	return &followRepository{db: db}
}

func (r *followRepository) Create(ctx context.Context, followerID, followingID uint64) error {
	query := `
		INSERT INTO follows (follower_id, following_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, followerID, followingID, now, now)
	if err != nil {
		return fmt.Errorf("failed to create follow relationship: %w", err)
	}
	return nil
}

func (r *followRepository) Delete(ctx context.Context, followerID, followingID uint64) error {
	query := `
		DELETE FROM follows
		WHERE follower_id = ? AND following_id = ?
	`
	_, err := r.db.ExecContext(ctx, query, followerID, followingID)
	if err != nil {
		return fmt.Errorf("failed to delete follow relationship: %w", err)
	}
	return nil
}

func (r *followRepository) Exists(ctx context.Context, followerID, followingID uint64) (bool, error) {
	query := `
		SELECT COUNT(*) FROM follows
		WHERE follower_id = ? AND following_id = ?
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, followerID, followingID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check follow relationship: %w", err)
	}
	return count > 0, nil
}

func (r *followRepository) GetFollowers(ctx context.Context, userID uint64) ([]uint64, error) {
	query := `
		SELECT follower_id FROM follows
		WHERE following_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var followers []uint64
	for rows.Next() {
		var followerID uint64
		if err := rows.Scan(&followerID); err != nil {
			return nil, fmt.Errorf("failed to scan follower: %w", err)
		}
		followers = append(followers, followerID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating followers: %w", err)
	}
	return followers, nil
}

func (r *followRepository) GetFollowing(ctx context.Context, userID uint64) ([]uint64, error) {
	query := `
		SELECT following_id FROM follows
		WHERE follower_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var following []uint64
	for rows.Next() {
		var followingID uint64
		if err := rows.Scan(&followingID); err != nil {
			return nil, fmt.Errorf("failed to scan following: %w", err)
		}
		following = append(following, followingID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating following: %w", err)
	}
	return following, nil
}
