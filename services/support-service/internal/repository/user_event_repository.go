package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/support-service/internal/models"
)

type UserEventRepository interface {
	Create(ctx context.Context, event *models.UserEvent) (*models.UserEvent, error)
	GetByID(ctx context.Context, eventID uint64) (*models.UserEventWithReport, error)
	GetByUserID(ctx context.Context, userID uint64, page, perPage int32) ([]*models.UserEvent, int, error)
	CreateReport(ctx context.Context, report *models.UserEventReport) (*models.UserEventReport, error)
	UpdateReportStatus(ctx context.Context, reportID uint64, status int32) error
	CloseReport(ctx context.Context, reportID uint64) error
	CreateReportResponse(ctx context.Context, response *models.UserEventReportResponse) (*models.UserEventReportResponse, error)
	GetReportByEventID(ctx context.Context, eventID uint64) (*models.UserEventReport, error)
	GetReportResponses(ctx context.Context, reportID uint64) ([]models.UserEventReportResponse, error)
}

type userEventRepository struct {
	db *sql.DB
}

func NewUserEventRepository(db *sql.DB) UserEventRepository {
	return &userEventRepository{db: db}
}

func (r *userEventRepository) Create(ctx context.Context, event *models.UserEvent) (*models.UserEvent, error) {
	query := `
		INSERT INTO user_events (user_id, event, ip, device, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		event.UserID,
		event.Event,
		event.IP,
		event.Device,
		event.Status,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	event.ID = uint64(id)
	return event, nil
}

func (r *userEventRepository) GetByID(ctx context.Context, eventID uint64) (*models.UserEventWithReport, error) {
	query := `
		SELECT id, user_id, event, ip, device, status, created_at, updated_at
		FROM user_events
		WHERE id = ?
	`

	var event models.UserEventWithReport
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(
		&event.ID, &event.UserID, &event.Event, &event.IP,
		&event.Device, &event.Status, &event.CreatedAt, &event.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user event: %w", err)
	}

	// Load report if exists
	report, err := r.GetReportByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	event.Report = report

	// Load responses if report exists
	if report != nil {
		responses, err := r.GetReportResponses(ctx, report.ID)
		if err != nil {
			return nil, err
		}
		event.Responses = responses
	}

	return &event, nil
}

func (r *userEventRepository) GetByUserID(ctx context.Context, userID uint64, page, perPage int32) ([]*models.UserEvent, int, error) {
	// Count total events
	countQuery := `SELECT COUNT(*) FROM user_events WHERE user_id = ?`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user events: %w", err)
	}

	// Get events with pagination
	offset := (page - 1) * perPage
	query := `
		SELECT id, user_id, event, ip, device, status, created_at, updated_at
		FROM user_events
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []*models.UserEvent
	for rows.Next() {
		var event models.UserEvent
		err := rows.Scan(
			&event.ID, &event.UserID, &event.Event, &event.IP,
			&event.Device, &event.Status, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user event: %w", err)
		}
		events = append(events, &event)
	}

	return events, total, nil
}

func (r *userEventRepository) CreateReport(ctx context.Context, report *models.UserEventReport) (*models.UserEventReport, error) {
	query := `
		INSERT INTO user_event_reports (user_event_id, suspecious_citizen, event_description, status, closed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		report.UserEventID,
		report.SuspeciousCitizen,
		report.EventDescription,
		report.Status,
		report.Closed,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user event report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	report.ID = uint64(id)
	return report, nil
}

func (r *userEventRepository) UpdateReportStatus(ctx context.Context, reportID uint64, status int32) error {
	query := `UPDATE user_event_reports SET status = ?, updated_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, status, reportID)
	if err != nil {
		return fmt.Errorf("failed to update report status: %w", err)
	}

	return nil
}

func (r *userEventRepository) CloseReport(ctx context.Context, reportID uint64) error {
	query := `UPDATE user_event_reports SET closed = 1, updated_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, reportID)
	if err != nil {
		return fmt.Errorf("failed to close report: %w", err)
	}

	return nil
}

func (r *userEventRepository) CreateReportResponse(ctx context.Context, response *models.UserEventReportResponse) (*models.UserEventReportResponse, error) {
	query := `
		INSERT INTO user_event_report_responses (user_event_report_id, response, responser_name, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		response.UserEventReportID,
		response.Response,
		response.ResponserName,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create report response: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	response.ID = uint64(id)
	return response, nil
}

func (r *userEventRepository) GetReportByEventID(ctx context.Context, eventID uint64) (*models.UserEventReport, error) {
	query := `
		SELECT id, user_event_id, suspecious_citizen, event_description, status, closed, created_at, updated_at
		FROM user_event_reports
		WHERE user_event_id = ?
	`

	var report models.UserEventReport
	var suspeciousCitizen sql.NullString

	err := r.db.QueryRowContext(ctx, query, eventID).Scan(
		&report.ID, &report.UserEventID, &suspeciousCitizen, &report.EventDescription,
		&report.Status, &report.Closed, &report.CreatedAt, &report.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	if suspeciousCitizen.Valid {
		report.SuspeciousCitizen = &suspeciousCitizen.String
	}

	return &report, nil
}

func (r *userEventRepository) GetReportResponses(ctx context.Context, reportID uint64) ([]models.UserEventReportResponse, error) {
	query := `
		SELECT id, user_event_report_id, response, responser_name, created_at, updated_at
		FROM user_event_report_responses
		WHERE user_event_report_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, fmt.Errorf("failed to get report responses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var responses []models.UserEventReportResponse
	for rows.Next() {
		var response models.UserEventReportResponse
		err := rows.Scan(
			&response.ID, &response.UserEventReportID, &response.Response,
			&response.ResponserName, &response.CreatedAt, &response.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan response: %w", err)
		}
		responses = append(responses, response)
	}

	return responses, nil
}
