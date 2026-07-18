// Package service implements calendar business logic.
package service

import (
	"context"
	"errors"
	"fmt"

	"metarang/calendar-service/internal/models"
	"metarang/calendar-service/internal/repository"
)

var ErrEventNotFound = errors.New("event not found")

// CalendarServiceInterface defines the interface for calendar service operations
type CalendarServiceInterface interface {
	GetEvents(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error)
	GetEvent(ctx context.Context, eventID, userID uint64) (*models.Calendar, error)
	FilterByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error)
	GetLatestVersionTitle(ctx context.Context) (string, error)
	GetEventStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error)
	GetInteractionStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error)
	GetUserInteraction(ctx context.Context, eventID, userID uint64) (*models.Interaction, error)
	AddInteraction(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error
	IncrementView(ctx context.Context, eventID uint64, ipAddress string) error
}

type CalendarService struct {
	repo repository.CalendarRepositoryInterface
}

func NewCalendarService(repo repository.CalendarRepositoryInterface) CalendarServiceInterface {
	return &CalendarService{repo: repo}
}

func (s *CalendarService) GetEvents(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
	return s.repo.GetEvents(ctx, eventType, search, date, userID, page, perPage)
}

func (s *CalendarService) GetEvent(ctx context.Context, eventID, userID uint64) (*models.Calendar, error) {
	event, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	if event == nil {
		return nil, ErrEventNotFound
	}
	return event, nil
}

func (s *CalendarService) FilterByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error) {
	return s.repo.FilterByDateRange(ctx, startDate, endDate)
}

func (s *CalendarService) GetLatestVersionTitle(ctx context.Context) (string, error) {
	return s.repo.GetLatestVersionTitle(ctx)
}

func (s *CalendarService) GetEventStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
	return s.repo.GetEventStats(ctx, eventID)
}

func (s *CalendarService) GetInteractionStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
	return s.repo.GetInteractionStats(ctx, eventID)
}

func (s *CalendarService) GetUserInteraction(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
	return s.repo.GetUserInteraction(ctx, eventID, userID)
}

func (s *CalendarService) AddInteraction(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
	if liked < -1 || liked > 1 {
		return fmt.Errorf("invalid liked value: must be -1, 0, or 1")
	}
	return s.repo.AddInteraction(ctx, eventID, userID, liked, ipAddress)
}

func (s *CalendarService) IncrementView(ctx context.Context, eventID uint64, ipAddress string) error {
	return s.repo.IncrementView(ctx, eventID, ipAddress)
}
