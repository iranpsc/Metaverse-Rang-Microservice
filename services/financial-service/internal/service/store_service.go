package service

import (
	"context"
	"errors"
	"fmt"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/repository"
)

var (
	ErrInvalidCodes      = errors.New("codes must be an array with at least 2 items")
	ErrInvalidCodeLength = errors.New("each code must be at least 2 characters")
)

type StoreService interface {
	GetStorePackages(ctx context.Context, codes []string) ([]*PackageResource, error)
}

type PackageResource struct {
	ID        uint64  `json:"id"`
	Code      string  `json:"code"`
	Asset     string  `json:"asset"`
	Amount    float64 `json:"amount"`
	UnitPrice float64 `json:"unitPrice"`
	Image     *string `json:"image"` // null if no image
}

type storeService struct {
	optionRepo   repository.OptionRepository
	variableRepo repository.VariableRepository
	imageRepo    repository.ImageRepository
}

func NewStoreService(
	optionRepo repository.OptionRepository,
	variableRepo repository.VariableRepository,
	imageRepo repository.ImageRepository,
) StoreService {
	return &storeService{
		optionRepo:   optionRepo,
		variableRepo: variableRepo,
		imageRepo:    imageRepo,
	}
}

func (s *storeService) GetStorePackages(ctx context.Context, codes []string) ([]*PackageResource, error) {
	if err := validateStoreCodes(codes); err != nil {
		return nil, err
	}

	options, err := s.optionRepo.FindByCodes(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("failed to find options: %w", err)
	}

	packages := make([]*PackageResource, 0, len(options))
	for _, option := range options {
		packages = append(packages, s.packageResource(ctx, option))
	}
	return packages, nil
}

func validateStoreCodes(codes []string) error {
	if len(codes) < constants.MinStoreCodes {
		return ErrInvalidCodes
	}
	for _, code := range codes {
		if len(code) < constants.MinStoreCodeLength {
			return ErrInvalidCodeLength
		}
	}
	return nil
}

func (s *storeService) packageResource(ctx context.Context, option *models.Option) *PackageResource {
	return &PackageResource{
		ID:        option.ID,
		Code:      option.Code,
		Asset:     option.Asset,
		Amount:    option.Amount,
		UnitPrice: s.unitPrice(ctx, option.Asset),
		Image:     s.packageImage(ctx, option.ID),
	}
}

// unitPrice returns zero when the rate cannot be loaded so one missing rate does not drop the package list.
func (s *storeService) unitPrice(ctx context.Context, asset string) float64 {
	rate, err := s.variableRepo.GetRate(ctx, asset)
	if err != nil {
		return 0
	}
	return rate
}

func (s *storeService) packageImage(ctx context.Context, optionID uint64) *string {
	imageURL, err := s.imageRepo.FindImageURLByImageable(ctx, constants.OptionPayableType, optionID)
	if err != nil || imageURL == "" {
		return nil
	}
	return &imageURL
}
