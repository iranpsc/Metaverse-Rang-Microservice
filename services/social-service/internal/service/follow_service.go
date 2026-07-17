package service

import (
	"context"
	"errors"
	"fmt"

	"metarang/social-service/internal/client"
	"metarang/social-service/internal/models"
	"metarang/social-service/internal/repository"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrCannotFollowSelf  = errors.New("cannot follow yourself")
	ErrAlreadyFollowing  = errors.New("already following this user")
	ErrNotFollowing      = errors.New("not following this user")
	ErrProfileLimitation = errors.New("profile limitation prevents following")
)

type FollowService interface {
	GetFollowers(ctx context.Context, userID uint64) ([]*models.FollowResource, error)
	GetFollowing(ctx context.Context, userID uint64) ([]*models.FollowResource, error)
	Follow(ctx context.Context, userID, targetUserID uint64) error
	Unfollow(ctx context.Context, userID, targetUserID uint64) error
	Remove(ctx context.Context, userID, targetUserID uint64) error
}

type followService struct {
	followRepo   repository.FollowRepository
	userRepo     repository.UserRepository
	authClient   client.AuthClient
	levelsClient client.LevelsClient // optional; nil disables follower score updates
}

func NewFollowService(
	followRepo repository.FollowRepository,
	userRepo repository.UserRepository,
	authClient client.AuthClient,
	levelsClient client.LevelsClient,
) FollowService {
	return &followService{
		followRepo:   followRepo,
		userRepo:     userRepo,
		authClient:   authClient,
		levelsClient: levelsClient,
	}
}

func (s *followService) GetFollowers(ctx context.Context, userID uint64) ([]*models.FollowResource, error) {
	followerIDs, err := s.followRepo.GetFollowers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}

	resources := make([]*models.FollowResource, 0, len(followerIDs))
	for _, followerID := range followerIDs {
		resource, err := s.buildFollowResource(ctx, userID, followerID)
		if err != nil {
			// Log error but continue with other followers
			fmt.Printf("failed to build follow resource for user %d: %v\n", followerID, err)
			continue
		}
		if resource != nil {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *followService) GetFollowing(ctx context.Context, userID uint64) ([]*models.FollowResource, error) {
	followingIDs, err := s.followRepo.GetFollowing(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}

	resources := make([]*models.FollowResource, 0, len(followingIDs))
	for _, followingID := range followingIDs {
		resource, err := s.buildFollowResource(ctx, userID, followingID)
		if err != nil {
			// Log error but continue with other following
			fmt.Printf("failed to build follow resource for user %d: %v\n", followingID, err)
			continue
		}
		if resource != nil {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *followService) Follow(ctx context.Context, userID, targetUserID uint64) error {
	// Check if trying to follow self
	if userID == targetUserID {
		return ErrCannotFollowSelf
	}

	// Check if already following
	exists, err := s.followRepo.Exists(ctx, userID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check follow relationship: %w", err)
	}
	if exists {
		return ErrAlreadyFollowing
	}

	if s.authClient == nil {
		return fmt.Errorf("auth service client is not configured")
	}
	canFollow, err := s.authClient.CanFollow(ctx, userID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check profile limitation: %w", err)
	}
	if !canFollow {
		return ErrProfileLimitation
	}

	// Create follow relationship
	if err := s.followRepo.Create(ctx, userID, targetUserID); err != nil {
		return fmt.Errorf("failed to create follow relationship: %w", err)
	}

	// Laravel fires User::followed on the followed user (UserObserver::followed):
	// levels-service updates their followers_count log and recalculates score.
	// Best-effort: a levels-service outage must not fail the follow itself.
	if s.levelsClient != nil {
		if err := s.levelsClient.RecordFollower(ctx, targetUserID); err != nil {
			fmt.Printf("failed to record follower for user %d: %v\n", targetUserID, err)
		}
	}

	return nil
}

func (s *followService) Unfollow(ctx context.Context, userID, targetUserID uint64) error {
	// Delete follow relationship (idempotent - no error if doesn't exist)
	return s.followRepo.Delete(ctx, userID, targetUserID)
}

func (s *followService) Remove(ctx context.Context, userID, targetUserID uint64) error {
	// Remove target from user's followers (reverse of unfollow)
	// This removes the relationship where targetUserID is following userID
	return s.followRepo.Delete(ctx, targetUserID, userID)
}

// buildFollowResource builds the resource for userID as seen by viewerID (the
// authenticated user), mirroring Laravel FollowResource.
func (s *followService) buildFollowResource(ctx context.Context, viewerID, userID uint64) (*models.FollowResource, error) {
	// Get user basic info
	userInfo, err := s.userRepo.GetUserBasicInfo(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	if userInfo == nil {
		return nil, nil
	}

	// Get profile photos
	photos, err := s.userRepo.GetProfilePhotos(ctx, userID)
	if err != nil {
		// Log but continue - photos are optional
		fmt.Printf("failed to get profile photos for user %d: %v\n", userID, err)
		photos = []string{}
	}

	// Get level
	level, err := s.userRepo.GetUserLevel(ctx, userID)
	if err != nil {
		// Log but continue - level is optional
		fmt.Printf("failed to get level for user %d: %v\n", userID, err)
		level = ""
	}

	// Check if online
	online, err := s.userRepo.IsUserOnline(ctx, userID)
	if err != nil {
		// Log but continue - online status is optional
		fmt.Printf("failed to check online status for user %d: %v\n", userID, err)
		online = false
	}

	// Does the viewer follow this user?
	isFollowing, err := s.followRepo.Exists(ctx, viewerID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check following status: %w", err)
	}

	// Does this user follow the viewer? (viewer may remove them as follower)
	isFollower, err := s.followRepo.Exists(ctx, userID, viewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check follower status: %w", err)
	}

	isSelf := viewerID == userID

	return &models.FollowResource{
		ID:            userInfo.ID,
		Name:          userInfo.Name,
		Code:          userInfo.Code,
		ProfilePhotos: photos,
		Level:         level,
		Online:        online,
		Followed:      isFollowing,
		Can: models.FollowPermissions{
			Follow:         !isSelf && !isFollowing,
			Unfollow:       isFollowing,
			RemoveFollower: isFollower,
		},
	}, nil
}
