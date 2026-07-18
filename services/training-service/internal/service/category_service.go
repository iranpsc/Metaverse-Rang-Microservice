// Package service implements business logic for the training service.
package service

import (
	"context"
	"fmt"

	"metarang/training-service/internal/models"
	"metarang/training-service/internal/repository"
)

type CategoryService struct {
	categoryRepo repository.CategoryRepositoryInterface
	videoRepo    repository.VideoRepositoryInterface
}

func NewCategoryService(categoryRepo repository.CategoryRepositoryInterface, videoRepo repository.VideoRepositoryInterface) *CategoryService {
	return &CategoryService{
		categoryRepo: categoryRepo,
		videoRepo:    videoRepo,
	}
}

// GetCategories retrieves paginated categories with stats
func (s *CategoryService) GetCategories(ctx context.Context, page, perPage int32) ([]*models.VideoCategory, int32, error) {
	return s.categoryRepo.GetCategories(ctx, page, perPage)
}

// GetCategoryBySlug retrieves a category by slug with subcategories and stats
func (s *CategoryService) GetCategoryBySlug(ctx context.Context, slug string) (*CategoryDetails, error) {
	category, err := s.categoryRepo.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	if category == nil {
		return nil, fmt.Errorf("category not found")
	}

	details := &CategoryDetails{
		Category: category,
	}

	// Get subcategories with stats
	subCategories, err := s.categoryRepo.GetSubCategoriesByCategoryID(ctx, category.ID)
	if err == nil {
		details.SubCategories = subCategories
		// Get stats for each subcategory
		subCategoryStats, err := s.categoryRepo.GetSubCategoryStatsByCategoryID(ctx, category.ID)
		if err == nil {
			details.SubCategoryStats = subCategoryStats
		}
	}

	// Get category stats
	stats, err := s.categoryRepo.GetCategoryStats(ctx, category.ID)
	if err == nil {
		details.Stats = stats
	}

	return details, nil
}

// GetSubCategoryBySlugs retrieves a subcategory by slugs with category and stats
func (s *CategoryService) GetSubCategoryBySlugs(ctx context.Context, categorySlug, subCategorySlug string) (*SubCategoryDetails, error) {
	subCategory, err := s.categoryRepo.GetSubCategoryBySlugs(ctx, categorySlug, subCategorySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get subcategory: %w", err)
	}
	if subCategory == nil {
		return nil, fmt.Errorf("subcategory not found")
	}

	details := &SubCategoryDetails{
		SubCategory: subCategory,
	}

	// Get category
	category, err := s.categoryRepo.GetCategoryByID(ctx, subCategory.VideoCategoryID)
	if err == nil && category != nil {
		details.Category = category
	}

	// Get subcategory stats
	stats, err := s.categoryRepo.GetSubCategoryStats(ctx, subCategory.ID)
	if err == nil {
		details.Stats = stats
	}

	return details, nil
}

// GetCategoryVideos retrieves videos for a category
func (s *CategoryService) GetCategoryVideos(ctx context.Context, categorySlug string, page, perPage int32) ([]*models.Video, int32, error) {
	category, err := s.categoryRepo.GetCategoryBySlug(ctx, categorySlug)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get category: %w", err)
	}
	if category == nil {
		return nil, 0, fmt.Errorf("category not found")
	}

	categoryID := category.ID
	return s.videoRepo.GetVideos(ctx, page, perPage, &categoryID, nil)
}

// CategoryDetails contains a category with subcategories and stats
type CategoryDetails struct {
	Category         *models.VideoCategory
	SubCategories    []*models.VideoSubCategory
	SubCategoryStats map[uint64]*models.SubCategoryStats
	Stats            *models.CategoryStats
}

// SubCategoryDetails contains a subcategory with category and stats
type SubCategoryDetails struct {
	SubCategory *models.VideoSubCategory
	Category    *models.VideoCategory
	Stats       *models.SubCategoryStats
}

// GetCategoryStats retrieves statistics for a category
func (s *CategoryService) GetCategoryStats(ctx context.Context, categoryID uint64) (*models.CategoryStats, error) {
	return s.categoryRepo.GetCategoryStats(ctx, categoryID)
}
