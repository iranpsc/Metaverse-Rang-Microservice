package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/repository"
)

const (
	citizenPositionValue = "مدیریت موازی"
	citizenAvatarURL     = "https://irpsc.com/gb.glb"
)

type CitizenService interface {
	GetCitizenProfile(ctx context.Context, code string) (*models.CitizenProfile, error)
	GetCitizenReferrals(ctx context.Context, code string, search string, page int32) ([]*models.CitizenReferral, *models.PaginationMeta, error)
	GetCitizenReferralChart(ctx context.Context, code string, rangeType string) (*models.ReferralChartData, error)
	ScorePercentageToNextLevel(ctx context.Context, userID uint64, score int32) float64
	AbsoluteURL(path string) string
	PassionIconURL(passion string) string
	NationalityFlagURL() string
	CitizenPosition() string
	CitizenAvatar() string
}

type citizenService struct {
	citizenRepo repository.CitizenRepository
	userRepo    repository.UserRepository
	helperSvc   HelperService
	appURL      string
}

func NewCitizenService(
	citizenRepo repository.CitizenRepository,
	userRepo repository.UserRepository,
	helperSvc HelperService,
	appURL string,
) CitizenService {
	return &citizenService{
		citizenRepo: citizenRepo,
		userRepo:    userRepo,
		helperSvc:   helperSvc,
		appURL:      strings.TrimSuffix(appURL, "/"),
	}
}

// GetCitizenProfile retrieves a citizen's public profile (privacy applied in handler).
func (s *citizenService) GetCitizenProfile(ctx context.Context, code string) (*models.CitizenProfile, error) {
	profile, err := s.citizenRepo.GetCitizenByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to get citizen profile: %w", err)
	}
	if profile == nil {
		return nil, nil
	}

	if s.checkPrivacy(profile.Privacy, "level") {
		currentLevel, err := s.userRepo.GetUserLatestLevel(ctx, profile.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current level: %w", err)
		}
		if currentLevel != nil {
			profile.CurrentLevel = &models.CitizenLevel{
				ID:    currentLevel.ID,
				Name:  currentLevel.Name,
				Slug:  currentLevel.Slug,
				Score: currentLevel.Score,
				Image: currentLevel.Image,
			}

			subLevels, err := s.userRepo.GetLevelsBelowScore(ctx, currentLevel.Score)
			if err != nil {
				return nil, fmt.Errorf("failed to get achieved levels: %w", err)
			}
			for _, level := range subLevels {
				profile.AchievedLevels = append(profile.AchievedLevels, &models.CitizenLevel{
					ID:    level.ID,
					Name:  level.Name,
					Slug:  level.Slug,
					Score: level.Score,
					Image: level.Image,
				})
			}
		}
	}

	return profile, nil
}

// GetCitizenReferrals retrieves referrals for a citizen with pagination and search
func (s *citizenService) GetCitizenReferrals(ctx context.Context, code string, search string, page int32) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	user, err := s.userRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, nil, nil
	}

	referrals, meta, err := s.citizenRepo.GetCitizenReferrals(ctx, user.ID, search, int(page), 10)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get referrals: %w", err)
	}

	for _, referral := range referrals {
		orders, err := s.citizenRepo.GetCitizenReferralOrders(ctx, referral.ID)
		if err != nil || orders == nil {
			referral.ReferrerOrders = []*models.ReferrerOrder{}
			continue
		}
		referral.ReferrerOrders = orders
	}

	return referrals, meta, nil
}

// GetCitizenReferralChart retrieves referral chart data for a citizen
func (s *citizenService) GetCitizenReferralChart(ctx context.Context, code string, rangeType string) (*models.ReferralChartData, error) {
	user, err := s.userRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, nil
	}

	if rangeType == "" {
		rangeType = "daily"
	}
	rangeType = strings.ToLower(rangeType)
	if rangeType != "daily" && rangeType != "weekly" && rangeType != "monthly" && rangeType != "yearly" {
		rangeType = "daily"
	}

	chartData, err := s.citizenRepo.GetCitizenReferralChartData(ctx, user.ID, rangeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get chart data: %w", err)
	}

	return chartData, nil
}

// ScorePercentageToNextLevel returns Laravel getScorePercentageToNextLevel for a citizen.
func (s *citizenService) ScorePercentageToNextLevel(ctx context.Context, userID uint64, score int32) float64 {
	if s.helperSvc == nil {
		return 0
	}
	pct, err := s.helperSvc.GetScorePercentageToNextLevel(ctx, userID, score)
	if err != nil {
		return 0
	}
	return pct
}

// AbsoluteURL prepends APP_URL to a relative path (Laravel url() helper).
func (s *citizenService) AbsoluteURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	path = strings.TrimPrefix(path, "/")
	if s.appURL == "" {
		return "/" + path
	}
	return s.appURL + "/" + path
}

// PassionIconURL returns the Laravel favorites icon URL for an enabled passion.
func (s *citizenService) PassionIconURL(passion string) string {
	return s.AbsoluteURL("uploads/favorites/" + passion + ".png")
}

// NationalityFlagURL returns the Laravel nationality flag URL.
func (s *citizenService) NationalityFlagURL() string {
	return s.AbsoluteURL("uploads/flags/iran.svg")
}

// CitizenPosition returns the hardcoded position when privacy allows.
func (s *citizenService) CitizenPosition() string {
	return citizenPositionValue
}

// CitizenAvatar returns the hardcoded 3D avatar URL when privacy allows.
func (s *citizenService) CitizenAvatar() string {
	return citizenAvatarURL
}

// checkPrivacy mirrors Laravel PersonalInfo::checkFilter — default hidden.
func (s *citizenService) checkPrivacy(privacy map[string]bool, field string) bool {
	if privacy == nil {
		return false
	}
	return privacy[field]
}

// FormatRegisteredAt formats email_verified_at as Jalali Y/m/d (Laravel PersonalInfo).
func FormatRegisteredAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return FormatJalaliDate(t)
}
