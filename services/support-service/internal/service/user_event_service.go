package service

import (
	"context"
	"fmt"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/repository"
)

type UserEventService interface {
	CreateUserEvent(ctx context.Context, userID uint64, title, description, eventDate string) (*models.UserEvent, error)
	GetUserEvents(ctx context.Context, userID uint64, page, perPage int32) ([]*models.UserEvent, int, error)
	GetUserEvent(ctx context.Context, eventID, userID uint64) (*models.UserEventWithReport, error)
	ReportUserEvent(ctx context.Context, eventID uint64, suspiciousCitizen, eventDescription string) (*models.UserEventReport, error)
	SendEventReportResponse(ctx context.Context, eventID uint64, responderName, response string) (*models.UserEventReportResponse, error)
	CloseUserEventReport(ctx context.Context, eventID, userID uint64) error
}

type userEventService struct {
	userEventRepo repository.UserEventRepository
}

func NewUserEventService(userEventRepo repository.UserEventRepository) UserEventService {
	return &userEventService{
		userEventRepo: userEventRepo,
	}
}

func (s *userEventService) CreateUserEvent(ctx context.Context, userID uint64, title, description, eventDate string) (*models.UserEvent, error) {
	_ = description
	_ = eventDate
	event := &models.UserEvent{
		UserID: userID,
		Event:  title,
		IP:     "0.0.0.0",
		Device: "unknown",
		Status: true,
	}

	return s.userEventRepo.Create(ctx, event)
}

func (s *userEventService) GetUserEvents(ctx context.Context, userID uint64, page, perPage int32) ([]*models.UserEvent, int, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if page <= 0 {
		page = 1
	}

	return s.userEventRepo.GetByUserID(ctx, userID, page, perPage)
}

func (s *userEventService) GetUserEvent(ctx context.Context, eventID, userID uint64) (*models.UserEventWithReport, error) {
	event, err := s.userEventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, nil
	}
	if event.UserID != userID {
		return nil, fmt.Errorf("unauthorized: you don't have permission to view this event")
	}
	return event, nil
}

func (s *userEventService) ReportUserEvent(ctx context.Context, eventID uint64, suspiciousCitizen, eventDescription string) (*models.UserEventReport, error) {
	var suspiciousCitizenPtr *string
	if suspiciousCitizen != "" {
		suspiciousCitizenPtr = &suspiciousCitizen
	}

	report := &models.UserEventReport{
		UserEventID:       eventID,
		SuspeciousCitizen: suspiciousCitizenPtr,
		EventDescription:  eventDescription,
		Status:            0,
		Closed:            false,
	}

	return s.userEventRepo.CreateReport(ctx, report)
}

func (s *userEventService) SendEventReportResponse(ctx context.Context, eventID uint64, responderName, response string) (*models.UserEventReportResponse, error) {
	report, err := s.userEventRepo.GetReportByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}
	if report == nil {
		return nil, fmt.Errorf("report not found")
	}

	reportResponse := &models.UserEventReportResponse{
		UserEventReportID: report.ID,
		Response:          response,
		ResponserName:     responderName,
	}

	created, err := s.userEventRepo.CreateReportResponse(ctx, reportResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	err = s.userEventRepo.UpdateReportStatus(ctx, report.ID, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to update report status: %w", err)
	}

	return created, nil
}

func (s *userEventService) CloseUserEventReport(ctx context.Context, eventID, userID uint64) error {
	event, err := s.userEventRepo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return fmt.Errorf("user event not found")
	}
	if event.UserID != userID {
		return fmt.Errorf("unauthorized: you don't have permission to close this report")
	}

	report, err := s.userEventRepo.GetReportByEventID(ctx, eventID)
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("report not found")
	}

	return s.userEventRepo.CloseReport(ctx, report.ID)
}
