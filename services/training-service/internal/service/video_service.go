package service

import (
	"context"
	"fmt"

	"metargb/shared/pkg/jalali"
	"metargb/training-service/internal/models"
	"metargb/training-service/internal/repository"
)

type VideoService struct {
	videoRepo    repository.VideoRepositoryInterface
	categoryRepo repository.CategoryRepositoryInterface
	userRepo     repository.UserRepositoryInterface
}

func NewVideoService(videoRepo repository.VideoRepositoryInterface, categoryRepo repository.CategoryRepositoryInterface, userRepo repository.UserRepositoryInterface) *VideoService {
	return &VideoService{
		videoRepo:    videoRepo,
		categoryRepo: categoryRepo,
		userRepo:     userRepo,
	}
}

// GetVideos retrieves paginated videos
func (s *VideoService) GetVideos(ctx context.Context, page, perPage int32, categoryID, subCategoryID *uint64) ([]*models.Video, int32, error) {
	return s.videoRepo.GetVideos(ctx, page, perPage, categoryID, subCategoryID)
}

// GetVideoBySlug retrieves a video by slug and increments view
func (s *VideoService) GetVideoBySlug(ctx context.Context, slug string, userID *uint64, ipAddress string) (*models.Video, error) {
	video, err := s.videoRepo.GetVideoBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	if video == nil {
		return nil, fmt.Errorf("video not found")
	}

	// Increment view
	if err := s.videoRepo.IncrementView(ctx, video.ID, ipAddress); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to increment view: %v\n", err)
	}

	return video, nil
}

// GetVideoByFileName retrieves a video by partial file name and increments view
func (s *VideoService) GetVideoByFileName(ctx context.Context, fileName string, ipAddress string) (*models.Video, error) {
	video, err := s.videoRepo.GetVideoByFileName(ctx, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	if video == nil {
		return nil, fmt.Errorf("video not found")
	}

	// Increment view
	if err := s.videoRepo.IncrementView(ctx, video.ID, ipAddress); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to increment view: %v\n", err)
	}

	return video, nil
}

// SearchVideos searches videos by title
func (s *VideoService) SearchVideos(ctx context.Context, searchTerm string, page, perPage int32) ([]*models.Video, int32, error) {
	if searchTerm == "" {
		return nil, 0, fmt.Errorf("search term is required")
	}
	return s.videoRepo.SearchVideos(ctx, searchTerm, page, perPage)
}

// GetVideoStats retrieves statistics for a video
func (s *VideoService) GetVideoStats(ctx context.Context, videoID uint64) (*models.VideoStats, error) {
	return s.videoRepo.GetVideoStats(ctx, videoID)
}

// AddInteraction adds or updates a user's interaction on a video
func (s *VideoService) AddInteraction(ctx context.Context, videoID, userID uint64, liked bool, ipAddress string) error {
	return s.videoRepo.AddInteraction(ctx, videoID, userID, liked, ipAddress)
}

// IncrementView increments view count for a video
func (s *VideoService) IncrementView(ctx context.Context, videoID uint64, ipAddress string) error {
	return s.videoRepo.IncrementView(ctx, videoID, ipAddress)
}

// GetVideoWithDetails retrieves a video with all related information (creator, category, stats)
func (s *VideoService) GetVideoWithDetails(ctx context.Context, video *models.Video, userID *uint64) (*VideoDetails, error) {
	details := &VideoDetails{
		Video: video,
	}

	// Get creator information
	if video.CreatorCode != "" {
		creator, err := s.userRepo.GetUserBasicByCode(ctx, video.CreatorCode)
		if err == nil && creator != nil {
			details.Creator = creator
		}
	}

	// Get subcategory and category
	subCategory, err := s.categoryRepo.GetSubCategoryByID(ctx, video.VideoSubCategoryID)
	if err == nil && subCategory != nil {
		details.SubCategory = subCategory
		// Get category
		category, err := s.categoryRepo.GetCategoryByID(ctx, subCategory.VideoCategoryID)
		if err == nil && category != nil {
			details.Category = category
		}
	}

	// Get stats
	stats, err := s.GetVideoStats(ctx, video.ID)
	if err == nil {
		details.Stats = stats
	}

	// Format created_at as Jalali
	if !video.CreatedAt.IsZero() {
		details.CreatedAtJalali = jalali.CarbonToJalali(video.CreatedAt)
	}

	if userID != nil && *userID > 0 {
		interaction, err := s.videoRepo.GetUserInteraction(ctx, video.ID, *userID)
		if err == nil {
			details.UserInteraction = interaction
		}
	}

	return details, nil
}

// VideoDetails contains a video with all related information
type VideoDetails struct {
	Video           *models.Video
	Creator         *repository.UserBasic
	Category        *models.VideoCategory
	SubCategory     *models.VideoSubCategory
	Stats           *models.VideoStats
	CreatedAtJalali string
	UserInteraction *bool // true=liked, false=disliked, nil=no interaction
}
