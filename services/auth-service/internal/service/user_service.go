package service

import (
	"context"
	"database/sql"
	"fmt"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
	"metargb/shared/pkg/helpers"
)

type UserService interface {
	GetUser(ctx context.Context, userID uint64) (*models.User, error)
	UpdateProfile(ctx context.Context, userID uint64, name, email, phone string) (*models.User, error)
	// Users API methods
	ListUsers(ctx context.Context, search string, orderBy string, page int32) ([]*UserListItem, int32, int32, error)
	GetUserLevels(ctx context.Context, userID uint64) (*UserLevelsData, error)
	GetUserProfile(ctx context.Context, userID uint64, viewerUserID *uint64) (*UserProfileData, error)
	GetUserFeaturesCount(ctx context.Context, userID uint64) (*UserFeaturesCountData, error)
}

type userService struct {
	userRepo         repository.UserRepository
	kycRepo          repository.KYCRepository
	settingsRepo     repository.SettingsRepository
	profilePhotoRepo repository.ProfilePhotoRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

// NewUserServiceWithDependencies creates a user service with all dependencies
func NewUserServiceWithDependencies(
	userRepo repository.UserRepository,
	kycRepo repository.KYCRepository,
	settingsRepo repository.SettingsRepository,
	profilePhotoRepo repository.ProfilePhotoRepository,
) UserService {
	return &userService{
		userRepo:         userRepo,
		kycRepo:          kycRepo,
		settingsRepo:     settingsRepo,
		profilePhotoRepo: profilePhotoRepo,
	}
}

// UserListItem represents a user in the list (Laravel UserResource)
type UserListItem struct {
	ID             uint64
	Name           string
	Code           string
	Score          int32
	CurrentLevel   *LevelSummary
	PreviousLevels []*LevelSummary
	ProfilePhoto   string
}

// LevelSummary represents level information for list/cards
type LevelSummary struct {
	ID    uint64
	Name  string
	Slug  string
	Score int32
	Image string
}

// UserLevelsData represents user level ladder data
type UserLevelsData struct {
	LatestLevel                *LevelDetail
	PreviousLevels             []*LevelDetail
	ScorePercentageToNextLevel float64
}

// LevelDetail represents detailed level information
type LevelDetail struct {
	ID    uint64
	Name  string
	Score int32
	Slug  string
	Image string
}

// UserProfileData represents user profile information
type UserProfileData struct {
	ID             uint64
	Name           *string // nil if privacy disallows
	Code           string
	RegisteredAt   *string // Jalali format, nil if privacy disallows
	ProfileImages  []string
	FollowersCount *int32 // nil if privacy disallows
	FollowingCount *int32 // nil if privacy disallows
}

// UserFeaturesCountData represents feature counts by category
type UserFeaturesCountData struct {
	MaskoniFeaturesCount   int32 // karbari = 'm'
	TejariFeaturesCount    int32 // karbari = 't'
	AmoozeshiFeaturesCount int32 // karbari = 'a'
}

func (s *userService) GetUser(ctx context.Context, userID uint64) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID uint64, name, email, phone string) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	user.Name = name
	user.Email = email
	user.Phone = sql.NullString{String: phone, Valid: phone != ""}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// ListUsers returns paginated list of users
func (s *userService) ListUsers(ctx context.Context, search string, orderBy string, page int32) ([]*UserListItem, int32, int32, error) {
	if page < 1 {
		page = 1
	}
	limit := int32(20) // Default pagination size per API spec

	users, totalCount, err := s.userRepo.ListUsers(ctx, search, orderBy, page, limit)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to list users: %w", err)
	}

	userIDs := make([]uint64, 0, len(users))
	for _, ur := range users {
		userIDs = append(userIDs, ur.User.ID)
	}

	levelsByUser, err := s.userRepo.GetUsersLevelsForList(ctx, userIDs)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to load user levels: %w", err)
	}

	result := make([]*UserListItem, 0, len(users))
	for _, ur := range users {
		item := &UserListItem{
			ID:    ur.User.ID,
			Code:  ur.User.Code,
			Score: ur.User.Score,
		}

		if ur.KYCName != nil {
			item.Name = *ur.KYCName
		} else {
			item.Name = ur.User.Name
		}

		if ur.ProfilePhotoURL != nil {
			item.ProfilePhoto = *ur.ProfilePhotoURL
		}

		if bundle := levelsByUser[ur.User.ID]; bundle != nil {
			if bundle.Current != nil {
				item.CurrentLevel = levelSummaryFromRepo(bundle.Current)
			}
			for _, lvl := range bundle.Previous {
				item.PreviousLevels = append(item.PreviousLevels, levelSummaryFromRepo(lvl))
			}
		}

		result = append(result, item)
	}

	return result, totalCount, limit, nil
}

func levelSummaryFromRepo(lvl *repository.UserListLevel) *LevelSummary {
	if lvl == nil {
		return nil
	}
	return &LevelSummary{
		ID:    lvl.ID,
		Name:  lvl.Name,
		Slug:  lvl.Slug,
		Score: lvl.Score,
		Image: lvl.Image,
	}
}

// GetUserLevels returns user's level ladder data
func (s *userService) GetUserLevels(ctx context.Context, userID uint64) (*UserLevelsData, error) {
	// Verify user exists
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get latest level
	latestLevelDB, err := s.userRepo.GetUserLatestLevel(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest level: %w", err)
	}

	var latestLevel *LevelDetail
	var previousLevels []*LevelDetail

	if latestLevelDB != nil {
		latestLevel = &LevelDetail{
			ID:    latestLevelDB.ID,
			Name:  latestLevelDB.Name,
			Score: latestLevelDB.Score,
			Slug:  latestLevelDB.Slug,
			Image: latestLevelDB.Image,
		}

		// Get previous levels
		prevLevelsDB, err := s.userRepo.GetLevelsBelowScore(ctx, latestLevelDB.Score)
		if err != nil {
			return nil, fmt.Errorf("failed to get previous levels: %w", err)
		}

		for _, pl := range prevLevelsDB {
			previousLevels = append(previousLevels, &LevelDetail{
				ID:    pl.ID,
				Name:  pl.Name,
				Score: pl.Score,
				Slug:  pl.Slug,
				Image: pl.Image,
			})
		}
	}

	// Calculate score percentage to next level
	scorePercentage := float64(0)
	if latestLevel != nil && user.Score > 0 {
		nextScore, err := s.userRepo.GetNextLevelScore(ctx, latestLevel.Score)
		if err == nil && nextScore > 0 {
			scorePercentage = float64(user.Score) / float64(nextScore) * 100
		}
	}

	return &UserLevelsData{
		LatestLevel:                latestLevel,
		PreviousLevels:             previousLevels,
		ScorePercentageToNextLevel: scorePercentage,
	}, nil
}

// GetUserProfile returns user profile with privacy filtering
func (s *userService) GetUserProfile(ctx context.Context, userID uint64, viewerUserID *uint64) (*UserProfileData, error) {
	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get settings for privacy checking (if settingsRepo is available)
	var settings *models.Settings
	if s.settingsRepo != nil {
		settings, err = s.settingsRepo.FindByUserID(ctx, userID)
		if err != nil {
			// Settings not found is OK, just use defaults
			settings = nil
		}
	}

	// Check if viewer is the owner
	isOwner := viewerUserID != nil && *viewerUserID == userID

	profile := &UserProfileData{
		ID:   user.ID,
		Code: user.Code,
	}

	// Get profile images
	photoURLs, err := s.userRepo.GetAllProfilePhotoURLs(ctx, userID)
	if err == nil {
		profile.ProfileImages = photoURLs
	}

	// Privacy filtering - check each field
	// Name: check privacy['name']
	if isOwner || (settings != nil && settings.Privacy != nil && settings.Privacy["name"] == 1) {
		// Prefer KYC name if available
		if s.kycRepo != nil {
			kyc, err := s.kycRepo.FindByUserID(ctx, userID)
			if err == nil && kyc != nil && kyc.Approved() {
				fullName := kyc.FullName()
				profile.Name = &fullName
			} else {
				profile.Name = &user.Name
			}
		} else {
			profile.Name = &user.Name
		}
	}

	// Registered at: check privacy['registered_at']
	if isOwner || (settings != nil && settings.Privacy != nil && settings.Privacy["registered_at"] == 1) {
		registeredAt := helpers.FormatJalaliDate(user.CreatedAt)
		profile.RegisteredAt = &registeredAt
	}

	// Followers count: check privacy['followers_count']
	if isOwner || (settings != nil && settings.Privacy != nil && settings.Privacy["followers_count"] == 1) {
		count, err := s.userRepo.GetFollowersCount(ctx, userID)
		if err == nil {
			profile.FollowersCount = &count
		}
	}

	// Following count: check privacy['following_count']
	if isOwner || (settings != nil && settings.Privacy != nil && settings.Privacy["following_count"] == 1) {
		count, err := s.userRepo.GetFollowingCount(ctx, userID)
		if err == nil {
			profile.FollowingCount = &count
		}
	}

	return profile, nil
}

// GetUserFeaturesCount returns feature counts by category
func (s *userService) GetUserFeaturesCount(ctx context.Context, userID uint64) (*UserFeaturesCountData, error) {
	// Verify user exists
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	maskoni, tejari, amoozeshi, err := s.userRepo.GetFeatureCounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get feature counts: %w", err)
	}

	return &UserFeaturesCountData{
		MaskoniFeaturesCount:   maskoni,
		TejariFeaturesCount:    tejari,
		AmoozeshiFeaturesCount: amoozeshi,
	}, nil
}
