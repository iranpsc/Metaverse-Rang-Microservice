package service

import (
	"context"
	"fmt"

	"metarang/commercial-service/internal/repository"
)

// UserVariableService creates and manages per-user commercial variables.
type UserVariableService interface {
	CreateUserVariables(ctx context.Context, userID uint64) error
}

type userVariableService struct {
	userVariableRepo repository.UserVariableRepository
}

func NewUserVariableService(userVariableRepo repository.UserVariableRepository) UserVariableService {
	return &userVariableService{userVariableRepo: userVariableRepo}
}

func (s *userVariableService) CreateUserVariables(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return fmt.Errorf("user_id is required")
	}
	if err := s.userVariableRepo.Create(ctx, userID); err != nil {
		return fmt.Errorf("failed to create user variables: %w", err)
	}
	return nil
}
