// Package repository provides data access for the auth service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/auth-service/internal/models"
)

type ActivityRepository interface {
	// User Events
	CreateUserEvent(ctx context.Context, event *models.UserEvent) error
	GetUserEventsByUserID(ctx context.Context, userID uint64, page int32) ([]*models.UserEvent, error)
	GetUserEventByID(ctx context.Context, userID, eventID uint64) (*models.UserEvent, error)

	// User Event Reports
	CreateUserEventReport(ctx context.Context, report *models.UserEventReport) error
	GetUserEventReportByEventID(ctx context.Context, eventID uint64) (*models.UserEventReport, error)
	UpdateUserEventReportStatus(ctx context.Context, reportID uint64, status int32) error
	CloseUserEventReport(ctx context.Context, reportID uint64) error

	// User Event Report Responses
	CreateUserEventReportResponse(ctx context.Context, response *models.UserEventReportResponse) error
	GetUserEventReportResponses(ctx context.Context, reportID uint64) ([]*models.UserEventReportResponse, error)

	// User Activities
	CreateActivity(ctx context.Context, activity *models.UserActivity) error
	GetLatestActivity(ctx context.Context, userID uint64) (*models.UserActivity, error)
	UpdateActivity(ctx context.Context, activity *models.UserActivity) error
	GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error)

	// User Logs
	GetUserLog(ctx context.Context, userID uint64) (*models.UserLog, error)
	CreateUserLog(ctx context.Context, log *models.UserLog) error
	UpdateUserLog(ctx context.Context, log *models.UserLog) error
	IncrementLogField(ctx context.Context, userID uint64, field string, amount float64) error
}

type activityRepository struct {
	db *sql.DB
}

func NewActivityRepository(db *sql.DB) ActivityRepository {
	return &activityRepository{db: db}
}

// CreateUserEvent creates a new user event record
func (r *activityRepository) CreateUserEvent(ctx context.Context, event *models.UserEvent) error {
	query := `
		INSERT INTO user_events (user_id, event, ip, device, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		event.UserID, event.Event, event.IP, event.Device, event.Status,
		time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	event.ID = uint64(id)

	return nil
}

// GetUserEventsByUserID gets paginated user events for a user (simple pagination, 10 per page)
func (r *activityRepository) GetUserEventsByUserID(ctx context.Context, userID uint64, page int32) ([]*models.UserEvent, error) {
	perPage := int32(10)
	offset := (page - 1) * perPage

	query := `
		SELECT id, user_id, event, ip, device, status, created_at, updated_at
		FROM user_events
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, perPage+1, offset) // +1 to check if there's a next page
	if err != nil {
		return nil, fmt.Errorf("failed to get user events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []*models.UserEvent
	for rows.Next() {
		event := &models.UserEvent{}
		err := rows.Scan(
			&event.ID, &event.UserID, &event.Event, &event.IP,
			&event.Device, &event.Status, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user event: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user events: %w", err)
	}

	return events, nil
}

// GetUserEventByID gets a specific user event by ID, ensuring it belongs to the user
func (r *activityRepository) GetUserEventByID(ctx context.Context, userID, eventID uint64) (*models.UserEvent, error) {
	query := `
		SELECT id, user_id, event, ip, device, status, created_at, updated_at
		FROM user_events
		WHERE id = ? AND user_id = ?
	`

	event := &models.UserEvent{}
	err := r.db.QueryRowContext(ctx, query, eventID, userID).Scan(
		&event.ID, &event.UserID, &event.Event, &event.IP,
		&event.Device, &event.Status, &event.CreatedAt, &event.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user event: %w", err)
	}

	return event, nil
}

// CreateUserEventReport creates a new user event report
func (r *activityRepository) CreateUserEventReport(ctx context.Context, report *models.UserEventReport) error {
	query := `
		INSERT INTO user_event_reports (user_event_id, suspecious_citizen, event_description, status, closed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	var suspeciousCitizen interface{}
	if report.SuspeciousCitizen.Valid {
		suspeciousCitizen = report.SuspeciousCitizen.String
	} else {
		suspeciousCitizen = nil
	}

	result, err := r.db.ExecContext(ctx, query,
		report.UserEventID, suspeciousCitizen, report.EventDescription,
		report.Status, report.Closed, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user event report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	report.ID = uint64(id)

	return nil
}

// GetUserEventReportByEventID gets a user event report by event ID
func (r *activityRepository) GetUserEventReportByEventID(ctx context.Context, eventID uint64) (*models.UserEventReport, error) {
	query := `
		SELECT id, user_event_id, suspecious_citizen, event_description, status, closed, created_at, updated_at
		FROM user_event_reports
		WHERE user_event_id = ?
	`

	report := &models.UserEventReport{}
	var suspeciousCitizen sql.NullString
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(
		&report.ID, &report.UserEventID, &suspeciousCitizen, &report.EventDescription,
		&report.Status, &report.Closed, &report.CreatedAt, &report.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user event report: %w", err)
	}

	report.SuspeciousCitizen = suspeciousCitizen
	return report, nil
}

// UpdateUserEventReportStatus updates the status of a user event report
func (r *activityRepository) UpdateUserEventReportStatus(ctx context.Context, reportID uint64, status int32) error {
	query := `
		UPDATE user_event_reports
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), reportID)
	if err != nil {
		return fmt.Errorf("failed to update user event report status: %w", err)
	}
	return nil
}

// CloseUserEventReport closes a user event report (sets closed = 1)
func (r *activityRepository) CloseUserEventReport(ctx context.Context, reportID uint64) error {
	query := `
		UPDATE user_event_reports
		SET closed = 1, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), reportID)
	if err != nil {
		return fmt.Errorf("failed to close user event report: %w", err)
	}
	return nil
}

// CreateUserEventReportResponse creates a new response to a user event report
func (r *activityRepository) CreateUserEventReportResponse(ctx context.Context, response *models.UserEventReportResponse) error {
	query := `
		INSERT INTO user_event_report_responses (user_event_report_id, response, responser_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		response.UserEventReportID, response.Response, response.ResponserName,
		time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user event report response: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	response.ID = uint64(id)

	return nil
}

// GetUserEventReportResponses gets all responses for a user event report
func (r *activityRepository) GetUserEventReportResponses(ctx context.Context, reportID uint64) ([]*models.UserEventReportResponse, error) {
	query := `
		SELECT id, user_event_report_id, response, responser_name, created_at, updated_at
		FROM user_event_report_responses
		WHERE user_event_report_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user event report responses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var responses []*models.UserEventReportResponse
	for rows.Next() {
		response := &models.UserEventReportResponse{}
		err := rows.Scan(
			&response.ID, &response.UserEventReportID, &response.Response,
			&response.ResponserName, &response.CreatedAt, &response.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user event report response: %w", err)
		}
		responses = append(responses, response)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user event report responses: %w", err)
	}

	return responses, nil
}

// CreateActivity creates a new activity record
func (r *activityRepository) CreateActivity(ctx context.Context, activity *models.UserActivity) error {
	query := `
		INSERT INTO user_activities (user_id, start, ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		activity.UserID, activity.Start, activity.IP, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	activity.ID = uint64(id)

	return nil
}

// GetLatestActivity gets the most recent activity for a user
func (r *activityRepository) GetLatestActivity(ctx context.Context, userID uint64) (*models.UserActivity, error) {
	query := `
		SELECT id, user_id, start, end, total, ip, created_at, updated_at
		FROM user_activities
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT 1
	`
	activity := &models.UserActivity{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&activity.ID, &activity.UserID, &activity.Start, &activity.End,
		&activity.Total, &activity.IP, &activity.CreatedAt, &activity.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest activity: %w", err)
	}
	return activity, nil
}

// UpdateActivity updates an activity record
func (r *activityRepository) UpdateActivity(ctx context.Context, activity *models.UserActivity) error {
	query := `
		UPDATE user_activities
		SET end = ?, total = ?, ip = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		activity.End, activity.Total, activity.IP, time.Now(), activity.ID)
	if err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}
	return nil
}

// GetTotalActivityMinutes calculates total minutes from all activities
func (r *activityRepository) GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error) {
	query := `
		SELECT COALESCE(SUM(total), 0) as total_minutes
		FROM user_activities
		WHERE user_id = ?
	`
	var total int32
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total activity minutes: %w", err)
	}
	return total, nil
}

// GetUserLog retrieves the user log record
func (r *activityRepository) GetUserLog(ctx context.Context, userID uint64) (*models.UserLog, error) {
	query := `
		SELECT id, user_id, transactions_count, followers_count, deposit_amount, 
		       activity_hours, score, created_at, updated_at
		FROM user_logs
		WHERE user_id = ?
	`
	log := &models.UserLog{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&log.ID, &log.UserID, &log.TransactionsCount, &log.FollowersCount,
		&log.DepositAmount, &log.ActivityHours, &log.Score,
		&log.CreatedAt, &log.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user log: %w", err)
	}
	return log, nil
}

// CreateUserLog creates a new user log record
func (r *activityRepository) CreateUserLog(ctx context.Context, log *models.UserLog) error {
	query := `
		INSERT INTO user_logs (user_id, transactions_count, followers_count, 
		                       deposit_amount, activity_hours, score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		log.UserID, log.TransactionsCount, log.FollowersCount,
		log.DepositAmount, log.ActivityHours, log.Score,
		time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	log.ID = uint64(id)

	return nil
}

// UpdateUserLog updates a user log record
func (r *activityRepository) UpdateUserLog(ctx context.Context, log *models.UserLog) error {
	query := `
		UPDATE user_logs
		SET transactions_count = ?, followers_count = ?, deposit_amount = ?,
		    activity_hours = ?, score = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		log.TransactionsCount, log.FollowersCount, log.DepositAmount,
		log.ActivityHours, log.Score, time.Now(), log.ID)
	if err != nil {
		return fmt.Errorf("failed to update user log: %w", err)
	}
	return nil
}

// IncrementLogField increments a specific field in user log
func (r *activityRepository) IncrementLogField(ctx context.Context, userID uint64, field string, amount float64) error {
	// Validate field name to prevent SQL injection
	validFields := map[string]bool{
		"transactions_count": true,
		"followers_count":    true,
		"deposit_amount":     true,
		"activity_hours":     true,
		"score":              true,
	}

	if !validFields[field] {
		return fmt.Errorf("invalid field name: %s", field)
	}

	query := fmt.Sprintf(`
		UPDATE user_logs
		SET %s = %s + ?, updated_at = ?
		WHERE user_id = ?
	`, field, field)

	_, err := r.db.ExecContext(ctx, query, amount, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to increment log field %s: %w", field, err)
	}
	return nil
}
