package service

import (
	"context"
	"fmt"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/repository"
)

type ReportService interface {
	CreateReport(ctx context.Context, userID uint64, subject, title, content, url string, imageURLs []string) (*models.ReportWithImages, error)
	GetReports(ctx context.Context, userID uint64, page, perPage int32) ([]*models.Report, int, error)
	GetReport(ctx context.Context, reportID, userID uint64) (*models.ReportWithImages, error)
}

type reportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) ReportService {
	return &reportService{
		reportRepo: reportRepo,
	}
}

func (s *reportService) CreateReport(ctx context.Context, userID uint64, subject, title, content, url string, imageURLs []string) (*models.ReportWithImages, error) {
	report := &models.Report{
		Subject: subject,
		Title:   title,
		Content: content,
		URL:     url,
		UserID:  userID,
		Status:  0, // Default status
	}

	createdReport, err := s.reportRepo.Create(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to create report: %w", err)
	}

	for _, imageURL := range imageURLs {
		err := s.reportRepo.CreateImage(ctx, createdReport.ID, imageURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create image: %w", err)
		}
	}

	return s.reportRepo.GetByID(ctx, createdReport.ID)
}

func (s *reportService) GetReports(ctx context.Context, userID uint64, page, perPage int32) ([]*models.Report, int, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if page <= 0 {
		page = 1
	}

	return s.reportRepo.GetByUserID(ctx, userID, page, perPage)
}

func (s *reportService) GetReport(ctx context.Context, reportID, userID uint64) (*models.ReportWithImages, error) {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, nil
	}
	if report.UserID != userID {
		return nil, fmt.Errorf("unauthorized: you don't have permission to view this report")
	}
	return report, nil
}
