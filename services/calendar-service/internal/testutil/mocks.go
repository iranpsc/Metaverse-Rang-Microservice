package testutil

import (
	"context"

	"metargb/calendar-service/internal/models"
)

// MockCalendarRepo implements repository.CalendarRepositoryInterface for tests.
type MockCalendarRepo struct {
	GetEventsFunc             func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error)
	GetEventByIDFunc          func(ctx context.Context, id uint64) (*models.Calendar, error)
	FilterByDateRangeFunc     func(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error)
	GetLatestVersionTitleFunc func(ctx context.Context) (string, error)
	GetEventStatsFunc         func(ctx context.Context, eventID uint64) (*models.CalendarStats, error)
	GetInteractionStatsFunc   func(ctx context.Context, eventID uint64) (*models.CalendarStats, error)
	GetUserInteractionFunc    func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error)
	AddInteractionFunc        func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error
	IncrementViewFunc         func(ctx context.Context, eventID uint64, ipAddress string) error
}

func (m *MockCalendarRepo) GetEvents(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
	if m.GetEventsFunc != nil {
		return m.GetEventsFunc(ctx, eventType, search, date, userID, page, perPage)
	}
	return nil, false, nil
}

func (m *MockCalendarRepo) GetEventByID(ctx context.Context, id uint64) (*models.Calendar, error) {
	if m.GetEventByIDFunc != nil {
		return m.GetEventByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockCalendarRepo) FilterByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error) {
	if m.FilterByDateRangeFunc != nil {
		return m.FilterByDateRangeFunc(ctx, startDate, endDate)
	}
	return nil, nil
}

func (m *MockCalendarRepo) GetLatestVersionTitle(ctx context.Context) (string, error) {
	if m.GetLatestVersionTitleFunc != nil {
		return m.GetLatestVersionTitleFunc(ctx)
	}
	return "", nil
}

func (m *MockCalendarRepo) GetEventStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
	if m.GetEventStatsFunc != nil {
		return m.GetEventStatsFunc(ctx, eventID)
	}
	return &models.CalendarStats{}, nil
}

func (m *MockCalendarRepo) GetInteractionStats(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
	if m.GetInteractionStatsFunc != nil {
		return m.GetInteractionStatsFunc(ctx, eventID)
	}
	return &models.CalendarStats{}, nil
}

func (m *MockCalendarRepo) GetUserInteraction(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
	if m.GetUserInteractionFunc != nil {
		return m.GetUserInteractionFunc(ctx, eventID, userID)
	}
	return nil, nil
}

func (m *MockCalendarRepo) AddInteraction(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
	if m.AddInteractionFunc != nil {
		return m.AddInteractionFunc(ctx, eventID, userID, liked, ipAddress)
	}
	return nil
}

func (m *MockCalendarRepo) IncrementView(ctx context.Context, eventID uint64, ipAddress string) error {
	if m.IncrementViewFunc != nil {
		return m.IncrementViewFunc(ctx, eventID, ipAddress)
	}
	return nil
}
