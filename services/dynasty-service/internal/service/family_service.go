package service

import (
	"context"
	"fmt"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
	levelspb "metarang/shared/pb/levels"
)

type FamilyService struct {
	familyRepo  *repository.FamilyRepository
	dynastyRepo *repository.DynastyRepository
	levels      userLevelPort
}

// userLevelPort fetches a user's latest achieved level from levels-service.
type userLevelPort interface {
	GetUserLevel(ctx context.Context, userID uint64) (*levelspb.UserLevelResponse, error)
}

func NewFamilyService(
	familyRepo *repository.FamilyRepository,
	dynastyRepo *repository.DynastyRepository,
	levels userLevelPort,
) *FamilyService {
	return &FamilyService{
		familyRepo:  familyRepo,
		dynastyRepo: dynastyRepo,
		levels:      levels,
	}
}

// GetFamily retrieves a family by ID or dynasty ID.
// When both IDs are provided, the family must belong to the given dynasty.
func (s *FamilyService) GetFamily(ctx context.Context, familyID, dynastyID uint64) (*models.Family, error) {
	if familyID == 0 && dynastyID == 0 {
		return nil, fmt.Errorf("either familyID or dynastyID must be provided")
	}

	var (
		family *models.Family
		err    error
	)

	switch {
	case familyID > 0:
		family, err = s.familyRepo.GetFamilyByID(ctx, familyID)
	case dynastyID > 0:
		family, err = s.familyRepo.GetFamilyByDynastyID(ctx, dynastyID)
	}

	if err != nil {
		return nil, err
	}
	if family == nil {
		return nil, fmt.Errorf("family not found")
	}
	if dynastyID > 0 && family.DynastyID != dynastyID {
		return nil, fmt.Errorf("family not found")
	}

	return family, nil
}

// GetFamilyMembers retrieves all members of a family
func (s *FamilyService) GetFamilyMembers(ctx context.Context, familyID uint64, page, perPage int32) ([]*models.FamilyMember, int32, error) {
	return s.familyRepo.GetFamilyMembers(ctx, familyID, page, perPage)
}

// GetUserBasicInfo retrieves basic user information, including the latest achieved level.
func (s *FamilyService) GetUserBasicInfo(ctx context.Context, userID uint64) (*models.UserBasic, error) {
	user, err := s.familyRepo.GetUserBasicInfo(ctx, userID)
	if err != nil || user == nil {
		return user, err
	}
	user.Level = s.latestLevelName(ctx, userID)
	return user, nil
}

func (s *FamilyService) latestLevelName(ctx context.Context, userID uint64) string {
	if s.levels == nil {
		return ""
	}
	resp, err := s.levels.GetUserLevel(ctx, userID)
	if err != nil || resp == nil || resp.LatestLevel == nil {
		return ""
	}
	return resp.LatestLevel.Name
}
