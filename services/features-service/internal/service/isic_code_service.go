package service

import (
	"context"
	"fmt"
	"strings"

	"metarang/features-service/internal/models"
)

type isicCodeRepo interface {
	FindPaginated(ctx context.Context, search string, limit, offset int) ([]models.IsicCode, error)
	Count(ctx context.Context, search string) (int, error)
}

// IsicCodeService lists ISIC codes (GET /api/isic-codes).
type IsicCodeService struct {
	repo isicCodeRepo
}

func NewIsicCodeService(repo isicCodeRepo) *IsicCodeService {
	return &IsicCodeService{repo: repo}
}

// Paginate returns a page of ISIC codes (10 per page). Search filters by name or code when set.
func (s *IsicCodeService) Paginate(ctx context.Context, page int, search string) (*models.IsicCodePage, error) {
	if page < 1 {
		page = 1
	}

	search = strings.TrimSpace(search)
	perPage := models.IsicCodePerPage

	total, err := s.repo.Count(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("count isic codes: %w", err)
	}

	lastPage := total / perPage
	if total%perPage != 0 {
		lastPage++
	}
	if lastPage < 1 {
		lastPage = 1
	}

	offset := (page - 1) * perPage
	rows, err := s.repo.FindPaginated(ctx, search, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("list isic codes: %w", err)
	}

	result := &models.IsicCodePage{
		Items:       rows,
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		LastPage:    lastPage,
		Path:        models.IsicCodePath,
		Search:      search,
	}
	if len(rows) > 0 {
		from := offset + 1
		to := offset + len(rows)
		result.From = &from
		result.To = &to
	}

	return result, nil
}
